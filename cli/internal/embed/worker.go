package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devctx/cli/internal/config"
	"github.com/devctx/cli/internal/parser"
	"github.com/pterm/pterm"
)

// EmbeddingWorker handles embedding generation
type EmbeddingWorker struct {
	config *config.Config
	client *http.Client
}

// EmbeddingResult holds the result of an embedding operation
type EmbeddingResult struct {
	ChunkID   string    `json:"chunk_id"`
	Embedding []float32 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

// NewEmbeddingWorker creates a new embedding worker
func NewEmbeddingWorker(cfg *config.Config) *EmbeddingWorker {
	return &EmbeddingWorker{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// EmbedChunks generates embeddings for a batch of chunks
func (w *EmbeddingWorker) EmbedChunks(chunks []parser.Chunk) ([]EmbeddingResult, error) {
	var results []EmbeddingResult

	// Process in batches of 100
	batchSize := 100
	totalBatches := (len(chunks) + batchSize - 1) / batchSize

	progressBar, _ := pterm.DefaultProgressbar.WithTotal(totalBatches).WithTitle("Generating embeddings").Start()

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		batchResults, err := w.embedBatch(batch)
		if err != nil {
			pterm.Warning.Printf("Batch %d failed: %v\n", i/batchSize+1, err)
			// Add error results for failed batch
			for _, chunk := range batch {
				results = append(results, EmbeddingResult{
					ChunkID: chunk.ID,
					Error:   err.Error(),
				})
			}
		} else {
			results = append(results, batchResults...)
		}

		progressBar.Increment()
	}

	return results, nil
}

func (w *EmbeddingWorker) embedBatch(chunks []parser.Chunk) ([]EmbeddingResult, error) {
	switch w.config.DevCtx.LLM.Provider {
	case "openai", "azure":
		return w.embedOpenAI(chunks)
	case "anthropic":
		// Anthropic doesn't have embeddings, use OpenAI
		return w.embedOpenAI(chunks)
	case "gemini":
		return w.embedGemini(chunks)
	case "ollama":
		return w.embedOllama(chunks)
	case "custom", "vllm":
		return w.embedOpenAI(chunks) // Assume OpenAI-compatible
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", w.config.DevCtx.LLM.Provider)
	}
}

// OpenAI embeddings
func (w *EmbeddingWorker) embedOpenAI(chunks []parser.Chunk) ([]EmbeddingResult, error) {
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	model := w.config.DevCtx.LLM.EmbeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}

	body := map[string]interface{}{
		"input": texts,
		"model": model,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	baseURL := w.config.DevCtx.LLM.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	req, err := http.NewRequest("POST", baseURL+"/embeddings", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.config.DevCtx.LLM.APIKey)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var results []EmbeddingResult
	for i, chunk := range chunks {
		if i < len(result.Data) {
			results = append(results, EmbeddingResult{
				ChunkID:   chunk.ID,
				Embedding: result.Data[i].Embedding,
			})
		}
	}

	return results, nil
}

// Gemini embeddings
func (w *EmbeddingWorker) embedGemini(chunks []parser.Chunk) ([]EmbeddingResult, error) {
	var results []EmbeddingResult

	model := w.config.DevCtx.LLM.EmbeddingModel
	if model == "" {
		model = "text-embedding-004"
	}

	for _, chunk := range chunks {
		url := fmt.Sprintf("%s/models/%s:embedContent?key=%s",
			w.config.DevCtx.LLM.BaseURL, model, w.config.DevCtx.LLM.APIKey)

		body := map[string]interface{}{
			"content": map[string]interface{}{
				"parts": []map[string]string{
					{"text": chunk.Content},
				},
			},
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := w.client.Do(req)
		if err != nil {
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}

		var result struct {
			Embedding struct {
				Values []float32 `json:"values"`
			} `json:"embedding"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}
		resp.Body.Close()

		results = append(results, EmbeddingResult{
			ChunkID:   chunk.ID,
			Embedding: result.Embedding.Values,
		})
	}

	return results, nil
}

// Ollama embeddings
func (w *EmbeddingWorker) embedOllama(chunks []parser.Chunk) ([]EmbeddingResult, error) {
	var results []EmbeddingResult

	model := w.config.DevCtx.LLM.EmbeddingModel
	if model == "" {
		model = "nomic-embed-text"
	}

	baseURL := w.config.DevCtx.LLM.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	for _, chunk := range chunks {
		body := map[string]interface{}{
			"model":  model,
			"prompt": chunk.Content,
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}

		req, err := http.NewRequest("POST", baseURL+"/api/embeddings", bytes.NewBuffer(jsonBody))
		if err != nil {
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := w.client.Do(req)
		if err != nil {
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}

		var result struct {
			Embedding []float32 `json:"embedding"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			results = append(results, EmbeddingResult{ChunkID: chunk.ID, Error: err.Error()})
			continue
		}
		resp.Body.Close()

		results = append(results, EmbeddingResult{
			ChunkID:   chunk.ID,
			Embedding: result.Embedding,
		})
	}

	return results, nil
}

// GetEmbeddingDimension returns the expected embedding dimension for the configured model
func (w *EmbeddingWorker) GetEmbeddingDimension() int {
	switch w.config.DevCtx.LLM.Provider {
	case "openai", "azure":
		model := w.config.DevCtx.LLM.EmbeddingModel
		switch model {
		case "text-embedding-3-large":
			return 3072
		case "text-embedding-3-small":
			return 1536
		case "text-embedding-ada-002":
			return 1536
		default:
			return 1536
		}
	case "gemini":
		return 768
	case "ollama":
		return 768 // nomic-embed-text default
	default:
		return 1536
	}
}
