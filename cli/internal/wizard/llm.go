package wizard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/devctx/cli/internal/config"
	"github.com/pterm/pterm"
)

// LLM provider options
var providers = []string{
	"openai",
	"anthropic",
	"gemini",
	"azure",
	"bedrock",
	"ollama",
	"vllm",
	"custom",
}

// Provider descriptions for help text
var providerDescriptions = map[string]string{
	"openai":    "OpenAI API (gpt-4o, gpt-4o-mini, o3-mini)",
	"anthropic": "Anthropic API (claude-sonnet-4-5, claude-opus-4-6)",
	"gemini":    "Google Gemini API (gemini-2.5-flash, gemini-2.5-pro)",
	"azure":     "Azure OpenAI Service",
	"bedrock":   "AWS Bedrock (Claude, Titan, Llama)",
	"ollama":    "Ollama local models (localhost:11434)",
	"vllm":      "vLLM on-premises server",
	"custom":    "Custom OpenAI-compatible endpoint",
}

// Default models per provider
var defaultModels = map[string]string{
	"openai":    "gpt-4o",
	"anthropic": "claude-sonnet-4-5",
	"gemini":    "gemini-2.5-flash",
	"azure":     "gpt-4o",
	"bedrock":   "anthropic.claude-3-sonnet",
	"ollama":    "llama3.2",
	"vllm":      "meta-llama/Llama-3-8b",
	"custom":    "gpt-4o",
}

// Default embedding models per provider
var defaultEmbeddingModels = map[string]string{
	"openai":    "text-embedding-3-small",
	"anthropic": "text-embedding-3-small", // Uses OpenAI for embeddings
	"gemini":    "text-embedding-004",
	"azure":     "text-embedding-ada-002",
	"bedrock":   "amazon.titan-embed-text-v1",
	"ollama":    "nomic-embed-text",
	"vllm":      "nomic-embed-text",
	"custom":    "text-embedding-3-small",
}

// Default base URLs per provider
var defaultBaseURLs = map[string]string{
	"openai":    "https://api.openai.com/v1",
	"anthropic": "https://api.anthropic.com",
	"gemini":    "https://generativelanguage.googleapis.com/v1beta",
	"ollama":    "http://localhost:11434",
	"vllm":      "http://localhost:8000/v1",
}

// ConfigureProvider runs the LLM provider configuration wizard
func ConfigureProvider(cfg *config.Config) error {
	pterm.DefaultHeader.WithFullWidth().Println("LLM Provider Configuration")
	fmt.Println()

	// Build provider options with descriptions
	var options []string
	for _, p := range providers {
		options = append(options, fmt.Sprintf("%s - %s", p, providerDescriptions[p]))
	}

	// Select provider
	var selectedIdx int
	prompt := &survey.Select{
		Message: "Select your LLM provider:",
		Options: options,
	}
	if err := survey.AskOne(prompt, &selectedIdx); err != nil {
		return err
	}
	provider := providers[selectedIdx]
	cfg.DevCtx.LLM.Provider = provider

	// Configure based on provider
	switch provider {
	case "openai":
		return configureOpenAI(cfg)
	case "anthropic":
		return configureAnthropic(cfg)
	case "gemini":
		return configureGemini(cfg)
	case "azure":
		return configureAzure(cfg)
	case "bedrock":
		return configureBedrock(cfg)
	case "ollama":
		return configureOllama(cfg)
	case "vllm":
		return configureVLLM(cfg)
	case "custom":
		return configureCustom(cfg)
	}

	return nil
}

func configureOpenAI(cfg *config.Config) error {
	// API Key
	var apiKey string
	if err := survey.AskOne(&survey.Password{
		Message: "Enter your OpenAI API key:",
	}, &apiKey, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.APIKey = apiKey
	cfg.DevCtx.LLM.BaseURL = defaultBaseURLs["openai"]

	// Model selection
	models := []string{"gpt-4o", "gpt-4o-mini", "o3-mini", "gpt-4-turbo"}
	var model string
	if err := survey.AskOne(&survey.Select{
		Message: "Select chat model:",
		Options: models,
		Default: "gpt-4o",
	}, &model); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["openai"]

	// Validate
	return validateOpenAI(cfg)
}

func configureAnthropic(cfg *config.Config) error {
	var apiKey string
	if err := survey.AskOne(&survey.Password{
		Message: "Enter your Anthropic API key:",
	}, &apiKey, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.APIKey = apiKey
	cfg.DevCtx.LLM.BaseURL = defaultBaseURLs["anthropic"]

	models := []string{"claude-sonnet-4-5", "claude-opus-4-6", "claude-3-5-haiku-latest"}
	var model string
	if err := survey.AskOne(&survey.Select{
		Message: "Select chat model:",
		Options: models,
		Default: "claude-sonnet-4-5",
	}, &model); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["anthropic"]

	return validateAnthropic(cfg)
}

func configureGemini(cfg *config.Config) error {
	var apiKey string
	if err := survey.AskOne(&survey.Password{
		Message: "Enter your Google AI API key:",
	}, &apiKey, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.APIKey = apiKey
	cfg.DevCtx.LLM.BaseURL = defaultBaseURLs["gemini"]

	models := []string{"gemini-2.5-flash", "gemini-2.5-pro", "gemini-2.0-flash", "gemini-1.5-pro", "gemini-1.5-flash"}
	var model string
	if err := survey.AskOne(&survey.Select{
		Message: "Select chat model:",
		Options: models,
		Default: "gemini-2.5-flash",
	}, &model); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["gemini"]

	return validateGemini(cfg)
}

func configureAzure(cfg *config.Config) error {
	var endpoint string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter your Azure OpenAI endpoint URL:",
		Help:    "e.g., https://your-resource.openai.azure.com",
	}, &endpoint, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.BaseURL = endpoint

	var apiKey string
	if err := survey.AskOne(&survey.Password{
		Message: "Enter your Azure API key:",
	}, &apiKey, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.APIKey = apiKey

	var deployment string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter your deployment name:",
	}, &deployment, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.AzureDeployment = deployment
	cfg.DevCtx.LLM.Model = deployment

	var apiVersion string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter API version:",
		Default: "2024-02-01",
	}, &apiVersion); err != nil {
		return err
	}
	cfg.DevCtx.LLM.AzureAPIVersion = apiVersion
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["azure"]

	return validateAzure(cfg)
}

func configureBedrock(cfg *config.Config) error {
	var region string
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-northeast-1"}
	if err := survey.AskOne(&survey.Select{
		Message: "Select AWS region:",
		Options: regions,
		Default: "us-east-1",
	}, &region); err != nil {
		return err
	}
	cfg.DevCtx.LLM.AWSRegion = region

	models := []string{
		"anthropic.claude-3-sonnet",
		"anthropic.claude-3-haiku",
		"amazon.titan-text-express-v1",
		"meta.llama3-70b-instruct-v1",
	}
	var model string
	if err := survey.AskOne(&survey.Select{
		Message: "Select model:",
		Options: models,
		Default: "anthropic.claude-3-sonnet",
	}, &model); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["bedrock"]

	pterm.Info.Println("Bedrock uses AWS credentials from environment or ~/.aws/credentials")
	return nil
}

func configureOllama(cfg *config.Config) error {
	var baseURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter Ollama server URL:",
		Default: "http://localhost:11434",
	}, &baseURL); err != nil {
		return err
	}
	cfg.DevCtx.LLM.BaseURL = baseURL

	var model string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter model name (must be pulled):",
		Default: "llama3.2",
		Help:    "Run 'ollama pull <model>' first",
	}, &model); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["ollama"]

	return validateOllama(cfg)
}

func configureVLLM(cfg *config.Config) error {
	var baseURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter vLLM server URL:",
		Default: "http://localhost:8000/v1",
	}, &baseURL); err != nil {
		return err
	}
	cfg.DevCtx.LLM.BaseURL = baseURL

	var model string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter model name served by vLLM:",
		Default: "meta-llama/Llama-3-8b",
	}, &model); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = defaultEmbeddingModels["vllm"]

	return validateVLLM(cfg)
}

func configureCustom(cfg *config.Config) error {
	var baseURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter OpenAI-compatible API URL:",
		Help:    "Must support /chat/completions endpoint",
	}, &baseURL, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.BaseURL = baseURL

	var apiKey string
	if err := survey.AskOne(&survey.Password{
		Message: "Enter API key (leave empty if not required):",
	}, &apiKey); err != nil {
		return err
	}
	cfg.DevCtx.LLM.APIKey = apiKey

	var model string
	if err := survey.AskOne(&survey.Input{
		Message: "Enter model name:",
	}, &model, survey.WithValidator(survey.Required)); err != nil {
		return err
	}
	cfg.DevCtx.LLM.Model = model
	cfg.DevCtx.LLM.EmbeddingModel = "text-embedding-3-small"

	return nil
}

// Validation functions
func validateOpenAI(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Validating OpenAI API key...")

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.DevCtx.LLM.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		spinner.Fail("Failed to connect to OpenAI")
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		spinner.Fail("Invalid API key")
		return fmt.Errorf("invalid OpenAI API key")
	}

	if resp.StatusCode != 200 {
		spinner.Fail("API error")
		return fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	spinner.Success("OpenAI API key validated")
	return nil
}

func validateAnthropic(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Validating Anthropic API key...")

	client := &http.Client{Timeout: 10 * time.Second}

	body := map[string]interface{}{
		"model":      cfg.DevCtx.LLM.Model,
		"max_tokens": 1,
		"messages":   []map[string]string{{"role": "user", "content": "hi"}},
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonBody))
	req.Header.Set("x-api-key", cfg.DevCtx.LLM.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		spinner.Fail("Failed to connect to Anthropic")
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		spinner.Fail("Invalid API key")
		return fmt.Errorf("invalid Anthropic API key")
	}

	spinner.Success("Anthropic API key validated")
	return nil
}

func validateGemini(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Validating Google AI API key...")

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/models?key=%s", cfg.DevCtx.LLM.BaseURL, cfg.DevCtx.LLM.APIKey)

	resp, err := client.Get(url)
	if err != nil {
		spinner.Fail("Failed to connect to Google AI")
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 400 || resp.StatusCode == 401 {
		spinner.Fail("Invalid API key")
		return fmt.Errorf("invalid Google AI API key")
	}

	spinner.Success("Google AI API key validated")
	return nil
}

func validateAzure(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Validating Azure OpenAI configuration...")

	// Just check if endpoint is reachable
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/openai/deployments?api-version=%s",
		strings.TrimSuffix(cfg.DevCtx.LLM.BaseURL, "/"),
		cfg.DevCtx.LLM.AzureAPIVersion)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("api-key", cfg.DevCtx.LLM.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		spinner.Fail("Failed to connect to Azure")
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		spinner.Fail("Invalid credentials")
		return fmt.Errorf("invalid Azure credentials")
	}

	spinner.Success("Azure OpenAI configuration validated")
	return nil
}

func validateOllama(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Checking Ollama server...")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(cfg.DevCtx.LLM.BaseURL + "/api/tags")
	if err != nil {
		spinner.Fail("Ollama not running")
		pterm.Warning.Println("Make sure Ollama is running: ollama serve")
		return fmt.Errorf("cannot connect to Ollama at %s", cfg.DevCtx.LLM.BaseURL)
	}
	defer resp.Body.Close()

	spinner.Success("Ollama server connected")
	return nil
}

func validateVLLM(cfg *config.Config) error {
	spinner, _ := pterm.DefaultSpinner.Start("Checking vLLM server...")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(cfg.DevCtx.LLM.BaseURL + "/models")
	if err != nil {
		spinner.Fail("vLLM not reachable")
		return fmt.Errorf("cannot connect to vLLM at %s", cfg.DevCtx.LLM.BaseURL)
	}
	defer resp.Body.Close()

	spinner.Success("vLLM server connected")
	return nil
}
