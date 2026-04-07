package cmd

import (
	"fmt"
	"os"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/spf13/cobra"
)

// newClient creates an API client with the current cache config flags
func newClient() *api.Client {
	return api.NewClientWithCache(api.CacheConfig{
		Enabled: !noCache,
		Refresh: refreshCache,
	})
}

var (
	outputFormat string
	prettyPrint  bool
	noColor      bool
	quiet        bool
	noCache      bool
	refreshCache bool
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
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "Bypass cache for this request")
	rootCmd.PersistentFlags().BoolVar(&refreshCache, "refresh", false, "Ignore cache and write fresh data")
}
