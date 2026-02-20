package ssh

import (
	"fmt"
	"log/slog"
	"net"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"

	"github.com/pocketbrain/pocketbrain/internal/tui"
)

// Config holds SSH server configuration.
type Config struct {
	Addr       string // listen address, e.g. ":2222"
	HostKeyDir string // directory to store/read host keys
	Logger     *slog.Logger
}

// Server wraps a Wish SSH server that serves Bubble Tea TUI sessions.
type Server struct {
	srv    *ssh.Server
	logger *slog.Logger
}

// New creates a new SSH server. Each SSH connection gets its own TUI model
// subscribed to the shared EventBus.
func New(cfg Config, bus *tui.EventBus) (*Server, error) {
	srv, err := wish.NewServer(
		wish.WithAddress(cfg.Addr),
		wish.WithHostKeyPath(cfg.HostKeyDir+"/ssh_host_key"),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				pty, _, _ := s.Pty()
				model := tui.New(bus)
				model.SetSize(pty.Window.Width, pty.Window.Height)
				return model, []tea.ProgramOption{tea.WithAltScreen()}
			}),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("wish server: %w", err)
	}

	return &Server{srv: srv, logger: cfg.Logger}, nil
}

// Start begins accepting SSH connections on the configured address.
func (s *Server) Start() error {
	s.logger.Info("SSH server listening", "addr", s.srv.Addr)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil {
			s.logger.Error("SSH server stopped", "error", err)
		}
	}()
	return nil
}

// Serve accepts SSH connections on the given listener (e.g. from tsnet).
func (s *Server) Serve(ln net.Listener) error {
	s.logger.Info("SSH server listening", "addr", ln.Addr().String())
	go func() {
		if err := s.srv.Serve(ln); err != nil {
			s.logger.Error("SSH server stopped", "error", err)
		}
	}()
	return nil
}

// Stop gracefully shuts down the SSH server.
func (s *Server) Stop() error {
	return s.srv.Close()
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.srv.Addr
}
