package authn

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type internalClaims struct {
	MerchantID string `json:"merchant_id"`
	jwt.RegisteredClaims
}

// SignInternalJWT issues a short-lived (60s) token carrying merchantID,
// which conduit-ledger verifies before trusting any request. Short-lived by
// design: this token is minted fresh for every outbound call, not cached and
// reused, so a leaked token is only useful for a minute.
func SignInternalJWT(secret string, merchantID uuid.UUID) (string, error) {
	claims := internalClaims{
		MerchantID: merchantID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(60 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
