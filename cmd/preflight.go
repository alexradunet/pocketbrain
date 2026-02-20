package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pocketbrain/pocketbrain/internal/setup"
	"golang.org/x/term"
)

func runSetupPreflight(headless bool) error {
	envPath := filepath.Join(".", ".env")
	need, reason, err := setup.NeedSetup(envPath)
	if err != nil {
		return err
	}
	if !need {
		return nil
	}

	if headless || !stdinIsTerminal() {
		return fmt.Errorf("setup required (%s). Run `pocketbrain setup` first", reason)
	}

	fmt.Printf("Configuration incomplete (%s). Launching setup wizard...\n", reason)
	w := setup.NewWizard(os.Stdin, os.Stdout)
	return w.Run(envPath)
}

func stdinIsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
