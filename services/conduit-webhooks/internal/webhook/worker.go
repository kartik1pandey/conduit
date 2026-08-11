package webhook

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

// Worker drives delivery attempts: pop what's due from the retry schedule,
// attempt each, and decide whether it's delivered, retried, or
// dead-lettered. now is a field (not a direct time.Now() call) so tests can
// fast-forward it instead of actually sleeping through real backoff delays.
type Worker struct {
	store       *Store
	queue       *RetryQueue
	deliverer   *Deliverer
	maxAttempts int
	now         func() time.Time
}

func NewWorker(store *Store, queue *RetryQueue, deliverer *Deliverer, maxAttempts int) *Worker {
	return &Worker{
		store:       store,
		queue:       queue,
		deliverer:   deliverer,
		maxAttempts: maxAttempts,
		now:         time.Now,
	}
}

// ScheduleFirstAttempt marks a freshly created delivery as due immediately.
func (w *Worker) ScheduleFirstAttempt(ctx context.Context, deliveryID uuid.UUID) error {
	return w.queue.Schedule(ctx, deliveryID, w.now())
}

// ProcessOnce attempts every currently-due delivery once and returns how
// many it processed. Called on a ticker in production (cmd/server/main.go);
// called directly and repeatedly by tests, which control time via w.now
// instead of waiting on a ticker.
func (w *Worker) ProcessOnce(ctx context.Context) (int, error) {
	due, err := w.queue.PopDue(ctx, w.now(), 100)
	if err != nil {
		return 0, err
	}
	for _, deliveryID := range due {
		w.attemptOne(ctx, deliveryID)
	}
	return len(due), nil
}

func (w *Worker) attemptOne(ctx context.Context, deliveryID uuid.UUID) {
	job, err := w.store.GetDeliveryJob(ctx, deliveryID)
	if err != nil {
		log.Printf("webhooks: could not load delivery job %s: %v", deliveryID, err)
		return
	}

	statusCode, attemptErr := w.deliverer.Attempt(ctx, job)
	attemptNumber := job.AttemptCount + 1
	now := w.now()

	if attemptErr == nil && statusCode >= 200 && statusCode < 300 {
		if err := w.store.RecordAttempt(ctx, deliveryID, now, &statusCode, StatusDelivered); err != nil {
			log.Printf("webhooks: could not record successful delivery %s: %v", deliveryID, err)
		}
		return
	}

	var responseStatus *int
	if attemptErr == nil {
		responseStatus = &statusCode
	}

	if attemptNumber >= w.maxAttempts {
		if err := w.store.RecordAttempt(ctx, deliveryID, now, responseStatus, StatusDeadLettered); err != nil {
			log.Printf("webhooks: could not record dead-lettered delivery %s: %v", deliveryID, err)
		}
		return
	}

	if err := w.store.RecordAttempt(ctx, deliveryID, now, responseStatus, StatusPending); err != nil {
		log.Printf("webhooks: could not record failed attempt for delivery %s: %v", deliveryID, err)
		return
	}
	delay := Delay(attemptNumber)
	log.Printf("webhooks: delivery %s attempt %d failed, retrying in %s (cap %s)", deliveryID, attemptNumber, delay, Cap(attemptNumber))
	if err := w.queue.Schedule(ctx, deliveryID, now.Add(delay)); err != nil {
		log.Printf("webhooks: could not reschedule delivery %s: %v", deliveryID, err)
	}
}
