package cmd

import (
	"fmt"
	"os"

	"github.com/pocketbrain/pocketbrain/internal/app"
	"github.com/spf13/cobra"
)

var headless bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Aliases: []string{"start"},
	Short: "Start PocketBrain (TUI mode by default, --headless for daemon)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runSetupPreflight(headless); err != nil {
			return err
		}
		return app.Run(headless)
	},
}

func init() {
	serveCmd.Flags().BoolVar(&headless, "headless", false, "Run without TUI (daemon mode for systemd/Docker)")
	rootCmd.AddCommand(serveCmd)

	// Make serve the default command when no subcommand given.
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Check if user has a TERM for TUI.
		if os.Getenv("TERM") == "" {
			headless = true
		}
		if err := runSetupPreflight(headless); err != nil {
			return err
		}
		fmt.Println("Starting PocketBrain...")
		return app.Run(headless)
	}
}
