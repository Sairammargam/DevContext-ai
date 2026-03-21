package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Chunk represents a code chunk for embedding
type Chunk struct {
	ID         string            `json:"id"`
	RepoID     string            `json:"repo_id"`
	FilePath   string            `json:"file_path"`
	Language   string            `json:"language"`
	Content    string            `json:"content"`
	StartLine  int               `json:"start_line"`
	EndLine    int               `json:"end_line"`
	TokenCount int               `json:"token_count"`
	Metadata   map[string]string `json:"metadata"`
}

// ChunkerConfig holds configuration for the chunker
type ChunkerConfig struct {
	MaxTokens    int // Maximum tokens per chunk (default: 300)
	OverlapLines int // Number of lines to overlap between chunks (default: 3)
	MinTokens    int // Minimum tokens for a chunk (default: 50)
}

// DefaultChunkerConfig returns default chunker configuration
func DefaultChunkerConfig() ChunkerConfig {
	return ChunkerConfig{
		MaxTokens:    300,
		OverlapLines: 3,
		MinTokens:    50,
	}
}

// Chunker splits source files into chunks for embedding
type Chunker struct {
	config ChunkerConfig
}

// NewChunker creates a new chunker with the given config
func NewChunker(config ChunkerConfig) *Chunker {
	return &Chunker{config: config}
}

// ChunkFile splits a parsed file into chunks
func (c *Chunker) ChunkFile(file *ParsedFile, repoID string) []Chunk {
	var chunks []Chunk

	// First, try to chunk by symbols (functions, types)
	symbolChunks := c.chunkBySymbols(file, repoID)
	if len(symbolChunks) > 0 {
		chunks = append(chunks, symbolChunks...)
	}

	// For remaining content or files without clear symbols, use line-based chunking
	if len(chunks) == 0 {
		chunks = c.chunkByLines(file, repoID)
	}

	return chunks
}

// chunkBySymbols creates chunks based on code symbols (functions, types, etc.)
func (c *Chunker) chunkBySymbols(file *ParsedFile, repoID string) []Chunk {
	var chunks []Chunk
	lines := strings.Split(file.Content, "\n")

	for _, sym := range file.Symbols {
		// Skip symbols that are too small
		if sym.EndLine-sym.Line < 2 {
			continue
		}

		// Get the content for this symbol
		startIdx := sym.Line - 1
		endIdx := sym.EndLine
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		content := strings.Join(lines[startIdx:endIdx], "\n")
		tokenCount := estimateTokens(content)

		// If symbol is too large, split it further
		if tokenCount > c.config.MaxTokens {
			subChunks := c.splitLargeContent(content, file.Path, file.Language, repoID, sym.Line, sym)
			chunks = append(chunks, subChunks...)
		} else if tokenCount >= c.config.MinTokens {
			chunk := Chunk{
				ID:         generateChunkID(file.Path, sym.Line),
				RepoID:     repoID,
				FilePath:   file.Path,
				Language:   file.Language,
				Content:    content,
				StartLine:  sym.Line,
				EndLine:    sym.EndLine,
				TokenCount: tokenCount,
				Metadata: map[string]string{
					"symbol_name": sym.Name,
					"symbol_kind": sym.Kind,
					"package":     file.Package,
				},
			}
			if sym.Signature != "" {
				chunk.Metadata["signature"] = sym.Signature
			}
			if sym.Doc != "" {
				chunk.Metadata["doc"] = truncate(sym.Doc, 500)
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// chunkByLines creates chunks based on line counts with overlap
func (c *Chunker) chunkByLines(file *ParsedFile, repoID string) []Chunk {
	var chunks []Chunk
	lines := strings.Split(file.Content, "\n")

	// Estimate lines per chunk (assume ~4 chars per token, ~40 chars per line)
	linesPerChunk := (c.config.MaxTokens * 4) / 40
	if linesPerChunk < 10 {
		linesPerChunk = 10
	}

	for i := 0; i < len(lines); i += linesPerChunk - c.config.OverlapLines {
		endIdx := i + linesPerChunk
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		content := strings.Join(lines[i:endIdx], "\n")
		tokenCount := estimateTokens(content)

		// Skip chunks that are too small
		if tokenCount < c.config.MinTokens {
			continue
		}

		chunk := Chunk{
			ID:         generateChunkID(file.Path, i+1),
			RepoID:     repoID,
			FilePath:   file.Path,
			Language:   file.Language,
			Content:    content,
			StartLine:  i + 1,
			EndLine:    endIdx,
			TokenCount: tokenCount,
			Metadata: map[string]string{
				"package": file.Package,
			},
		}
		chunks = append(chunks, chunk)

		// Stop if we've reached the end
		if endIdx >= len(lines) {
			break
		}
	}

	return chunks
}

// splitLargeContent splits a large symbol into multiple chunks
func (c *Chunker) splitLargeContent(content, path, language, repoID string, startLine int, sym Symbol) []Chunk {
	var chunks []Chunk
	lines := strings.Split(content, "\n")

	linesPerChunk := (c.config.MaxTokens * 4) / 40
	if linesPerChunk < 10 {
		linesPerChunk = 10
	}

	for i := 0; i < len(lines); i += linesPerChunk - c.config.OverlapLines {
		endIdx := i + linesPerChunk
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		chunkContent := strings.Join(lines[i:endIdx], "\n")
		tokenCount := estimateTokens(chunkContent)

		if tokenCount < c.config.MinTokens && len(chunks) > 0 {
			// Append to previous chunk if too small
			continue
		}

		chunk := Chunk{
			ID:         generateChunkID(path, startLine+i),
			RepoID:     repoID,
			FilePath:   path,
			Language:   language,
			Content:    chunkContent,
			StartLine:  startLine + i,
			EndLine:    startLine + endIdx - 1,
			TokenCount: tokenCount,
			Metadata: map[string]string{
				"symbol_name": sym.Name,
				"symbol_kind": sym.Kind,
				"part":        "partial",
			},
		}
		chunks = append(chunks, chunk)

		if endIdx >= len(lines) {
			break
		}
	}

	return chunks
}

// estimateTokens provides a rough estimate of token count
// Most tokenizers average ~4 characters per token for code
func estimateTokens(text string) int {
	// Simple estimation: count words and special characters
	words := 0
	inWord := false

	for _, c := range text {
		if c == ' ' || c == '\n' || c == '\t' {
			if inWord {
				words++
				inWord = false
			}
		} else {
			inWord = true
			// Count special characters as separate tokens
			if c == '(' || c == ')' || c == '{' || c == '}' ||
				c == '[' || c == ']' || c == ';' || c == ',' ||
				c == '.' || c == ':' || c == '=' {
				words++
			}
		}
	}
	if inWord {
		words++
	}

	// Adjust for code-specific patterns
	return int(float64(words) * 1.3)
}

// generateChunkID creates a unique ID for a chunk
func generateChunkID(path string, line int) string {
	data := []byte(path + ":" + string(rune(line)))
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ChunkFiles processes multiple files and returns all chunks
func (c *Chunker) ChunkFiles(files []*ParsedFile, repoID string) []Chunk {
	var allChunks []Chunk

	for _, file := range files {
		chunks := c.ChunkFile(file, repoID)
		allChunks = append(allChunks, chunks...)
	}

	return allChunks
}
