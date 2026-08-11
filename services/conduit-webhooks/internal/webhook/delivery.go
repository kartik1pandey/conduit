package webhook

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"
)

// Deliverer makes the actual HTTP call for one delivery attempt. Split out
// from Worker so it's the one piece that talks to the network — everything
// else (scheduling, retry/dead-letter decisions) is pure and testable
// without a real HTTP round trip.
type Deliverer struct {
	client *http.Client
}

func NewDeliverer(timeout time.Duration) *Deliverer {
	return &Deliverer{client: &http.Client{Timeout: timeout}}
}

// Attempt POSTs job's payload to its endpoint, signed. A non-nil error means
// the request never got a response at all (timeout, connection refused,
// connection dropped mid-request) — always retryable. A nil error with any
// status code (including 4xx/5xx) means the endpoint was reached and is the
// caller's signal for whether the attempt succeeded.
func (d *Deliverer) Attempt(ctx context.Context, job DeliveryJob) (statusCode int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.EndpointURL, bytes.NewReader(job.EventPayload))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Conduit-Signature", Sign(job.EndpointSecret, job.EventPayload, time.Now()))
	req.Header.Set("Conduit-Event-Type", job.EventType)

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection can be reused; the body itself is unused

	return resp.StatusCode, nil
}
