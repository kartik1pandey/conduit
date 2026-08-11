package webhook

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignAndVerify(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_1","type":"payment.succeeded"}`)
	at := time.Unix(1_700_000_000, 0)

	header := Sign(secret, payload, at)
	assert.True(t, strings.HasPrefix(header, "t=1700000000,v1="))
	assert.True(t, Verify(secret, payload, header))
}

func TestVerify_RejectsTamperedPayload(t *testing.T) {
	secret := "whsec_test"
	header := Sign(secret, []byte(`{"amount":"10.00"}`), time.Now())

	assert.False(t, Verify(secret, []byte(`{"amount":"99999.00"}`), header))
}

func TestVerify_RejectsWrongSecret(t *testing.T) {
	payload := []byte(`{"id":"evt_1"}`)
	header := Sign("whsec_correct", payload, time.Now())

	assert.False(t, Verify("whsec_wrong", payload, header))
}

func TestVerify_RejectsMalformedHeader(t *testing.T) {
	payload := []byte(`{"id":"evt_1"}`)

	for _, header := range []string{"", "garbage", "t=abc,v1=deadbeef", "v1=deadbeef"} {
		assert.False(t, Verify("whsec_test", payload, header), "header %q should not verify", header)
	}
}
