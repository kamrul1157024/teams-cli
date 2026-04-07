package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
	prettyPrint  bool
	noColor      bool
	quiet        bool
)

var rootCmd = &cobra.Command{
	Use:   "teams-cli",
	Short: "Unix-style CLI for Microsoft Teams",
	Long:  "A scriptable, pipe-friendly CLI for Microsoft Teams. Read, send, and search messages from your terminal.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&outputFormat, "format", "json", "Output format: json, table, text")
	rootCmd.PersistentFlags().BoolVar(&prettyPrint, "pretty", false, "Pretty-print JSON output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "Suppress non-essential output")
}
