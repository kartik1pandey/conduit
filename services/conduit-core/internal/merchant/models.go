package merchant

import (
	"time"

	"github.com/google/uuid"
)

type Merchant struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	PublishableKey string    `json:"publishable_key"`
	CreatedAt      time.Time `json:"created_at"`
}
