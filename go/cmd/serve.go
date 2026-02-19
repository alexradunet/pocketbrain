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
	Short: "Start PocketBrain (TUI mode by default, --headless for daemon)",
	RunE: func(cmd *cobra.Command, args []string) error {
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
		fmt.Println("Starting PocketBrain...")
		return app.Run(headless)
	}
}
