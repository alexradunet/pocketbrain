package main

import (
	"testing"

	"github.com/pocketbrain/pocketbrain/internal/config"
)

func TestValidateWebTerminalExposure(t *testing.T) {
	tests := []struct {
		name      string
		webAddr   string
		sshOnly   bool
		tsnetFlag bool
		wantErr   bool
	}{
		{
			name:      "ssh only skips web checks",
			webAddr:   "0.0.0.0:8080",
			sshOnly:   true,
			tsnetFlag: false,
			wantErr:   false,
		},
		{
			name:      "tsnet allows non-local bind",
			webAddr:   "0.0.0.0:8080",
			sshOnly:   false,
			tsnetFlag: true,
			wantErr:   false,
		},
		{
			name:      "non-local bind without tsnet rejected",
			webAddr:   "0.0.0.0:8080",
			sshOnly:   false,
			tsnetFlag: false,
			wantErr:   true,
		},
		{
			name:      "localhost bind without tsnet allowed",
			webAddr:   "127.0.0.1:8080",
			sshOnly:   false,
			tsnetFlag: false,
			wantErr:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebTerminalExposure(tc.webAddr, tc.sshOnly, tc.tsnetFlag)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateWebTerminalExposure error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestApplyServeOverrides(t *testing.T) {
	cfg := &config.Config{
		SSHAddr:         ":2222",
		WebTerminalAddr: ":8080",
		TsnetHostname:   "pocketbrain",
	}

	applyServeOverrides(cfg, ":3333", "", "pb-node")
	if cfg.SSHAddr != ":3333" {
		t.Fatalf("SSHAddr = %q, want %q", cfg.SSHAddr, ":3333")
	}
	if cfg.WebTerminalAddr != ":8080" {
		t.Fatalf("WebTerminalAddr = %q, want %q", cfg.WebTerminalAddr, ":8080")
	}
	if cfg.TsnetHostname != "pb-node" {
		t.Fatalf("TsnetHostname = %q, want %q", cfg.TsnetHostname, "pb-node")
	}
}
