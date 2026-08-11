package webhook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCap_IncreasesWithAttempt verifies Checkpoint 2.2's literal criterion
// ("logs increasing delay between successive attempts") deterministically,
// against the cap that Delay draws its jitter from — the actual jittered
// delay is randomized by design (see Delay's doc comment) and isn't a value
// a test should assert monotonicity on directly.
func TestCap_IncreasesWithAttempt(t *testing.T) {
	var previous time.Duration
	for attempt := 1; attempt <= 6; attempt++ {
		capDelay := Cap(attempt)
		assert.Greater(t, capDelay, previous, "attempt %d: capDelay did not increase", attempt)
		previous = capDelay
	}
}

func TestCap_ClampsAtMaximum(t *testing.T) {
	capDelay := Cap(30) // 2^29 seconds would be enormous without a ceiling
	assert.Equal(t, backoffCap, capDelay)
}

func TestDelay_IsWithinBounds(t *testing.T) {
	for attempt := 1; attempt <= 10; attempt++ {
		capDelay := Cap(attempt)
		for i := 0; i < 50; i++ {
			d := Delay(attempt)
			assert.GreaterOrEqual(t, d, time.Duration(0))
			assert.LessOrEqual(t, d, capDelay)
		}
	}
}
