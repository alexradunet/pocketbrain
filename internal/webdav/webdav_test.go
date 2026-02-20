package webdav

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
				Enabled: true,
				Addr:    "127.0.0.1:0",
				RootDir: t.TempDir(),
			},
			wantErr: false,
		},
		{
			name: "empty root dir",
			cfg: Config{
				Enabled: true,
				Addr:    "127.0.0.1:0",
				RootDir: "",
			},
			wantErr: true,
		},
		{
			name: "non-existent root dir",
			cfg: Config{
				Enabled: true,
				Addr:    "127.0.0.1:0",
				RootDir: "/tmp/webdav-test-nonexistent-dir-xyz",
			},
			wantErr: true,
		},
		{
			name: "disabled config skips validation",
			cfg: Config{
				Enabled: false,
				RootDir: "",
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

func TestHandler_PROPFIND(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled: true,
		Addr:    "127.0.0.1:0",
		RootDir: dir,
		Logger:  logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	handler := svc.handler()

	req := httptest.NewRequest("PROPFIND", "/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// WebDAV PROPFIND returns 207 Multi-Status.
	if rec.Code != http.StatusMultiStatus {
		t.Errorf("PROPFIND status = %d, want %d", rec.Code, http.StatusMultiStatus)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "test.txt") {
		t.Errorf("PROPFIND response should contain test.txt, got:\n%s", body)
	}
}

func TestHandler_GETServesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "hello from workspace"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled: true,
		Addr:    "127.0.0.1:0",
		RootDir: dir,
		Logger:  logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	handler := svc.handler()

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
}

func TestHandler_PUTCreatesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled: true,
		Addr:    "127.0.0.1:0",
		RootDir: dir,
		Logger:  logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	handler := svc.handler()

	req := httptest.NewRequest(http.MethodPut, "/new-file.txt", strings.NewReader("new content"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusNoContent {
		t.Errorf("PUT status = %d, want 201 or 204", rec.Code)
	}

	data, err := os.ReadFile(filepath.Join(dir, "new-file.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}
}

func TestHandler_RejectsTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc, err := New(Config{
		Enabled: true,
		Addr:    "127.0.0.1:0",
		RootDir: dir,
		Logger:  logger,
	})
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	handler := svc.handler()

	traversalPaths := []string{
		"/../secret.txt",
		"/../../etc/passwd",
		"/subdir/../../secret.txt",
	}

	for _, path := range traversalPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

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
		Enabled: true,
		Addr:    "127.0.0.1:0",
		RootDir: dir,
		Logger:  logger,
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
