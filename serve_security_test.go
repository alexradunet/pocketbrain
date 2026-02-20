package main

import "testing"

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
