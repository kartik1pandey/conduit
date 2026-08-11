package webhook

import (
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID         uuid.UUID `json:"id"`
	MerchantID uuid.UUID `json:"merchant_id"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"created_at"`
}

type Event struct {
	ID             uuid.UUID `json:"id"`
	MerchantID     uuid.UUID `json:"merchant_id"`
	Type           string    `json:"type"`
	Payload        []byte    `json:"-"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

type DeliveryStatus string

const (
	StatusPending      DeliveryStatus = "pending"
	StatusDelivered    DeliveryStatus = "delivered"
	StatusDeadLettered DeliveryStatus = "dead_lettered"
)

type Delivery struct {
	ID                 uuid.UUID      `json:"id"`
	WebhookEventID     uuid.UUID      `json:"webhook_event_id"`
	WebhookEndpointID  uuid.UUID      `json:"webhook_endpoint_id"`
	Status             DeliveryStatus `json:"status"`
	AttemptCount       int            `json:"attempt_count"`
	LastAttemptAt      *time.Time     `json:"last_attempt_at,omitempty"`
	LastResponseStatus *int           `json:"last_response_status,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}
