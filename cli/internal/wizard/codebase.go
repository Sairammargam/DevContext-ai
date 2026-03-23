package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/devctx/cli/internal/config"
	"github.com/pterm/pterm"
)

// ConfigureCodebase runs the codebase configuration wizard
func ConfigureCodebase(cfg *config.Config) error {
	pterm.DefaultHeader.WithFullWidth().Println("Codebase Configuration")
	fmt.Println()

	// Select codebase type
	var sourceType string
	if err := survey.AskOne(&survey.Select{
		Message: "Where is your codebase?",
		Options: []string{
			"local - Local directory on this machine",
			"github - GitHub repository (public or private)",
		},
	}, &sourceType); err != nil {
		return err
	}

	if strings.HasPrefix(sourceType, "local") {
		return configureLocalCodebase(cfg)
	}
	return configureGitHubCodebase(cfg)
}

func configureLocalCodebase(cfg *config.Config) error {
	cfg.DevCtx.Codebase.Type = "local"

	// Get current working directory as default
	cwd, _ := os.Getwd()

	var path string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter the path to your codebase:",
		Default: cwd,
		Help:    "Full path to the root directory of your project",
	}, &path, survey.WithValidator(func(ans interface{}) error {
		p := ans.(string)
		// Expand ~ to home directory
		if strings.HasPrefix(p, "~") {
			home, _ := os.UserHomeDir()
			p = filepath.Join(home, p[1:])
		}

		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("path does not exist: %s", p)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", p)
		}
		return nil
	})); err != nil {
		return err
	}

	// Expand ~ if present
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	cfg.DevCtx.Codebase.Path = absPath

	// Check if it looks like a code directory
	if err := validateCodeDirectory(absPath); err != nil {
		pterm.Warning.Println(err.Error())
		var proceed bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Continue anyway?",
			Default: true,
		}, &proceed); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !proceed {
			return fmt.Errorf("aborted by user")
		}
	}

	pterm.Success.Printf("Codebase configured: %s\n", absPath)
	return nil
}

func configureGitHubCodebase(cfg *config.Config) error {
	cfg.DevCtx.Codebase.Type = "github"

	// Get repository URL
	var repoURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter the GitHub repository URL:",
		Help:    "e.g., https://github.com/owner/repo",
	}, &repoURL, survey.WithValidator(func(ans interface{}) error {
		url := ans.(string)
		if !strings.Contains(url, "github.com") {
			return fmt.Errorf("must be a GitHub URL")
		}
		return nil
	})); err != nil {
		return err
	}

	cfg.DevCtx.Codebase.Remote = repoURL

	// Check if private
	var isPrivate bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Is this a private repository?",
		Default: false,
	}, &isPrivate); err != nil {
		return err
	}

	if isPrivate {
		if err := ConfigureGitHubToken(cfg); err != nil {
			return err
		}
	}

	// Set local clone path
	repoName := extractRepoName(repoURL)
	home, _ := os.UserHomeDir()
	cfg.DevCtx.Codebase.Path = filepath.Join(home, ".devctx", "repos", repoName)

	pterm.Success.Printf("GitHub repository configured: %s\n", repoURL)
	if isPrivate {
		pterm.Info.Println("Repository will be cloned on first index")
	}

	return nil
}

func validateCodeDirectory(path string) error {
	// Check for common code indicators
	codeIndicators := []string{
		"go.mod", "go.sum", // Go
		"package.json",            // Node.js
		"pom.xml", "build.gradle", // Java
		"requirements.txt", "setup.py", "pyproject.toml", // Python
		"Cargo.toml", // Rust
		".git",       // Git repo
		"src",        // Common source directory
	}

	for _, indicator := range codeIndicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			return nil // Found a code indicator
		}
	}

	// Check for any source files
	codeExtensions := []string{".go", ".java", ".py", ".js", ".ts", ".rs", ".c", ".cpp", ".rb"}
	foundCode := false

	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || foundCode {
			return filepath.SkipDir
		}
		for _, ext := range codeExtensions {
			if strings.HasSuffix(p, ext) {
				foundCode = true
				return filepath.SkipDir
			}
		}
		return nil
	})

	if !foundCode {
		return fmt.Errorf("no source code files detected in this directory")
	}

	return nil
}

func extractRepoName(url string) string {
	// Extract repo name from URL like https://github.com/owner/repo
	parts := strings.Split(strings.TrimSuffix(url, ".git"), "/")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return "repo"
}
