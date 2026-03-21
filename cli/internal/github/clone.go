package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/pterm/pterm"
)

// RepoManager handles GitHub repository operations
type RepoManager struct {
	token string
}

// NewRepoManager creates a new repository manager
func NewRepoManager(token string) *RepoManager {
	return &RepoManager{token: token}
}

// CloneResult contains the result of a clone operation
type CloneResult struct {
	LocalPath string
	Branch    string
	Commit    string
	ClonedAt  time.Time
}

// Clone clones a GitHub repository to the local filesystem
func (r *RepoManager) Clone(repoURL, destPath string) (*CloneResult, error) {
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Cloning %s...", repoURL))

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		spinner.Fail("Failed to create directory")
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Remove existing directory if it exists
	if _, err := os.Stat(destPath); err == nil {
		if err := os.RemoveAll(destPath); err != nil {
			spinner.Fail("Failed to remove existing directory")
			return nil, fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Set up authentication if token is provided
	var auth *http.BasicAuth
	if r.token != "" {
		auth = &http.BasicAuth{
			Username: "x-access-token", // GitHub PAT uses this
			Password: r.token,
		}
	}

	// Clone the repository
	cloneOpts := &git.CloneOptions{
		URL:      repoURL,
		Progress: nil, // Could add progress writer
		Auth:     auth,
		Depth:    0, // Full clone for history analysis
	}

	repo, err := git.PlainClone(destPath, false, cloneOpts)
	if err != nil {
		spinner.Fail("Clone failed")
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get HEAD reference
	ref, err := repo.Head()
	if err != nil {
		spinner.Fail("Failed to get HEAD")
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	spinner.Success(fmt.Sprintf("Cloned to %s", destPath))

	return &CloneResult{
		LocalPath: destPath,
		Branch:    ref.Name().Short(),
		Commit:    ref.Hash().String()[:8],
		ClonedAt:  time.Now(),
	}, nil
}

// Pull pulls the latest changes from remote
func (r *RepoManager) Pull(repoPath string) error {
	spinner, _ := pterm.DefaultSpinner.Start("Pulling latest changes...")

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		spinner.Fail("Failed to open repository")
		return fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		spinner.Fail("Failed to get worktree")
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Set up authentication
	var auth *http.BasicAuth
	if r.token != "" {
		auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: r.token,
		}
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Auth:       auth,
		Force:      false,
	})

	if err == git.NoErrAlreadyUpToDate {
		spinner.Success("Already up to date")
		return nil
	}

	if err != nil {
		spinner.Fail("Pull failed")
		return fmt.Errorf("failed to pull: %w", err)
	}

	spinner.Success("Updated to latest")
	return nil
}

// GetChangedFiles returns files changed since the given commit
func (r *RepoManager) GetChangedFiles(repoPath, sinceCommit string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get the commit object
	commitHash := plumbing.NewHash(sinceCommit)
	commit, err := repo.CommitObject(commitHash)
	if err != nil {
		// If commit not found, return all files
		return r.GetAllFiles(repoPath)
	}

	// Get HEAD commit
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Get the diff
	patch, err := commit.Patch(headCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	var changedFiles []string
	seen := make(map[string]bool)

	for _, filePatch := range patch.FilePatches() {
		from, to := filePatch.Files()
		if from != nil && !seen[from.Path()] {
			changedFiles = append(changedFiles, from.Path())
			seen[from.Path()] = true
		}
		if to != nil && !seen[to.Path()] {
			changedFiles = append(changedFiles, to.Path())
			seen[to.Path()] = true
		}
	}

	return changedFiles, nil
}

// GetAllFiles returns all tracked files in the repository
func (r *RepoManager) GetAllFiles(repoPath string) ([]string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var files []string
	tree.Files().ForEach(func(f *object.File) error {
		files = append(files, f.Name)
		return nil
	})

	return files, nil
}

// GetCurrentCommit returns the current HEAD commit hash
func (r *RepoManager) GetCurrentCommit(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	return head.Hash().String(), nil
}

// IsGitRepository checks if the path is a git repository
func IsGitRepository(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// ParseGitHubURL extracts owner and repo from a GitHub URL
func ParseGitHubURL(url string) (owner, repo string, err error) {
	url = strings.TrimSuffix(url, ".git")

	// Handle HTTPS URLs
	if strings.HasPrefix(url, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(url, "https://github.com/"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	// Handle SSH URLs
	if strings.HasPrefix(url, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(url, "git@github.com:"), "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("invalid GitHub URL: %s", url)
}
