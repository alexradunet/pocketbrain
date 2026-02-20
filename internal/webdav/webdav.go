// Package webdav provides a WebDAV file server that exposes the workspace
// directory. System-level networking (e.g. Tailscale VPN) handles
// auth and connectivity — this server itself requires no authentication.
package webdav

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"
)

// Config holds the settings for the WebDAV file-serving service.
type Config struct {
	Enabled bool
	Addr    string // listen address, e.g. "0.0.0.0:6060"
	RootDir string // workspace root path
	Logger  *slog.Logger
}

// Service manages a WebDAV server that serves the workspace directory.
type Service struct {
	cfg      Config
	server   *http.Server
	listener net.Listener
	logger   *slog.Logger
	mu       sync.Mutex
}

// New validates the config and returns a ready-to-start Service.
// When cfg.Enabled is false the returned service is a no-op stub.
func New(cfg Config) (*Service, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	if cfg.Enabled {
		if err := validateConfig(cfg); err != nil {
			return nil, fmt.Errorf("webdav config: %w", err)
		}
	}

	return &Service{
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start begins serving the workspace directory over WebDAV.
// If the service is disabled this is a no-op.
func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return errors.New("webdav: already started")
	}

	addr := s.cfg.Addr
	if addr == "" {
		addr = "127.0.0.1:6060"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("webdav listen: %w", err)
	}

	s.listener = ln
	s.server = &http.Server{
		Handler:      s.handler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("webdav server error", "error", err)
		}
	}()

	s.logger.Info("webdav started",
		"addr", ln.Addr().String(),
		"rootDir", s.cfg.RootDir,
	)

	return nil
}

// Stop gracefully shuts down the WebDAV server. If the service is disabled or
// was never started this is a no-op.
func (s *Service) Stop() error {
	if !s.cfg.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.logger.Info("webdav stopping")
	err := s.server.Shutdown(ctx)

	s.server = nil
	s.listener = nil

	if err != nil {
		return fmt.Errorf("webdav shutdown: %w", err)
	}
	return nil
}

// Addr returns the listen address (host:port) after Start has been called.
// Returns an empty string if the service is not running.
func (s *Service) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// handler builds the WebDAV handler scoped to the root directory.
func (s *Service) handler() http.Handler {
	return &webdav.Handler{
		Prefix:     "/",
		FileSystem: webdav.Dir(s.cfg.RootDir),
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				s.logger.Debug("webdav request",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err,
				)
			}
		},
	}
}

// validateConfig checks that required fields are set and the root directory
// exists on disk.
func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.RootDir) == "" {
		return errors.New("root directory must not be empty")
	}

	info, err := os.Stat(cfg.RootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("root directory does not exist: %s", cfg.RootDir)
		}
		return fmt.Errorf("root directory stat: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root path is not a directory: %s", cfg.RootDir)
	}

	return nil
}
