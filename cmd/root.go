package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pocketbrain",
	Short: "Autonomous personal assistant with WhatsApp, vault, and heartbeat",
	Long:  "PocketBrain is an autonomous assistant runtime with WhatsApp messaging, Markdown vault/PKM, heartbeat scheduler, and WebDAV file sharing.",
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
