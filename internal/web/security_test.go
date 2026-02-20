package web

import (
	"net/http"
	"testing"
)

func TestIsLocalOnlyAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8080", want: true},
		{addr: "localhost:8080", want: true},
		{addr: "[::1]:8080", want: true},
		{addr: "0.0.0.0:8080", want: false},
		{addr: ":8080", want: false},
		{addr: "10.0.0.1:8080", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.addr, func(t *testing.T) {
			got := IsLocalOnlyAddr(tc.addr)
			if got != tc.want {
				t.Fatalf("IsLocalOnlyAddr(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

func TestWebsocketCheckOrigin(t *testing.T) {
	t.Run("same host allowed", func(t *testing.T) {
		r := &http.Request{
			Header: http.Header{"Origin": []string{"http://localhost:8080"}},
			Host:   "localhost:8080",
		}
		if !websocketCheckOrigin(r) {
			t.Fatal("expected same-host origin to be allowed")
		}
	})

	t.Run("different host rejected", func(t *testing.T) {
		r := &http.Request{
			Header: http.Header{"Origin": []string{"https://evil.example.com"}},
			Host:   "localhost:8080",
		}
		if websocketCheckOrigin(r) {
			t.Fatal("expected cross-host origin to be rejected")
		}
	})
}
