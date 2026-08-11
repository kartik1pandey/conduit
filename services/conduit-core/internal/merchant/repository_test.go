package merchant

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/db"
	"github.com/kartik1pandey/conduit/services/conduit-core/migrations"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	dbURL := os.Getenv("CORE_DATABASE_URL")
	if dbURL == "" {
		t.Skip("CORE_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, db.Migrate(ctx, pool, migrations.FS, "."))
	_, err = pool.Exec(ctx, "TRUNCATE idempotency_keys, payment_intents, merchants CASCADE")
	require.NoError(t, err)

	return NewStore(pool)
}

func TestCreateAndAuthenticate(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	m, secretKey, err := store.Create(ctx, "Acme Corp")
	require.NoError(t, err)
	require.NotEmpty(t, secretKey)
	require.Contains(t, secretKey, "sk_test_")
	require.Contains(t, m.PublishableKey, "pk_test_")

	merchantID, err := store.AuthenticateBySecretKey(ctx, secretKey)
	require.NoError(t, err)
	require.Equal(t, m.ID, merchantID)
}

func TestAuthenticateWithWrongKeyFails(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	_, _, err := store.Create(ctx, "Acme Corp")
	require.NoError(t, err)

	_, err = store.AuthenticateBySecretKey(ctx, "sk_test_this_key_does_not_exist")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestSecretKeysAreUniquePerMerchant(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	_, keyA, err := store.Create(ctx, "Merchant A")
	require.NoError(t, err)
	_, keyB, err := store.Create(ctx, "Merchant B")
	require.NoError(t, err)

	require.NotEqual(t, keyA, keyB)
}

func TestGet(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	created, _, err := store.Create(ctx, "Acme Corp")
	require.NoError(t, err)

	fetched, err := store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created, fetched)
}

func TestGetUnknownIDFails(t *testing.T) {
	store := setupStore(t)

	_, err := store.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, ErrNotFound)
}
