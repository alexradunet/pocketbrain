package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive configuration wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("PocketBrain Setup Wizard")
		fmt.Println("========================")
		fmt.Println("(Setup wizard will be implemented in a later phase)")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
