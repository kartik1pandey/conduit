package authn

import (
	"context"

	"github.com/google/uuid"
)

type contextKey int

const merchantIDKey contextKey = iota

func withMerchantID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, merchantIDKey, id)
}

func MerchantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(merchantIDKey).(uuid.UUID)
	return id, ok
}
