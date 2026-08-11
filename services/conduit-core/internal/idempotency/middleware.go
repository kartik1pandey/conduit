package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/kartik1pandey/conduit/services/conduit-core/internal/authn"
)

// RequireKey enforces the Idempotency-Key header on the wrapped handler: a
// missing header is rejected outright, a fresh key runs the handler and
// stores its response, and a repeated key replays the stored response
// without running the handler again. httptest.ResponseRecorder is used here
// as a general-purpose response-capturing http.ResponseWriter — the same
// interface tests use, repurposed because it's exactly what "run the
// handler, then decide what to do with its output" needs.
func RequireKey(store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			merchantID, ok := authn.MerchantIDFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"missing merchant context"}`, http.StatusUnauthorized)
				return
			}

			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				http.Error(w, `{"error":"Idempotency-Key header is required"}`, http.StatusBadRequest)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, `{"error":"could not read request body"}`, http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			requestHash := hashRequest(r.Method, r.URL.Path, bodyBytes)

			claimed, existing, err := store.Claim(r.Context(), merchantID, key, requestHash)
			switch {
			case errors.Is(err, ErrKeyReused):
				http.Error(w, `{"error":"Idempotency-Key was already used with different request parameters"}`, http.StatusUnprocessableEntity)
				return
			case errors.Is(err, ErrInFlight):
				http.Error(w, `{"error":"a request with this Idempotency-Key is already being processed"}`, http.StatusConflict)
				return
			case err != nil:
				http.Error(w, `{"error":"idempotency check failed"}`, http.StatusInternalServerError)
				return
			}

			if !claimed {
				// existing is guaranteed non-nil and filled here: Claim only
				// returns claimed=false, err=nil once a stored response exists.
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Idempotency-Replayed", "true")
				w.WriteHeader(*existing.ResponseStatus)
				_, _ = w.Write(existing.ResponseBody)
				return
			}

			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)

			if err := store.Fill(r.Context(), merchantID, key, rec.Code, rec.Body.Bytes()); err != nil {
				// The handler already ran; the client still gets its real
				// response. A failure here only means a *future* retry with
				// this key won't find a cached response until it's reclaimed
				// as stale.
				log.Printf("idempotency: failed to store response for key %q: %v", key, err)
			}

			for k, vs := range rec.Header() {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
		})
	}
}

func hashRequest(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte(path))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
