//go:build tsnet

package tsnet

import (
	"net"

	"tailscale.com/tsnet"
)

// Config holds Tailscale tsnet configuration.
type Config struct {
	Hostname string // Tailscale hostname, e.g. "pocketbrain"
	StateDir string // directory for tsnet state
}

// Listener wraps a tsnet.Server to provide network listeners on the Tailscale mesh.
type Listener struct {
	server *tsnet.Server
}

// New creates a new Tailscale listener.
func New(cfg Config) (*Listener, error) {
	srv := &tsnet.Server{
		Hostname: cfg.Hostname,
		Dir:      cfg.StateDir,
	}
	return &Listener{server: srv}, nil
}

// Listen returns a net.Listener bound to the Tailscale interface.
func (l *Listener) Listen(network, addr string) (net.Listener, error) {
	return l.server.Listen(network, addr)
}

// Close shuts down the Tailscale node.
func (l *Listener) Close() error {
	return l.server.Close()
}

// Available returns true when tsnet support is compiled in.
func Available() bool {
	return true
}
