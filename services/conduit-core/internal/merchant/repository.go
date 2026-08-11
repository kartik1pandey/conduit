package merchant

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("merchant not found")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create provisions a new merchant with a test-mode key pair and returns the
// plaintext secret key exactly once — like Stripe, it's never retrievable
// again after this call, only the hash is persisted.
//
// This endpoint exists to bootstrap merchants at all: dashboard-based
// signup (docs/CHECKPOINTS.md Phase 4) doesn't exist yet, and ARCHITECTURE.md
// doesn't specify another provisioning path for Phase 1. It's deliberately
// unauthenticated for now — acceptable because CONDUIT_MODE is always "test"
// and there is no live-money path anywhere in this project — and should be
// gated or removed once the dashboard provides real onboarding.
func (s *Store) Create(ctx context.Context, name string) (Merchant, string, error) {
	secretKey, err := generateKey("sk_test_")
	if err != nil {
		return Merchant{}, "", fmt.Errorf("generating secret key: %w", err)
	}
	publishableKey, err := generateKey("pk_test_")
	if err != nil {
		return Merchant{}, "", fmt.Errorf("generating publishable key: %w", err)
	}

	var m Merchant
	err = s.pool.QueryRow(ctx, `
		INSERT INTO merchants (name, secret_key_hash, publishable_key)
		VALUES ($1, $2, $3)
		RETURNING id, name, publishable_key, created_at
	`, name, hashKey(secretKey), publishableKey).Scan(&m.ID, &m.Name, &m.PublishableKey, &m.CreatedAt)
	if err != nil {
		return Merchant{}, "", fmt.Errorf("creating merchant: %w", err)
	}
	return m, secretKey, nil
}

// AuthenticateBySecretKey resolves a plaintext secret key to a merchant_id.
// Implements authn.MerchantAuthenticator.
func (s *Store) AuthenticateBySecretKey(ctx context.Context, secretKey string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM merchants WHERE secret_key_hash = $1
	`, hashKey(secretKey)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("looking up merchant: %w", err)
	}
	return id, nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func generateKey(prefix string) (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}
