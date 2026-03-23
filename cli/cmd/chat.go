package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devctx/cli/internal/config"
	"github.com/devctx/cli/internal/render"
	"github.com/devctx/cli/internal/socket"
	"github.com/pterm/pterm"
)

// runAsk handles the ask command
func runAsk(question string) {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Println("No configuration found. Run 'devctx init' first.")
		return
	}

	client := socket.NewClient(cfg.DevCtx.Daemon.Socket)

	// Check if daemon is running
	if !client.IsConnected() {
		pterm.Warning.Println("Daemon not running. Starting daemon...")
		if err := startDaemonProcess(cfg); err != nil {
			pterm.Error.Printf("Failed to start daemon: %v\n", err)
			return
		}

		// Wait for daemon to be ready
		if err := client.WaitForConnection(15 * cfg.DevCtx.Daemon.Timeout()); err != nil {
			pterm.Error.Println("Daemon failed to start. Check logs with 'devctx daemon logs'")
			return
		}
	}

	// Interactive mode if no question provided
	if question == "" {
		runInteractiveAsk(client)
		return
	}

	// Single question mode
	askQuestion(client, question, nil)
}

func runInteractiveAsk(client *socket.Client) {
	var history []socket.Message
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	pterm.Info.Println("DevContext AI - Interactive Mode")
	pterm.Info.Println("Type 'exit' or press Ctrl+C to quit")
	fmt.Println()

	for {
		render.PrintUserPrompt()
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			pterm.Info.Println("Goodbye!")
			break
		}

		// Get response and capture it for history
		response := askQuestionWithHistory(client, input, history)

		// Add both user and assistant messages to history
		history = append(history, socket.Message{Role: "user", Content: input})
		if response != "" {
			history = append(history, socket.Message{Role: "assistant", Content: response})
		}
		fmt.Println()
	}
}

// askQuestionWithHistory sends a question and returns the full response for history tracking
func askQuestionWithHistory(client *socket.Client, question string, history []socket.Message) string {
	respChan, err := client.Ask(question, history, "")
	if err != nil {
		pterm.Error.Printf("Failed to send question: %v\n", err)
		return ""
	}

	printer := render.NewPrinter("DevCtx")
	printer.PrintAgentLabel()

	var responseBuilder strings.Builder

	for resp := range respChan {
		switch resp.Type {
		case "token":
			printer.PrintToken(resp.Content)
			responseBuilder.WriteString(resp.Content)
		case "error":
			fmt.Println()
			pterm.Error.Println(resp.Error)
			return ""
		case "done":
			fmt.Println()
		}
	}

	return responseBuilder.String()
}

func askQuestion(client *socket.Client, question string, history []socket.Message) {
	respChan, err := client.Ask(question, history, "")
	if err != nil {
		pterm.Error.Printf("Failed to send question: %v\n", err)
		return
	}

	printer := render.NewPrinter("DevCtx")
	printer.PrintAgentLabel()

	for resp := range respChan {
		switch resp.Type {
		case "token":
			printer.PrintToken(resp.Content)
		case "error":
			fmt.Println()
			pterm.Error.Println(resp.Error)
		case "done":
			fmt.Println()
		}
	}
}

// runExplain handles the explain command
func runExplain(target string) {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Println("No configuration found. Run 'devctx init' first.")
		return
	}

	client := socket.NewClient(cfg.DevCtx.Daemon.Socket)
	if err := ensureDaemonRunning(client, cfg); err != nil {
		pterm.Error.Printf("Daemon error: %v\n", err)
		return
	}

	// Read file content if target is a file path
	var context map[string]interface{}
	if _, err := os.Stat(target); err == nil {
		content, err := os.ReadFile(target)
		if err == nil {
			context = map[string]interface{}{
				"file":    target,
				"content": string(content),
			}
		}
	}

	respChan, err := client.Explain(target, context)
	if err != nil {
		pterm.Error.Printf("Failed to explain: %v\n", err)
		return
	}

	printer := render.NewPrinter("Explain")
	printer.PrintAgentLabel()

	for resp := range respChan {
		switch resp.Type {
		case "token":
			printer.PrintToken(resp.Content)
		case "error":
			fmt.Println()
			pterm.Error.Println(resp.Error)
		case "done":
			fmt.Println()
		}
	}
}

// runWhy handles the why command
func runWhy(source, target string) {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Println("No configuration found. Run 'devctx init' first.")
		return
	}

	client := socket.NewClient(cfg.DevCtx.Daemon.Socket)
	if err := ensureDaemonRunning(client, cfg); err != nil {
		pterm.Error.Printf("Daemon error: %v\n", err)
		return
	}

	respChan, err := client.Why(source, target)
	if err != nil {
		pterm.Error.Printf("Failed to trace dependency: %v\n", err)
		return
	}

	printer := render.NewPrinter("Dependency")
	printer.PrintAgentLabel()

	for resp := range respChan {
		switch resp.Type {
		case "token":
			printer.PrintToken(resp.Content)
		case "error":
			fmt.Println()
			pterm.Error.Println(resp.Error)
		case "done":
			fmt.Println()
		}
	}
}

// runReview handles the review command
func runReview() {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Println("No configuration found. Run 'devctx init' first.")
		return
	}

	// Get git diff
	diff, err := getGitDiff()
	if err != nil {
		pterm.Error.Printf("Failed to get git diff: %v\n", err)
		return
	}

	if diff == "" {
		pterm.Info.Println("No changes to review")
		return
	}

	client := socket.NewClient(cfg.DevCtx.Daemon.Socket)
	if err := ensureDaemonRunning(client, cfg); err != nil {
		pterm.Error.Printf("Daemon error: %v\n", err)
		return
	}

	respChan, err := client.Review(diff)
	if err != nil {
		pterm.Error.Printf("Failed to review: %v\n", err)
		return
	}

	printer := render.NewPrinter("Review")
	printer.PrintAgentLabel()

	for resp := range respChan {
		switch resp.Type {
		case "token":
			printer.PrintToken(resp.Content)
		case "error":
			fmt.Println()
			pterm.Error.Println(resp.Error)
		case "done":
			fmt.Println()
		}
	}
}

// runOnboard handles the onboard command
func runOnboard() {
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Println("No configuration found. Run 'devctx init' first.")
		return
	}

	client := socket.NewClient(cfg.DevCtx.Daemon.Socket)
	if err := ensureDaemonRunning(client, cfg); err != nil {
		pterm.Error.Printf("Daemon error: %v\n", err)
		return
	}

	pterm.Info.Println("Generating onboarding guide...")
	fmt.Println()

	respChan, err := client.Onboard()
	if err != nil {
		pterm.Error.Printf("Failed to generate guide: %v\n", err)
		return
	}

	printer := render.NewPrinter("Onboard")
	printer.PrintAgentLabel()

	for resp := range respChan {
		switch resp.Type {
		case "token":
			printer.PrintToken(resp.Content)
		case "error":
			fmt.Println()
			pterm.Error.Println(resp.Error)
		case "done":
			fmt.Println()
		}
	}
}

func getGitDiff() (string, error) {
	// Try staged changes first
	cmd := exec.Command("git", "diff", "--staged")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return string(output), nil
	}

	// Fall back to unstaged changes
	cmd = exec.Command("git", "diff", "HEAD")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

func ensureDaemonRunning(client *socket.Client, cfg *config.Config) error {
	if !client.IsConnected() {
		pterm.Warning.Println("Daemon not running. Starting...")
		if err := startDaemonProcess(cfg); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
		if err := client.WaitForConnection(15 * cfg.DevCtx.Daemon.Timeout()); err != nil {
			return fmt.Errorf("daemon failed to start: %w", err)
		}
	}
	return nil
}

func startDaemonProcess(cfg *config.Config) error {
	jarPath := cfg.DevCtx.Daemon.JAR

	// Check if JAR exists
	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		// Try to find daemon in development mode
		home, _ := os.UserHomeDir()
		devPath := filepath.Join(home, "Desktop", "project", "devctx", "daemon")
		if _, err := os.Stat(filepath.Join(devPath, "pom.xml")); err == nil {
			// Run with Maven in development
			cmd := exec.Command("mvn", "spring-boot:run")
			cmd.Dir = devPath
			cmd.Stdout = nil
			cmd.Stderr = nil
			return cmd.Start()
		}
		return fmt.Errorf("daemon JAR not found at %s", jarPath)
	}

	cmd := exec.Command("java", "-jar", jarPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}
