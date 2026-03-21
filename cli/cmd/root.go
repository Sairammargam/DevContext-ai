package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "devctx",
	Short: "DevContext AI - AI-powered codebase intelligence",
	Long: `DevContext AI is a native terminal tool that gives developers
AI-powered intelligence about their codebase.

It provides semantic code search, explanations, reviews, and
contextual answers about your project.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip daemon check for init and daemon subcommands
		cmdName := cmd.Name()
		parentName := ""
		if cmd.Parent() != nil {
			parentName = cmd.Parent().Name()
		}

		if cmdName == "init" || cmdName == "daemon" || parentName == "daemon" {
			return
		}

		fmt.Println("daemon check: TODO")
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Register all subcommands
	registerCommands()
}
