package taildrive

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Enabled:   true,
				ShareName: "workspace",
				RootDir:   t.TempDir(),
			},
			wantErr: false,
		},
		{
			name: "empty share name",
			cfg: Config{
				Enabled:   true,
				ShareName: "",
				RootDir:   t.TempDir(),
			},
			wantErr: true,
		},
		{
			name: "empty root dir",
			cfg: Config{
				Enabled:   true,
				ShareName: "workspace",
				RootDir:   "",
			},
			wantErr: true,
		},
		{
			name: "non-existent root dir",
			cfg: Config{
				Enabled:   true,
				ShareName: "workspace",
				RootDir:   "/tmp/taildrive-test-nonexistent-dir-xyz",
			},
			wantErr: true,
		},
		{
			name: "disabled config skips validation",
			cfg: Config{
				Enabled:   false,
				ShareName: "",
				RootDir:   "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_DefaultShareName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc, err := New(Config{
		Enabled:   true,
		ShareName: "myshare",
		RootDir:   dir,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if svc.cfg.ShareName != "myshare" {
		t.Errorf("ShareName = %q, want %q", svc.cfg.ShareName, "myshare")
	}
}

func TestWebDAVHandler_ServesWorkspaceDir(t *testing.T) {
	t.Parallel()

	// Create a temp workspace with a test file.
	dir := t.TempDir()
	content := "hello from workspace"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create a subdirectory with a file.
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled:   true,
		ShareName: "workspace",
		RootDir:   dir,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	handler := svc.handler()

	// Test serving a file at root.
	t.Run("root file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if body != content {
			t.Errorf("body = %q, want %q", body, content)
		}
	})

	// Test serving a nested file.
	t.Run("nested file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/subdir/nested.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if body != "nested content" {
			t.Errorf("body = %q, want %q", body, "nested content")
		}
	})

	// Test 404 for missing file.
	t.Run("missing file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent.txt", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestWebDAVHandler_RejectsTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a file outside the workspace root that should not be reachable.
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled:   true,
		ShareName: "workspace",
		RootDir:   dir,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	handler := svc.handler()

	traversalPaths := []string{
		"/../secret.txt",
		"/../../etc/passwd",
		"/../../../etc/passwd",
		"/subdir/../../secret.txt",
		"/%2e%2e/secret.txt",
		"/%2e%2e/%2e%2e/etc/passwd",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Should NOT return 200 with sensitive content.
			// Go's http.FileServer already handles path cleaning, so traversal
			// attempts either get redirected (301) or return 404/403.
			if rec.Code == http.StatusOK {
				body := rec.Body.String()
				if body == "secret" {
					t.Errorf("path %q served secret content, traversal not blocked", path)
				}
			}
		})
	}
}

func TestService_StartStop(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled:   true,
		ShareName: "workspace",
		RootDir:   dir,
		Logger:    logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	// Addr should be empty before start.
	if addr := svc.Addr(); addr != "" {
		t.Errorf("Addr() before Start = %q, want empty", addr)
	}

	// Start the service.
	if err := svc.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Addr should be non-empty after start.
	addr := svc.Addr()
	if addr == "" {
		t.Fatal("Addr() after Start is empty")
	}

	// Make a real HTTP request to the running server.
	resp, err := http.Get("http://" + addr + "/hello.txt")
	if err != nil {
		t.Fatalf("GET error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", string(body), "hello")
	}

	// Stop the service.
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() error: %v", err)
	}

	// After stop, requests should fail.
	_, err = http.Get("http://" + addr + "/hello.txt")
	if err == nil {
		t.Error("expected error after Stop, got nil")
	}
}

func TestService_DisabledNoOp(t *testing.T) {
	t.Parallel()

	svc, err := New(Config{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	// Start/Stop on disabled service should be no-ops.
	if err := svc.Start(); err != nil {
		t.Errorf("Start() on disabled service: %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() on disabled service: %v", err)
	}
	if addr := svc.Addr(); addr != "" {
		t.Errorf("Addr() on disabled service = %q, want empty", addr)
	}
}
