package main

import (
	"github.com/pocketbrain/pocketbrain/internal/config"
	sshsrv "github.com/pocketbrain/pocketbrain/internal/ssh"
	"github.com/pocketbrain/pocketbrain/internal/tsnet"
	"github.com/pocketbrain/pocketbrain/internal/web"
)

func applyServeOverrides(cfg *config.Config, sshAddr, webAddr, tsnetHostname string) {
	if sshAddr != "" {
		cfg.SSHAddr = sshAddr
	}
	if webAddr != "" {
		cfg.WebTerminalAddr = webAddr
	}
	if tsnetHostname != "" {
		cfg.TsnetHostname = tsnetHostname
	}
}

func shutdownServeComponents(tsListener *tsnet.Listener, webSrv *web.Server, ssh *sshsrv.Server, cleanup func()) {
	if tsListener != nil {
		_ = tsListener.Close()
	}
	if webSrv != nil {
		_ = webSrv.Stop()
	}
	if ssh != nil {
		_ = ssh.Stop()
	}
	if cleanup != nil {
		cleanup()
	}
}
