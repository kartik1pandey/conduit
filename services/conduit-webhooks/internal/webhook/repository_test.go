package webhook

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-webhooks/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-webhooks/migrations"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	dbURL := os.Getenv("WEBHOOKS_DATABASE_URL")
	if dbURL == "" {
		t.Skip("WEBHOOKS_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE webhook_deliveries, webhook_events, webhook_endpoints CASCADE")
	require.NoError(t, err)

	return NewStore(pool)
}

func TestCreateEndpoint(t *testing.T) {
	store := setupStore(t)
	merchantID := uuid.New()

	endpoint, secret, err := store.CreateEndpoint(context.Background(), merchantID, "https://merchant.example/webhooks")
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	require.Contains(t, secret, "whsec_")
	require.Equal(t, "https://merchant.example/webhooks", endpoint.URL)
}

func TestCreateEvent_IsIdempotent(t *testing.T) {
	store := setupStore(t)
	merchantID := uuid.New()
	ctx := context.Background()

	first, wasNew, err := store.CreateEvent(ctx, merchantID, "payment.succeeded", "confirm:pi_1:succeeded", []byte(`{"id":"evt_1"}`))
	require.NoError(t, err)
	require.True(t, wasNew)

	second, wasNew, err := store.CreateEvent(ctx, merchantID, "payment.succeeded", "confirm:pi_1:succeeded", []byte(`{"id":"evt_1"}`))
	require.NoError(t, err)
	require.False(t, wasNew, "a replayed emission must report wasNew=false")
	require.Equal(t, first.ID, second.ID)
}

func TestDeliveriesAreScopedPerMerchant(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantA := uuid.New()
	merchantB := uuid.New()

	endpointA, _, err := store.CreateEndpoint(ctx, merchantA, "https://a.example/hook")
	require.NoError(t, err)

	event, _, err := store.CreateEvent(ctx, merchantA, "payment.succeeded", "key-1", []byte(`{}`))
	require.NoError(t, err)
	_, err = store.CreateDeliveries(ctx, event.ID, []uuid.UUID{endpointA.ID})
	require.NoError(t, err)

	deliveries, err := store.ListDeliveriesForEndpoint(ctx, merchantB, endpointA.ID)
	require.NoError(t, err)
	require.Empty(t, deliveries, "merchant B must not see merchant A's endpoint deliveries")

	ownDeliveries, err := store.ListDeliveriesForEndpoint(ctx, merchantA, endpointA.ID)
	require.NoError(t, err)
	require.Len(t, ownDeliveries, 1)
}

func TestListEndpointsIsScopedPerMerchant(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantA := uuid.New()
	merchantB := uuid.New()

	_, _, err := store.CreateEndpoint(ctx, merchantA, "https://a.example/hook")
	require.NoError(t, err)
	_, _, err = store.CreateEndpoint(ctx, merchantA, "https://a2.example/hook")
	require.NoError(t, err)
	_, _, err = store.CreateEndpoint(ctx, merchantB, "https://b.example/hook")
	require.NoError(t, err)

	endpointsA, err := store.ListEndpoints(ctx, merchantA)
	require.NoError(t, err)
	require.Len(t, endpointsA, 2, "merchant A must see exactly its own endpoints, never merchant B's")

	endpointsB, err := store.ListEndpoints(ctx, merchantB)
	require.NoError(t, err)
	require.Len(t, endpointsB, 1)
}

// TestListEndpoints_EmptyResultIsNonNil guards a real bug: a nil Go slice
// marshals to the JSON literal `null`, not `[]`, breaking any client typed
// against "always an array" for a merchant with zero registered endpoints.
func TestListEndpoints_EmptyResultIsNonNil(t *testing.T) {
	store := setupStore(t)

	endpoints, err := store.ListEndpoints(context.Background(), uuid.New())
	require.NoError(t, err)
	require.NotNil(t, endpoints)
	require.Empty(t, endpoints)
}

func TestGetDeliveryJob_JoinsEndpointAndEvent(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	merchantID := uuid.New()

	endpoint, secret, err := store.CreateEndpoint(ctx, merchantID, "https://merchant.example/hook")
	require.NoError(t, err)

	event, _, err := store.CreateEvent(ctx, merchantID, "payment.succeeded", "key-1", []byte(`{"id":"evt_1"}`))
	require.NoError(t, err)

	deliveries, err := store.CreateDeliveries(ctx, event.ID, []uuid.UUID{endpoint.ID})
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	job, err := store.GetDeliveryJob(ctx, deliveries[0].ID)
	require.NoError(t, err)
	require.Equal(t, endpoint.URL, job.EndpointURL)
	require.Equal(t, secret, job.EndpointSecret)
	require.Equal(t, "payment.succeeded", job.EventType)
	require.Equal(t, []byte(`{"id":"evt_1"}`), job.EventPayload)
	require.Equal(t, 0, job.AttemptCount)
}
