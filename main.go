package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/pocketbrain/pocketbrain/internal/app"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/setup"
	"github.com/pocketbrain/pocketbrain/internal/tui"
)

func main() {
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
