package cmd

import (
	"os"
	"path/filepath"

	"github.com/pocketbrain/pocketbrain/internal/setup"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive configuration wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		w := setup.NewWizard(os.Stdin, os.Stdout)
		return w.Run(filepath.Join(".", ".env"))
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
