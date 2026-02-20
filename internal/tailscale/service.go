package tailscale

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tailscale.com/drive"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

type Config struct {
	Enabled          bool
	AuthKey          string
	Hostname         string
	StateDir         string
	TaildriveEnabled bool
	ShareName        string
	AutoShare        bool
	RootDir          string
	Logger           *slog.Logger
}

type localClient interface {
	DriveShareList(ctx context.Context) ([]*drive.Share, error)
	DriveShareSet(ctx context.Context, share *drive.Share) error
	Status(ctx context.Context) (*ipnstate.Status, error)
}

type node interface {
	Up(ctx context.Context) (*ipnstate.Status, error)
	LocalClient() (localClient, error)
	Close() error
}

type tsnetNode struct {
	s *tsnet.Server
}

func (n *tsnetNode) Up(ctx context.Context) (*ipnstate.Status, error) {
	return n.s.Up(ctx)
}

func (n *tsnetNode) LocalClient() (localClient, error) {
	return n.s.LocalClient()
}

func (n *tsnetNode) Close() error {
	return n.s.Close()
}

type Service struct {
	cfg     Config
	logger  *slog.Logger
	mu      sync.Mutex
	started bool
	node    node

	newNode func(Config) (node, error)
}

func New(cfg Config) (*Service, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Service{
		cfg:    cfg,
		logger: logger,
		newNode: func(cfg Config) (node, error) {
			if cfg.Enabled && cfg.AuthKey == "" {
				return nil, errors.New("auth key is required when enabled")
			}
			return &tsnetNode{&tsnet.Server{
				Dir:      cfg.StateDir,
				Hostname: cfg.Hostname,
				AuthKey:  cfg.AuthKey,
			}}, nil
		},
	}
	if cfg.Enabled {
		if err := validateConfig(cfg); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Service) Start() error {
	if !s.cfg.Enabled {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("tailscale: already started")
	}

	n, err := s.newNode(s.cfg)
	if err != nil {
		return fmt.Errorf("new tsnet node: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	status, err := n.Up(ctx)
	if err != nil {
		_ = n.Close()
		return fmt.Errorf("tailscale up: %w", err)
	}

	if s.cfg.TaildriveEnabled && s.cfg.AutoShare {
		lc, err := n.LocalClient()
		if err != nil {
			_ = n.Close()
			return fmt.Errorf("tailscale local client: %w", err)
		}
		if err := ensureShare(ctx, lc, s.cfg.ShareName, s.cfg.RootDir); err != nil {
			_ = n.Close()
			return err
		}
	}

	s.node = n
	s.started = true
	if status != nil && status.Self != nil {
		s.logger.Info("embedded tailscale started", "dnsName", status.Self.DNSName, "hostname", s.cfg.Hostname)
	} else {
		s.logger.Info("embedded tailscale started", "hostname", s.cfg.Hostname)
	}
	return nil
}

func (s *Service) Stop() error {
	if !s.cfg.Enabled {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started || s.node == nil {
		return nil
	}
	err := s.node.Close()
	s.started = false
	s.node = nil
	if err != nil {
		return fmt.Errorf("tailscale close: %w", err)
	}
	return nil
}

func ensureShare(ctx context.Context, lc localClient, shareName, rootDir string) error {
	shares, err := lc.DriveShareList(ctx)
	if err != nil {
		return fmt.Errorf("drive share list: %w", err)
	}

	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("root dir abs: %w", err)
	}
	for _, s := range shares {
		if s != nil && s.Name == shareName && s.Path == rootAbs {
			return nil
		}
	}
	if err := lc.DriveShareSet(ctx, &drive.Share{
		Name: shareName,
		Path: rootAbs,
	}); err != nil {
		return fmt.Errorf("drive share set: %w", err)
	}
	return nil
}

func validateConfig(cfg Config) error {
	if cfg.AuthKey == "" {
		return errors.New("TS_AUTHKEY must be set")
	}
	if cfg.Hostname == "" {
		return errors.New("TS_HOSTNAME must be set")
	}
	if cfg.StateDir == "" {
		return errors.New("TS_STATE_DIR must be set")
	}
	if err := os.MkdirAll(cfg.StateDir, 0o755); err != nil {
		return fmt.Errorf("mkdir state dir: %w", err)
	}
	if cfg.TaildriveEnabled {
		if cfg.ShareName == "" {
			return errors.New("TAILDRIVE_SHARE_NAME must be set")
		}
		if cfg.RootDir == "" {
			return errors.New("workspace root must be set")
		}
		if info, err := os.Stat(cfg.RootDir); err != nil || !info.IsDir() {
			if err != nil {
				return fmt.Errorf("workspace root: %w", err)
			}
			return errors.New("workspace root is not a directory")
		}
	}
	return nil
}
