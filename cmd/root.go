package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Parallel AI agent workspace manager",
	Long:  "Swarm manages multiple AI coding agents in parallel using git worktrees and tmux.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("swarm: scaffold running")
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
