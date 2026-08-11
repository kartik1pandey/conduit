package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// --- flaky receiver harness -------------------------------------------------
//
// Simulates the "flaky receiving endpoint" docs/ARCHITECTURE.md describes for
// conduit-webhooks: each successive request to it gets the next behavior in
// a configured sequence (the last behavior repeats indefinitely once the
// sequence is exhausted, so "always fails" is just a one-element sequence).

type recordedHit struct {
	body    []byte
	headers http.Header
}

type flakyServer struct {
	server    *httptest.Server
	behaviors []func(w http.ResponseWriter, r *http.Request, body []byte)

	mu   sync.Mutex
	hits []recordedHit
}

func newFlakyServer(t *testing.T, behaviors ...func(http.ResponseWriter, *http.Request, []byte)) *flakyServer {
	t.Helper()
	fs := &flakyServer{behaviors: behaviors}
	fs.server = httptest.NewServer(http.HandlerFunc(fs.handle))
	t.Cleanup(fs.server.Close)
	return fs
}

func (fs *flakyServer) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	fs.mu.Lock()
	idx := len(fs.hits)
	fs.hits = append(fs.hits, recordedHit{body: body, headers: r.Header.Clone()})
	fs.mu.Unlock()

	var behavior func(http.ResponseWriter, *http.Request, []byte)
	if idx < len(fs.behaviors) {
		behavior = fs.behaviors[idx]
	} else {
		behavior = fs.behaviors[len(fs.behaviors)-1]
	}
	behavior(w, r, body)
}

func (fs *flakyServer) hitCount() int {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return len(fs.hits)
}

func (fs *flakyServer) hit(i int) recordedHit {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.hits[i]
}

func (fs *flakyServer) url() string { return fs.server.URL }

// --- individual receiver behaviors -----------------------------------------

func behaviorFail500(w http.ResponseWriter, _ *http.Request, _ []byte) {
	w.WriteHeader(http.StatusInternalServerError)
}

func behaviorSucceed(w http.ResponseWriter, _ *http.Request, _ []byte) {
	w.WriteHeader(http.StatusOK)
}

func behaviorSucceedVerifyingSignature(t *testing.T, secret string) func(http.ResponseWriter, *http.Request, []byte) {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		if !Verify(secret, body, r.Header.Get("Conduit-Signature")) {
			t.Errorf("Conduit-Signature did not validate against payload %s", body)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// behaviorDropConnection simulates a network-level failure: the connection
// is closed before any response is written, so the sender's HTTP client
// sees an error, not a status code.
func behaviorDropConnection(w http.ResponseWriter, _ *http.Request, _ []byte) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	conn.Close()
}

// behaviorProcessButDropResponse simulates the classic at-least-once
// delivery risk: the receiver actually does the work (this handler runs,
// meaning a real merchant server would have processed the event) but the
// acknowledgment never reaches the sender, which will retry — resulting in
// a second, genuinely duplicate delivery from the merchant's point of view.
// Structurally identical to behaviorDropConnection; kept as a separate named
// behavior because what it's simulating is different, and the chaos test
// using it asserts something different (both hits carry the same event ID).
func behaviorProcessButDropResponse(w http.ResponseWriter, r *http.Request, body []byte) {
	behaviorDropConnection(w, r, body)
}

func behaviorDelay(d time.Duration) func(http.ResponseWriter, *http.Request, []byte) {
	return func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		time.Sleep(d)
		w.WriteHeader(http.StatusOK)
	}
}

// --- test setup --------------------------------------------------------

// fakeClock lets chaos tests fast-forward past a scheduled retry delay
// without actually sleeping through it — Worker only cares whether "now" is
// past a delivery's due time, and the due time is computed from whatever
// this clock reports.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time              { return c.t }
func (c *fakeClock) advancePast(d time.Duration) { c.t = c.t.Add(d + time.Second) }

type chaosEnv struct {
	store *Store
	queue *RetryQueue
	clock *fakeClock
}

func setupChaosEnv(t *testing.T) *chaosEnv {
	t.Helper()
	store := setupStore(t)

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set; skipping chaos test")
	}
	opts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })
	require.NoError(t, client.Del(context.Background(), retryScheduleKey).Err())

	return &chaosEnv{store: store, queue: NewRetryQueue(client), clock: &fakeClock{t: time.Now()}}
}

// newDelivery registers a fresh endpoint pointed at the flaky server, emits
// one event, and returns the resulting delivery row plus the endpoint's
// HMAC secret (needed by signature-checking behaviors).
func (env *chaosEnv) newDelivery(t *testing.T, endpointURL string) (Delivery, string) {
	t.Helper()
	ctx := context.Background()
	merchantID := uuid.New()

	endpoint, secret, err := env.store.CreateEndpoint(ctx, merchantID, endpointURL)
	require.NoError(t, err)

	payload, err := json.Marshal(eventPayload{ID: uuid.New(), Type: "payment.succeeded", Data: json.RawMessage(`{}`)})
	require.NoError(t, err)

	event, _, err := env.store.CreateEvent(ctx, merchantID, "payment.succeeded", uuid.NewString(), payload)
	require.NoError(t, err)

	deliveries, err := env.store.CreateDeliveries(ctx, event.ID, []uuid.UUID{endpoint.ID})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	require.NoError(t, env.queue.Schedule(ctx, deliveries[0].ID, env.clock.now()))
	return deliveries[0], secret
}

func newTestWorker(env *chaosEnv, deliverer *Deliverer, maxAttempts int) *Worker {
	w := NewWorker(env.store, env.queue, deliverer, maxAttempts)
	w.now = env.clock.now
	return w
}

// --- chaos tests -------------------------------------------------------

func TestChaos_SucceedsOnFirstAttempt_SignatureVerifies(t *testing.T) {
	env := setupChaosEnv(t)
	fs := newFlakyServer(t) // populated after secret is known, see below

	delivery, secret := env.newDelivery(t, fs.url())
	fs.behaviors = []func(http.ResponseWriter, *http.Request, []byte){behaviorSucceedVerifyingSignature(t, secret)}

	worker := newTestWorker(env, NewDeliverer(2*time.Second), 5)
	processed, err := worker.ProcessOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	got, err := env.store.GetDelivery(context.Background(), delivery.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDelivered, got.Status)
	require.Equal(t, 1, got.AttemptCount)
	require.Equal(t, 1, fs.hitCount())
}

func TestChaos_RetriesWithBackoffThenSucceeds(t *testing.T) {
	env := setupChaosEnv(t)
	fs := newFlakyServer(t, behaviorFail500, behaviorFail500, behaviorSucceed)
	delivery, _ := env.newDelivery(t, fs.url())

	worker := newTestWorker(env, NewDeliverer(2*time.Second), 5)
	ctx := context.Background()

	for attempt := 1; attempt <= 3; attempt++ {
		processed, err := worker.ProcessOnce(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, processed, "attempt %d", attempt)
		env.clock.advancePast(Cap(attempt))
	}

	got, err := env.store.GetDelivery(ctx, delivery.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDelivered, got.Status)
	require.Equal(t, 3, got.AttemptCount)
	require.Equal(t, 3, fs.hitCount())
}

func TestChaos_DeadLettersAfterMaxAttempts_NoFurtherAttemptsFire(t *testing.T) {
	env := setupChaosEnv(t)
	fs := newFlakyServer(t, behaviorFail500) // repeats forever
	delivery, _ := env.newDelivery(t, fs.url())

	const maxAttempts = 3
	worker := newTestWorker(env, NewDeliverer(2*time.Second), maxAttempts)
	ctx := context.Background()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		processed, err := worker.ProcessOnce(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, processed, "attempt %d", attempt)
		env.clock.advancePast(Cap(attempt))
	}

	got, err := env.store.GetDelivery(ctx, delivery.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDeadLettered, got.Status)
	require.Equal(t, maxAttempts, got.AttemptCount)

	// Advancing time further and processing again must fire nothing: a
	// dead-lettered delivery was never rescheduled, so there's nothing due.
	env.clock.advancePast(Cap(maxAttempts + 5))
	processed, err := worker.ProcessOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, processed, "a dead-lettered delivery must not be retried again")
	require.Equal(t, maxAttempts, fs.hitCount(), "no additional HTTP calls should have been made")
}

func TestChaos_ProcessedButResponseDropped_EventuallyDeliveredWithStableEventID(t *testing.T) {
	env := setupChaosEnv(t)
	fs := newFlakyServer(t, behaviorProcessButDropResponse, behaviorSucceed)
	delivery, _ := env.newDelivery(t, fs.url())

	worker := newTestWorker(env, NewDeliverer(2*time.Second), 5)
	ctx := context.Background()

	_, err := worker.ProcessOnce(ctx) // attempt 1: "processed" but ack dropped -> retryable
	require.NoError(t, err)
	env.clock.advancePast(Cap(1))
	_, err = worker.ProcessOnce(ctx) // attempt 2: succeeds
	require.NoError(t, err)

	got, err := env.store.GetDelivery(ctx, delivery.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDelivered, got.Status)
	require.Equal(t, 2, got.AttemptCount)
	require.Equal(t, 2, fs.hitCount(), "the merchant's endpoint really was invoked twice for one logical event")

	// This is exactly why every webhook payload carries a stable event ID:
	// the merchant's receiver can deduplicate the two invocations by it,
	// even though our sender has no way to know the first one "succeeded".
	var firstPayload, secondPayload eventPayload
	require.NoError(t, json.Unmarshal(fs.hit(0).body, &firstPayload))
	require.NoError(t, json.Unmarshal(fs.hit(1).body, &secondPayload))
	require.Equal(t, firstPayload.ID, secondPayload.ID)
}

func TestChaos_DroppedConnectionMidRequest_IsTreatedAsRetryableFailure(t *testing.T) {
	env := setupChaosEnv(t)
	fs := newFlakyServer(t, behaviorDropConnection, behaviorSucceed)
	delivery, _ := env.newDelivery(t, fs.url())

	worker := newTestWorker(env, NewDeliverer(2*time.Second), 5)
	ctx := context.Background()

	_, err := worker.ProcessOnce(ctx)
	require.NoError(t, err, "a dropped connection must not surface as a worker/test error")
	env.clock.advancePast(Cap(1))
	_, err = worker.ProcessOnce(ctx)
	require.NoError(t, err)

	got, err := env.store.GetDelivery(ctx, delivery.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDelivered, got.Status)
	require.Equal(t, 2, got.AttemptCount)
}

func TestChaos_DelayedResponseBeyondClientTimeout_IsRetryableNotHung(t *testing.T) {
	env := setupChaosEnv(t)
	fs := newFlakyServer(t, behaviorDelay(300*time.Millisecond), behaviorSucceed)
	delivery, _ := env.newDelivery(t, fs.url())

	// A short client timeout so this test doesn't have to actually wait out
	// a realistic production timeout to prove the point.
	worker := newTestWorker(env, NewDeliverer(50*time.Millisecond), 5)
	ctx := context.Background()

	start := time.Now()
	_, err := worker.ProcessOnce(ctx)
	require.NoError(t, err)
	require.Less(t, time.Since(start), 300*time.Millisecond, "the worker must not block for the receiver's full delay")

	env.clock.advancePast(Cap(1))
	_, err = worker.ProcessOnce(ctx)
	require.NoError(t, err)

	got, err := env.store.GetDelivery(ctx, delivery.ID)
	require.NoError(t, err)
	require.Equal(t, StatusDelivered, got.Status)
	require.Equal(t, 2, got.AttemptCount)
}
