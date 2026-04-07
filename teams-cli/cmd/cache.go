package cmd

import (
	"fmt"

	"github.com/kamrul1157024/teams-cli/teams-cli/api"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local cache",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all cached data",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := api.CacheClear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Println("Cache cleared")
		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)
}
