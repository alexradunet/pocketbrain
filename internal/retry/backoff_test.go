package retry

import (
	"testing"
	"time"
)

func TestExponentialDelay(t *testing.T) {
	base := 100 * time.Millisecond

	if got := ExponentialDelay(base, 0, 1); got != 100*time.Millisecond {
		t.Fatalf("attempt 1 = %v, want 100ms", got)
	}
	if got := ExponentialDelay(base, 0, 2); got != 200*time.Millisecond {
		t.Fatalf("attempt 2 = %v, want 200ms", got)
	}
	if got := ExponentialDelay(base, 0, 3); got != 400*time.Millisecond {
		t.Fatalf("attempt 3 = %v, want 400ms", got)
	}
}

func TestExponentialDelay_MaxCap(t *testing.T) {
	got := ExponentialDelay(100*time.Millisecond, 250*time.Millisecond, 3)
	if got != 250*time.Millisecond {
		t.Fatalf("delay = %v, want 250ms cap", got)
	}
}

func TestExponentialDelay_InvalidAttemptAndBase(t *testing.T) {
	if got := ExponentialDelay(0, 0, 3); got != 0 {
		t.Fatalf("zero base delay = %v, want 0", got)
	}
	if got := ExponentialDelay(100*time.Millisecond, 0, 0); got != 100*time.Millisecond {
		t.Fatalf("attempt 0 delay = %v, want 100ms", got)
	}
}
