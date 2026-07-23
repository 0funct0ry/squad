package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information and exit",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("squad version %s (commit: %s)\n", Version, CommitSHA)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
