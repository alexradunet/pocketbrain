package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/pocketbrain/pocketbrain/internal/app"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/setup"
	sshsrv "github.com/pocketbrain/pocketbrain/internal/ssh"
	"github.com/pocketbrain/pocketbrain/internal/tsnet"
	"github.com/pocketbrain/pocketbrain/internal/tui"
	"github.com/pocketbrain/pocketbrain/internal/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServe()
		return
	}
	runLocal()
}

// runLocal is the default mode: local TUI (current behavior).
func runLocal() {
	headless := flag.Bool("headless", false, "Run without TUI (daemon mode)")
	forceSetup := flag.Bool("setup", false, "Force run setup wizard")
	flag.Parse()

	_ = config.LoadDotEnvFile(".env")

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		*headless = true
	}

	if *headless {
		need, reason, _ := setup.NeedSetup(".env")
		if need {
			fmt.Fprintf(os.Stderr, "setup required (%s)\nRun without --headless to launch the setup wizard.\n", reason)
			os.Exit(1)
		}
		if err := app.Run(true); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	p := tea.NewProgram(
		tui.NewApp(".env", *forceSetup, app.StartBackend),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runServe starts the backend in headless mode with SSH and web terminal servers.
func runServe() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	sshAddr := fs.String("ssh-addr", "", "SSH listen address (overrides SSH_ADDR env, default :2222)")
	webAddr := fs.String("web-addr", "", "Web terminal listen address (overrides WEB_TERMINAL_ADDR env, default :8080)")
	sshOnly := fs.Bool("ssh-only", false, "Start SSH server only (no web terminal)")
	tsnetFlag := fs.Bool("tsnet", false, "Expose via Tailscale mesh (requires -tags tsnet build)")
	tsnetHostname := fs.String("tsnet-hostname", "", "Tailscale hostname (default: pocketbrain)")
	fs.Parse(os.Args[2:])

	_ = config.LoadDotEnvFile(".env")

	// Check that setup has been completed.
	need, reason, _ := setup.NeedSetup(".env")
	if need {
		fmt.Fprintf(os.Stderr, "setup required (%s)\nRun 'pocketbrain' or 'pocketbrain --setup' to complete setup first.\n", reason)
		os.Exit(1)
	}

	// Load config (reads env vars set by .env).
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	// Apply flag overrides.
	if *sshAddr != "" {
		cfg.SSHAddr = *sshAddr
	}
	if *webAddr != "" {
		cfg.WebTerminalAddr = *webAddr
	}
	if *tsnetHostname != "" {
		cfg.TsnetHostname = *tsnetHostname
	}
	if err := validateWebTerminalExposure(cfg.WebTerminalAddr, *sshOnly, *tsnetFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Start the backend (all services: DB, AI, WhatsApp, etc.).
	bus := tui.NewEventBus(512)
	cleanup, err := app.StartBackend(bus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backend: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("PocketBrain serve mode")
	logger := slog.Default()

	// Start SSH server.
	ssh, err := sshsrv.New(sshsrv.Config{
		Addr:       cfg.SSHAddr,
		HostKeyDir: cfg.DataDir,
		Logger:     logger,
	}, bus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ssh server: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	if err := ssh.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "ssh start: %v\n", err)
		cleanup()
		os.Exit(1)
	}
	bus.Publish(tui.Event{
		Type: tui.EventSSHStatus,
		Data: tui.StatusEvent{Connected: true, Detail: "listening on " + cfg.SSHAddr},
	})
	fmt.Printf("  SSH server listening on %s\n", cfg.SSHAddr)

	// Start web terminal (unless --ssh-only).
	var webSrv *web.Server
	if !*sshOnly {
		sshTarget := "127.0.0.1" + cfg.SSHAddr
		webSrv = web.New(web.Config{
			Addr:    cfg.WebTerminalAddr,
			SSHAddr: sshTarget,
			Logger:  logger,
		})
		if err := webSrv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "web server: %v\n", err)
			_ = ssh.Stop()
			cleanup()
			os.Exit(1)
		}
		bus.Publish(tui.Event{
			Type: tui.EventWebStatus,
			Data: tui.StatusEvent{Connected: true, Detail: "listening on " + cfg.WebTerminalAddr},
		})
		fmt.Printf("  Web terminal at http://localhost%s\n", cfg.WebTerminalAddr)
	}

	// Tailscale integration (optional, requires -tags tsnet build).
	var tsListener *tsnet.Listener
	if *tsnetFlag {
		if !tsnet.Available() {
			fmt.Fprintln(os.Stderr, "tsnet support not compiled; rebuild with: go build -tags tsnet")
			if webSrv != nil {
				_ = webSrv.Stop()
			}
			_ = ssh.Stop()
			cleanup()
			os.Exit(1)
		}
		tsListener, err = tsnet.New(tsnet.Config{
			Hostname: cfg.TsnetHostname,
			StateDir: cfg.DataDir + "/tsnet",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "tsnet: %v\n", err)
			if webSrv != nil {
				_ = webSrv.Stop()
			}
			_ = ssh.Stop()
			cleanup()
			os.Exit(1)
		}

		// SSH over Tailscale.
		sshLn, err := tsListener.Listen("tcp", ":22")
		if err != nil {
			fmt.Fprintf(os.Stderr, "tsnet ssh listener: %v\n", err)
		} else {
			_ = ssh.Serve(sshLn)
			fmt.Printf("  Tailscale SSH at %s:22\n", cfg.TsnetHostname)
		}

		// HTTP over Tailscale.
		if webSrv != nil {
			httpLn, err := tsListener.Listen("tcp", ":80")
			if err != nil {
				fmt.Fprintf(os.Stderr, "tsnet http listener: %v\n", err)
			} else {
				_ = webSrv.Serve(httpLn)
				fmt.Printf("  Tailscale web at http://%s\n", cfg.TsnetHostname)
			}
		}
	}

	fmt.Println("Press Ctrl+C to stop")

	// Block until interrupted.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	fmt.Printf("\nReceived %s, shutting down...\n", sig)

	// Graceful shutdown in reverse order.
	if tsListener != nil {
		_ = tsListener.Close()
	}
	if webSrv != nil {
		_ = webSrv.Stop()
	}
	_ = ssh.Stop()
	cleanup()
	fmt.Println("Shutdown complete")
}

func validateWebTerminalExposure(webAddr string, sshOnly, tsnetEnabled bool) error {
	if sshOnly || tsnetEnabled {
		return nil
	}
	if web.IsLocalOnlyAddr(webAddr) {
		return nil
	}
	return fmt.Errorf("unsafe web terminal bind %q: use --tsnet or bind to localhost only", webAddr)
}
