//go:build !tsnet

package tsnet

import (
	"fmt"
	"net"
)

// Config holds Tailscale tsnet configuration.
type Config struct {
	Hostname string
	StateDir string
}

// Listener is a stub when tsnet support is not compiled in.
type Listener struct{}

// New returns an error when tsnet support is not compiled in.
// Rebuild with: go build -tags tsnet
func New(cfg Config) (*Listener, error) {
	return nil, fmt.Errorf("tsnet support not compiled; rebuild with -tags tsnet")
}

// Listen is a stub.
func (l *Listener) Listen(network, addr string) (net.Listener, error) {
	return nil, fmt.Errorf("tsnet not available")
}

// Close is a stub.
func (l *Listener) Close() error {
	return nil
}

// Available returns false when tsnet support is not compiled in.
func Available() bool {
	return false
}
