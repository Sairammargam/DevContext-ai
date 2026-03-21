package wizard

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/devctx/cli/internal/config"
	"github.com/google/go-github/v60/github"
	"github.com/pterm/pterm"
)

// ConfigureGitHubToken configures and validates a GitHub Personal Access Token
func ConfigureGitHubToken(cfg *config.Config) error {
	pterm.Info.Println("A Personal Access Token (PAT) is required for private repositories")
	pterm.Info.Println("Create one at: https://github.com/settings/tokens")
	pterm.Info.Println("Required scope: repo:read (or full repo)")
	fmt.Println()

	var token string
	if err := survey.AskOne(&survey.Password{
		Message: "Enter your GitHub Personal Access Token:",
	}, &token, survey.WithValidator(survey.Required)); err != nil {
		return err
	}

	cfg.DevCtx.Codebase.Token = token

	// Validate token
	if err := validateGitHubToken(cfg); err != nil {
		return err
	}

	return nil
}

func validateGitHubToken(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Validating GitHub token...")

	// Create GitHub client with token
	client := github.NewClient(nil).WithAuthToken(cfg.DevCtx.Codebase.Token)
	ctx := context.Background()

	// Get authenticated user to verify token
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		spinner.Fail("Invalid GitHub token")
		return fmt.Errorf("failed to authenticate with GitHub: %w", err)
	}

	spinner.Success(fmt.Sprintf("Authenticated as: %s", user.GetLogin()))

	// If we have a repo URL, verify access to it
	if cfg.DevCtx.Codebase.Remote != "" {
		owner, repo := parseGitHubURL(cfg.DevCtx.Codebase.Remote)
		if owner != "" && repo != "" {
			spinner2, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Verifying access to %s/%s...", owner, repo))

			repository, _, err := client.Repositories.Get(ctx, owner, repo)
			if err != nil {
				spinner2.Fail("Cannot access repository")
				return fmt.Errorf("cannot access repository %s/%s: %w", owner, repo, err)
			}

			spinner2.Success(fmt.Sprintf("Access verified: %s (%s)", repository.GetFullName(), getVisibility(repository)))
		}
	}

	return nil
}

// ValidateRepoAccess checks if the configured token can access the repository
func ValidateRepoAccess(cfg *config.Config) error {
	if cfg.DevCtx.Codebase.Type != "github" {
		return nil
	}

	if cfg.DevCtx.Codebase.Token == "" {
		// Public repo, no token needed - just verify URL is valid
		return nil
	}

	return validateGitHubToken(cfg)
}

func parseGitHubURL(url string) (owner, repo string) {
	// Handle various GitHub URL formats:
	// https://github.com/owner/repo
	// https://github.com/owner/repo.git
	// git@github.com:owner/repo.git

	url = strings.TrimSuffix(url, ".git")

	if strings.HasPrefix(url, "git@github.com:") {
		// SSH format
		path := strings.TrimPrefix(url, "git@github.com:")
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1]
		}
	} else if strings.Contains(url, "github.com") {
		// HTTPS format
		parts := strings.Split(url, "/")
		// Find github.com in parts and get next two
		for i, part := range parts {
			if part == "github.com" && i+2 < len(parts) {
				return parts[i+1], parts[i+2]
			}
		}
	}

	return "", ""
}

func getVisibility(repo *github.Repository) string {
	if repo.GetPrivate() {
		return "private"
	}
	return "public"
}

// ListUserRepos lists repositories accessible to the authenticated user
func ListUserRepos(token string, limit int) ([]*github.Repository, error) {
	client := github.NewClient(nil).WithAuthToken(token)
	ctx := context.Background()

	opts := &github.RepositoryListOptions{
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: limit},
	}

	repos, _, err := client.Repositories.List(ctx, "", opts)
	if err != nil {
		return nil, err
	}

	return repos, nil
}
