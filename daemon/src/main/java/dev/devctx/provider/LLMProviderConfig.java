package dev.devctx.provider;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.client.ChatClient;
import org.springframework.ai.openai.OpenAiChatModel;
import org.springframework.ai.openai.OpenAiChatOptions;
import org.springframework.ai.openai.api.OpenAiApi;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Creates a ChatClient bean wired to the user's chosen LLM provider.
 *
 * Supports OpenAI-compatible APIs including:
 * - OpenAI, Anthropic (via proxy), Gemini, Ollama, vLLM
 */
@Configuration
public class LLMProviderConfig {

    private static final Logger log = LoggerFactory.getLogger(LLMProviderConfig.class);

    @Value("${devctx.llm.provider:openai}")
    private String provider;

    @Value("${devctx.llm.api-key:}")
    private String apiKey;

    @Value("${devctx.llm.model:gpt-4o}")
    private String model;

    @Value("${devctx.llm.base-url:}")
    private String configuredBaseUrl;

    @Value("${spring.ai.openai.chat.options.temperature:0.7}")
    private Float temperature;

    @Bean
    public ChatClient chatClient(ChatClient.Builder builder) {
        return builder.build();
    }

    @Bean
    public ChatClient.Builder chatClientBuilder() {
        String resolvedUrl = resolveBaseUrl();
        String resolvedKey = resolveApiKey();
        String completionsPath = resolveCompletionsPath();

        log.info("LLM provider={} model={} baseUrl={} completionsPath={}",
                provider, model, resolvedUrl, completionsPath);

        // Create OpenAI-compatible API client with custom completions path
        OpenAiApi openAiApi = OpenAiApi.builder()
                .baseUrl(resolvedUrl)
                .apiKey(resolvedKey)
                .completionsPath(completionsPath)
                .build();

        // Configure chat options
        OpenAiChatOptions options = OpenAiChatOptions.builder()
                .model(model)
                .temperature(temperature.doubleValue())
                .build();

        // Create chat model
        OpenAiChatModel chatModel = OpenAiChatModel.builder()
                .openAiApi(openAiApi)
                .defaultOptions(options)
                .build();

        return ChatClient.builder(chatModel);
    }

    /**
     * Resolves the base URL for the LLM provider.
     */
    private String resolveBaseUrl() {
        if (configuredBaseUrl != null && !configuredBaseUrl.isBlank()) {
            return stripTrailingSlash(configuredBaseUrl);
        }
        return switch (provider.toLowerCase()) {
            case "openai"    -> "https://api.openai.com";
            case "anthropic" -> "https://api.anthropic.com";
            case "gemini"    -> "https://generativelanguage.googleapis.com";
            case "ollama"    -> "http://localhost:11434";
            case "vllm"      -> "http://localhost:8000";
            case "azure"     -> throw new IllegalStateException(
                    "Azure requires devctx.llm.base-url to be set explicitly.");
            default          -> "https://api.openai.com";
        };
    }

    /**
     * Resolves the completions path based on provider.
     * Gemini uses /v1beta/openai/chat/completions instead of /v1/chat/completions
     */
    private String resolveCompletionsPath() {
        return switch (provider.toLowerCase()) {
            case "gemini" -> "/v1beta/openai/chat/completions";
            default       -> "/v1/chat/completions";
        };
    }

    private String resolveApiKey() {
        if (apiKey != null && !apiKey.isBlank()) {
            return apiKey;
        }
        return switch (provider.toLowerCase()) {
            case "ollama", "vllm" -> "devctx-local";
            default -> "";
        };
    }

    private static String stripTrailingSlash(String url) {
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }
}
