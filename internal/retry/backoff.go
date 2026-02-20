package retry

import "time"

// ExponentialDelay returns base*2^(attempt-1), optionally capped by max.
// attempt is 1-based; values < 1 are treated as 1.
func ExponentialDelay(base, max time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if base <= 0 {
		return 0
	}

	delay := base * (1 << uint(attempt-1))
	if max > 0 && delay > max {
		return max
	}
	return delay
}
