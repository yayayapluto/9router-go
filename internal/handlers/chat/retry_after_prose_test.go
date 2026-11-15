package chat

import (
	"testing"
	"time"
)

// TestProseRetry covers the "retry after|in N unit" prose shape. Every case
// routes through extractResetDuration because that is the single entry point
// both the combo loop and the account fallback path use, and it applies the
// 5s..2h clamp; the sub-5s cases below therefore assert the clamped floor.
func TestProseRetry(t *testing.T) {
	cases := []struct {
		name string
		body string
		want time.Duration
	}{
		{"seconds", `{"error":{"message":"rate limited, retry in 30s"}}`, 30 * time.Second},
		{"fractional seconds clamps to floor", `{"error":{"message":"rate limited, retry after 1.5s"}}`, minResetCooldown},
		{"minutes", `{"error":{"message":"busy, retry in 2 min"}}`, 2 * time.Minute},
		{"milliseconds clamp to floor", `{"error":{"message":"retry after 500ms"}}`, minResetCooldown},
		{"resets in wins", `{"error":{"message":"resets in 10m, retry in 5s"}}`, 10 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := extractResetDuration([]byte(tc.body))
			if !ok {
				t.Fatalf("expected a duration from %s", tc.body)
			}
			if got != tc.want {
				t.Errorf("extractResetDuration(%s) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

// TestProseRetryRejectsNonPositive pins that a zero second count is not a
// usable wait: the caller must fall through to other detection.
func TestProseRetryRejectsNonPositive(t *testing.T) {
	if d, ok := proseRetryDelay([]byte("retry in 0s")); ok {
		t.Errorf("expected no duration for retry in 0s, got %v", d)
	}
}

// FuzzProseRetryDelay keeps the prose parser total: any byte string yields
// either no delay or a delay inside the clamp window extractResetDuration
// promises, never a panic.
func FuzzProseRetryDelay(f *testing.F) {
	f.Add([]byte(`{"error":{"message":"rate limited, retry in 30s"}}`))
	f.Add([]byte("retry after 1.5s"))
	f.Add([]byte("retry in 999999 min"))
	f.Add([]byte{0x00, 0xff, 0x7f})

	f.Fuzz(func(t *testing.T, body []byte) {
		d, ok := proseRetryDelay(body)
		if !ok {
			return
		}
		if d < minResetCooldown || d > maxResetCooldown {
			t.Fatalf("delay %s outside [%s, %s]", d, minResetCooldown, maxResetCooldown)
		}
	})
}
