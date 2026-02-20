package web

import "time"

// deadlineForInitialResize returns a short deadline for reading the initial
// resize message from the WebSocket client.
func deadlineForInitialResize() time.Time {
	return time.Now().Add(2 * time.Second)
}

// noDeadline returns the zero time, disabling any read deadline.
func noDeadline() time.Time {
	return time.Time{}
}
