package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/devctx/cli/internal/config"
	"github.com/devctx/cli/internal/wizard"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// registerCommands adds all subcommands to the root command
func registerCommands() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(explainCmd)
	rootCmd.AddCommand(whyCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(daemonCmd)

	// Add daemon sub-subcommands
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonLogsCmd)
}

// initCmd initializes a new DevContext project
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize DevContext in the current directory",
	Long:  `Initialize a new DevContext project, creating necessary configuration files and setting up LLM provider.`,
	Run: func(cmd *cobra.Command, args []string) {
		runInitWizard()
	},
}

func runInitWizard() {
	// Print welcome banner
	pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("DevCtx", pterm.NewStyle(pterm.FgCyan)),
	).Render()

	pterm.DefaultParagraph.Println("Welcome to DevContext AI! This wizard will help you set up your environment.")
	fmt.Println()

	// Check if config already exists
	if config.Exists() {
		pterm.Warning.Println("Configuration already exists at ~/.devctx/config.yml")
		fmt.Println()

		var overwrite bool
		fmt.Print("Overwrite existing configuration? (y/N): ")
		var answer string
		fmt.Scanln(&answer)
		overwrite = answer == "y" || answer == "Y"

		if !overwrite {
			pterm.Info.Println("Run 'devctx config' to view or modify existing settings")
			return
		}
	}

	// Create config directories
	if err := config.EnsureDirectories(); err != nil {
		pterm.Error.Printf("Failed to create directories: %v\n", err)
		os.Exit(1)
	}

	// Initialize config
	cfg := config.NewDefaultConfig()

	// Step 1: Configure LLM provider
	if err := wizard.ConfigureProvider(cfg); err != nil {
		pterm.Error.Printf("LLM configuration failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Step 2: Configure codebase
	if err := wizard.ConfigureCodebase(cfg); err != nil {
		pterm.Error.Printf("Codebase configuration failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()

	// Save configuration
	if err := cfg.Save(); err != nil {
		pterm.Error.Printf("Failed to save configuration: %v\n", err)
		os.Exit(1)
	}

	// Success message
	pterm.DefaultBox.WithTitle("Setup Complete").Println(
		"DevContext is now configured!\n\n" +
			"Next steps:\n" +
			"  1. Run 'devctx index' to index your codebase\n" +
			"  2. Run 'devctx ask \"your question\"' to ask questions\n" +
			"  3. Run 'devctx daemon start' to start the AI daemon\n\n" +
			"Configuration saved to: ~/.devctx/config.yml",
	)
}

// indexCmd indexes the codebase
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index the codebase for AI analysis",
	Long:  `Scan and index the codebase, generating embeddings for semantic search and analysis.`,
	Run: func(cmd *cobra.Command, args []string) {
		runIndex()
	},
}

// askCmd asks questions about the codebase
var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question about your codebase",
	Long:  `Ask natural language questions about your codebase and get AI-powered answers.`,
	Run: func(cmd *cobra.Command, args []string) {
		question := ""
		if len(args) > 0 {
			question = strings.Join(args, " ")
		}
		runAsk(question)
	},
}

// explainCmd explains code
var explainCmd = &cobra.Command{
	Use:   "explain [file or symbol]",
	Short: "Explain code or concepts",
	Long:  `Get AI-powered explanations of code files, functions, or concepts in your codebase.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runExplain(args[0])
	},
}

// whyCmd explains why code exists
var whyCmd = &cobra.Command{
	Use:   "why [file or symbol]",
	Short: "Explain why code exists or was written this way",
	Long:  `Understand the reasoning behind code decisions by analyzing history, comments, and context.`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) >= 2 { runWhy(args[0], args[1]) } else { runExplain(args[0]) }
	},
}

// reviewCmd reviews code changes
var reviewCmd = &cobra.Command{
	Use:   "review [file or diff]",
	Short: "Review code changes",
	Long:  `Get AI-powered code review suggestions for files or diffs.`,
	Run: func(cmd *cobra.Command, args []string) {
		runReview()
	},
}

// onboardCmd helps onboard to a codebase
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Get onboarding guidance for the codebase",
	Long:  `Generate an interactive onboarding guide to help understand the codebase structure and patterns.`,
	Run: func(cmd *cobra.Command, args []string) {
		runOnboard()
	},
}

// configCmd manages configuration
var configCmd = &cobra.Command{
	Use:   "config [set key value]",
	Short: "Manage DevContext configuration",
	Long:  `View and modify DevContext configuration settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			showConfig()
			return
		}
		if len(args) >= 3 && args[0] == "set" {
			setConfig(args[1], args[2])
			return
		}
		cmd.Help()
	},
}

func showConfig() {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Printf("Failed to load config: %v\n", err)
		pterm.Info.Println("Run 'devctx init' to create a configuration")
		return
	}

	pterm.DefaultHeader.Println("DevContext Configuration")
	fmt.Println()

	// LLM Settings
	pterm.DefaultSection.Println("LLM Provider")
	tableData := pterm.TableData{
		{"Setting", "Value"},
		{"Provider", cfg.DevCtx.LLM.Provider},
		{"Model", cfg.DevCtx.LLM.Model},
		{"Base URL", maskIfEmpty(cfg.DevCtx.LLM.BaseURL)},
		{"API Key", maskSecret(cfg.DevCtx.LLM.APIKey)},
		{"Embedding Model", cfg.DevCtx.LLM.EmbeddingModel},
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	fmt.Println()

	// Codebase Settings
	pterm.DefaultSection.Println("Codebase")
	tableData2 := pterm.TableData{
		{"Setting", "Value"},
		{"Type", cfg.DevCtx.Codebase.Type},
		{"Path", cfg.DevCtx.Codebase.Path},
		{"Remote", maskIfEmpty(cfg.DevCtx.Codebase.Remote)},
		{"Token", maskSecret(cfg.DevCtx.Codebase.Token)},
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableData2).Render()
	fmt.Println()

	// Index Settings
	pterm.DefaultSection.Println("Index")
	lastRun := "Never"
	if !cfg.DevCtx.Index.LastRun.IsZero() {
		lastRun = cfg.DevCtx.Index.LastRun.Format("2006-01-02 15:04:05")
	}
	tableData3 := pterm.TableData{
		{"Setting", "Value"},
		{"Store", cfg.DevCtx.Index.Store},
		{"Last Run", lastRun},
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableData3).Render()

	fmt.Println()
	pterm.Info.Println("Use 'devctx config set <key> <value>' to change settings")
	pterm.Info.Println("Example: devctx config set llm.provider anthropic")
}

func setConfig(key, value string) {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Printf("Failed to load config: %v\n", err)
		return
	}

	switch key {
	case "llm.provider":
		// Re-run provider wizard for the new provider
		cfg.DevCtx.LLM.Provider = value
		if err := wizard.ConfigureProvider(cfg); err != nil {
			pterm.Error.Printf("Configuration failed: %v\n", err)
			return
		}
	case "llm.model":
		cfg.DevCtx.LLM.Model = value
	case "llm.api-key":
		cfg.DevCtx.LLM.APIKey = value
	case "llm.base-url":
		cfg.DevCtx.LLM.BaseURL = value
	case "codebase.path":
		cfg.DevCtx.Codebase.Path = value
	case "codebase.type":
		cfg.DevCtx.Codebase.Type = value
	default:
		pterm.Error.Printf("Unknown config key: %s\n", key)
		pterm.Info.Println("Available keys: llm.provider, llm.model, llm.api-key, llm.base-url, codebase.path, codebase.type")
		return
	}

	if err := cfg.Save(); err != nil {
		pterm.Error.Printf("Failed to save config: %v\n", err)
		return
	}

	pterm.Success.Printf("Updated %s = %s\n", key, maskSecret(value))
}

func maskSecret(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func maskIfEmpty(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

// daemonCmd manages the background daemon
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the DevContext daemon",
	Long:  `Start, stop, and manage the DevContext background daemon that powers AI features.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// daemonStartCmd starts the daemon
var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	Long:  `Start the DevContext background daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("daemon start: TODO - Start daemon")
	},
}

// daemonStopCmd stops the daemon
var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	Long:  `Stop the running DevContext daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("daemon stop: TODO - Stop daemon")
	},
}

// daemonStatusCmd shows daemon status
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	Long:  `Display the current status of the DevContext daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("daemon status: TODO - Show daemon status")
	},
}

// daemonLogsCmd shows daemon logs
var daemonLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show daemon logs",
	Long:  `Display logs from the DevContext daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("daemon logs: TODO - Show daemon logs")
	},
}
