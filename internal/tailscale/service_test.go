package tailscale

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"tailscale.com/drive"
	"tailscale.com/ipn/ipnstate"
)

type fakeClient struct {
	shares  []*drive.Share
	set     *drive.Share
	listErr error
	setErr  error
}

func (f *fakeClient) DriveShareList(ctx context.Context) ([]*drive.Share, error) {
	return f.shares, f.listErr
}

func (f *fakeClient) DriveShareSet(ctx context.Context, share *drive.Share) error {
	f.set = share
	return f.setErr
}

func (f *fakeClient) Status(ctx context.Context) (*ipnstate.Status, error) {
	return &ipnstate.Status{}, nil
}

type fakeNode struct {
	lc     localClient
	closed bool
}

func (f *fakeNode) Up(ctx context.Context) (*ipnstate.Status, error) {
	return &ipnstate.Status{}, nil
}

func (f *fakeNode) LocalClient() (localClient, error) {
	return f.lc, nil
}

func (f *fakeNode) Close() error {
	f.closed = true
	return nil
}

func TestServiceStartCreatesShare(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "ts")
	fc := &fakeClient{}
	fn := &fakeNode{lc: fc}

	s, err := New(Config{
		Enabled:          true,
		AuthKey:          "tskey-auth-123",
		Hostname:         "pocketbrain",
		StateDir:         state,
		TaildriveEnabled: true,
		ShareName:        "workspace",
		AutoShare:        true,
		RootDir:          root,
		Logger:           slog.Default(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.newNode = func(cfg Config) (node, error) { return fn, nil }

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fc.set == nil {
		t.Fatal("expected DriveShareSet to be called")
	}
	wantPath, _ := filepath.Abs(root)
	if fc.set.Path != wantPath {
		t.Fatalf("share path=%q want=%q", fc.set.Path, wantPath)
	}
}

func TestServiceStartNoShareWhenDisabled(t *testing.T) {
	state := filepath.Join(t.TempDir(), "ts")
	fn := &fakeNode{lc: &fakeClient{}}
	s, err := New(Config{
		Enabled:          true,
		AuthKey:          "tskey-auth-123",
		Hostname:         "pocketbrain",
		StateDir:         state,
		TaildriveEnabled: false,
		Logger:           slog.Default(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.newNode = func(cfg Config) (node, error) { return fn, nil }
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !fn.closed {
		t.Fatal("expected node to close")
	}
}

func TestValidateConfigRequiresAuth(t *testing.T) {
	root := t.TempDir()
	err := validateConfig(Config{
		Enabled:          true,
		AuthKey:          "",
		Hostname:         "pb",
		StateDir:         filepath.Join(t.TempDir(), "state"),
		TaildriveEnabled: true,
		ShareName:        "workspace",
		RootDir:          root,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateConfigTaildriveRootMustExist(t *testing.T) {
	err := validateConfig(Config{
		Enabled:          true,
		AuthKey:          "tskey-auth-1",
		Hostname:         "pb",
		StateDir:         filepath.Join(t.TempDir(), "state"),
		TaildriveEnabled: true,
		ShareName:        "workspace",
		RootDir:          filepath.Join(os.TempDir(), "does-not-exist-xyz"),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestServiceStartContinuesOnTaildriveACLDenied(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "ts")
	fc := &fakeClient{listErr: errors.New(`Access denied: taildrive sharing not enabled, please add the attribute "drive:share"`)}
	fn := &fakeNode{lc: fc}

	s, err := New(Config{
		Enabled:          true,
		AuthKey:          "tskey-auth-123",
		Hostname:         "pocketbrain",
		StateDir:         state,
		TaildriveEnabled: true,
		ShareName:        "workspace",
		AutoShare:        true,
		RootDir:          root,
		Logger:           slog.Default(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.newNode = func(cfg Config) (node, error) { return fn, nil }

	if err := s.Start(); err != nil {
		t.Fatalf("Start should not fail on ACL denial: %v", err)
	}
}

func TestServiceStartContinuesOnTaildriveNotEnabledFromShareSet(t *testing.T) {
	root := t.TempDir()
	state := filepath.Join(t.TempDir(), "ts")
	fc := &fakeClient{
		shares: []*drive.Share{},
		setErr: errors.New("500 Internal Server Error: Taildrive not enabled"),
	}
	fn := &fakeNode{lc: fc}

	s, err := New(Config{
		Enabled:          true,
		AuthKey:          "tskey-auth-123",
		Hostname:         "pocketbrain",
		StateDir:         state,
		TaildriveEnabled: true,
		ShareName:        "workspace",
		AutoShare:        true,
		RootDir:          root,
		Logger:           slog.Default(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.newNode = func(cfg Config) (node, error) { return fn, nil }

	if err := s.Start(); err != nil {
		t.Fatalf("Start should not fail when taildrive is disabled in tailnet: %v", err)
	}
}
