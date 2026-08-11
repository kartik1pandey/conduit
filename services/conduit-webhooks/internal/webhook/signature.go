package webhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Sign produces a Conduit-Signature header value, mirroring Stripe's own
// Stripe-Signature scheme: "t={unix_timestamp},v1={hex_hmac_sha256}" where
// the HMAC is computed over "{timestamp}.{payload}", not the payload alone.
// Binding the timestamp into the signed content is what lets a receiver
// reject a replayed-months-later payload even if the raw signature bytes
// were somehow intercepted — the signature itself proves *when* it was
// generated, not just that the secret matches.
func Sign(secret string, payload []byte, at time.Time) string {
	ts := at.Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

// Verify checks a Conduit-Signature header against payload and secret. It
// does not enforce a tolerance window on the timestamp — see the note on
// that tradeoff in docs/learning.
func Verify(secret string, payload []byte, header string) bool {
	ts, sig, ok := parseSignatureHeader(header)
	if !ok {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	mac.Write(payload)
	expected := mac.Sum(nil)

	got, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	// hmac.Equal runs in constant time regardless of where the first
	// mismatched byte is — a naive == comparison leaks timing information
	// an attacker could use to guess the signature byte-by-byte.
	return hmac.Equal(expected, got)
}

func parseSignatureHeader(header string) (int64, string, bool) {
	var ts int64
	var sig string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			parsed, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return 0, "", false
			}
			ts = parsed
		case "v1":
			sig = kv[1]
		}
	}
	if sig == "" {
		return 0, "", false
	}
	return ts, sig, true
}

// GenerateSecret creates a new per-endpoint HMAC secret — see
// migrations/0001_init.up.sql for why this is per-endpoint, not global.
func GenerateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(buf), nil
}
