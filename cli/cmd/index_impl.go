package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/devctx/cli/internal/config"
	"github.com/devctx/cli/internal/github"
	"github.com/devctx/cli/internal/parser"
	"github.com/pterm/pterm"
)

// runIndex executes the indexing process
func runIndex() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		pterm.Error.Printf("Failed to load config: %v\n", err)
		pterm.Info.Println("Run 'devctx init' first to configure DevContext")
		os.Exit(1)
	}

	pterm.DefaultHeader.Println("Indexing Codebase")
	fmt.Println()

	// Create progress bar
	progressBar, _ := pterm.DefaultProgressbar.WithTotal(4).WithTitle("Indexing").Start()

	// Step 1: Clone or sync repository
	progressBar.UpdateTitle("Step 1: Syncing repository")
	repoPath, err := syncRepository(cfg)
	if err != nil {
		pterm.Error.Printf("Failed to sync repository: %v\n", err)
		os.Exit(1)
	}
	progressBar.Increment()

	// Step 2: Parse source files
	progressBar.UpdateTitle("Step 2: Parsing source files")
	files, err := parseSourceFiles(repoPath)
	if err != nil {
		pterm.Error.Printf("Failed to parse files: %v\n", err)
		os.Exit(1)
	}
	progressBar.Increment()

	// Step 3: Create chunks for embedding
	progressBar.UpdateTitle("Step 3: Creating chunks")
	chunks := createChunks(files, getRepoID(cfg))
	progressBar.Increment()

	// Step 4: Build dependency graph
	progressBar.UpdateTitle("Step 4: Building dependency graph")
	stats, err := buildGraph(cfg, files)
	if err != nil {
		pterm.Warning.Printf("Graph building had errors: %v\n", err)
	}
	progressBar.Increment()

	// Save chunks to disk (for later embedding)
	if err := saveChunks(cfg, chunks); err != nil {
		pterm.Warning.Printf("Failed to save chunks: %v\n", err)
	}

	// Update last run timestamp
	cfg.DevCtx.Index.LastRun = time.Now()
	if err := cfg.Save(); err != nil {
		pterm.Warning.Printf("Failed to save config: %v\n", err)
	}

	fmt.Println()
	pterm.Success.Println("Indexing complete!")
	fmt.Println()

	// Print summary
	printIndexSummary(files, chunks, stats)
}

func syncRepository(cfg *config.Config) (string, error) {
	if cfg.DevCtx.Codebase.Type == "local" {
		// Verify local path exists
		if _, err := os.Stat(cfg.DevCtx.Codebase.Path); err != nil {
			return "", fmt.Errorf("codebase path does not exist: %s", cfg.DevCtx.Codebase.Path)
		}
		pterm.Success.Printf("Using local repository: %s\n", cfg.DevCtx.Codebase.Path)
		return cfg.DevCtx.Codebase.Path, nil
	}

	// GitHub repository
	repoManager := github.NewRepoManager(cfg.DevCtx.Codebase.Token)

	// Check if already cloned
	if github.IsGitRepository(cfg.DevCtx.Codebase.Path) {
		// Pull latest changes
		if err := repoManager.Pull(cfg.DevCtx.Codebase.Path); err != nil {
			pterm.Warning.Printf("Pull failed, continuing with existing: %v\n", err)
		}
		return cfg.DevCtx.Codebase.Path, nil
	}

	// Clone the repository
	result, err := repoManager.Clone(cfg.DevCtx.Codebase.Remote, cfg.DevCtx.Codebase.Path)
	if err != nil {
		return "", err
	}

	pterm.Info.Printf("Cloned: %s (branch: %s, commit: %s)\n",
		result.LocalPath, result.Branch, result.Commit)

	return result.LocalPath, nil
}

func parseSourceFiles(repoPath string) ([]*parser.ParsedFile, error) {
	var allFiles []*parser.ParsedFile

	// Parse Go files
	goParser := parser.NewGoParser()
	goFiles, err := goParser.ParseDirectory(repoPath)
	if err != nil {
		pterm.Warning.Printf("Error parsing Go files: %v\n", err)
	} else {
		allFiles = append(allFiles, goFiles...)
	}

	// Parse other languages
	multiParser := parser.NewMultiLangParser()
	otherFiles, err := multiParser.ParseDirectory(repoPath)
	if err != nil {
		pterm.Warning.Printf("Error parsing other files: %v\n", err)
	} else {
		// Filter out Go files (already parsed with better AST)
		for _, f := range otherFiles {
			if f.Language != "go" {
				allFiles = append(allFiles, f)
			}
		}
	}

	return allFiles, nil
}

func createChunks(files []*parser.ParsedFile, repoID string) []parser.Chunk {
	chunker := parser.NewChunker(parser.DefaultChunkerConfig())
	return chunker.ChunkFiles(files, repoID)
}

func buildGraph(cfg *config.Config, files []*parser.ParsedFile) (map[string]int, error) {
	graphPath := filepath.Join(cfg.DevCtx.Index.Store, "graph.duckdb")

	graph, err := parser.NewDependencyGraph(graphPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph: %w", err)
	}
	defer graph.Close()

	// Clear existing data
	if err := graph.Clear(); err != nil {
		return nil, fmt.Errorf("failed to clear graph: %w", err)
	}

	// Build graph from parsed files
	if err := graph.BuildFromParsedFiles(files); err != nil {
		return nil, fmt.Errorf("failed to build graph: %w", err)
	}

	// Get statistics
	return graph.GetStatistics()
}

func saveChunks(cfg *config.Config, chunks []parser.Chunk) error {
	chunksPath := filepath.Join(cfg.DevCtx.Index.Store, "chunks.json")

	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(chunksPath, data, 0644)
}

func getRepoID(cfg *config.Config) string {
	if cfg.DevCtx.Codebase.Remote != "" {
		return cfg.DevCtx.Codebase.Remote
	}
	return cfg.DevCtx.Codebase.Path
}

func printIndexSummary(files []*parser.ParsedFile, chunks []parser.Chunk, graphStats map[string]int) {
	// Count by language
	langCount := make(map[string]int)
	symbolCount := 0
	for _, f := range files {
		langCount[f.Language]++
		symbolCount += len(f.Symbols)
	}

	pterm.DefaultSection.Println("Index Summary")

	// Files table
	tableData := pterm.TableData{{"Metric", "Count"}}
	tableData = append(tableData, []string{"Total Files", fmt.Sprintf("%d", len(files))})
	tableData = append(tableData, []string{"Total Symbols", fmt.Sprintf("%d", symbolCount)})
	tableData = append(tableData, []string{"Total Chunks", fmt.Sprintf("%d", len(chunks))})

	for lang, count := range langCount {
		tableData = append(tableData, []string{fmt.Sprintf("  %s files", lang), fmt.Sprintf("%d", count)})
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	// Graph stats
	if len(graphStats) > 0 {
		fmt.Println()
		pterm.DefaultSection.Println("Dependency Graph")
		graphTable := pterm.TableData{{"Type", "Count"}}
		for k, v := range graphStats {
			graphTable = append(graphTable, []string{k, fmt.Sprintf("%d", v)})
		}
		pterm.DefaultTable.WithHasHeader().WithData(graphTable).Render()
	}

	fmt.Println()
	pterm.Info.Println("Run 'devctx ask \"your question\"' to query your codebase")
}
