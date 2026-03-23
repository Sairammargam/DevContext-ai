package dev.devctx.provider;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.embedding.EmbeddingModel;
import org.springframework.ai.openai.OpenAiEmbeddingModel;
import org.springframework.ai.openai.OpenAiEmbeddingOptions;
import org.springframework.ai.openai.api.OpenAiApi;
import org.springframework.ai.vectorstore.VectorStore;
import org.springframework.ai.vectorstore.pgvector.PgVectorStore;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Configuration for PgVector store.
 * Only enabled when devctx.pgvector.enabled=true
 */
@Configuration
@ConditionalOnProperty(name = "devctx.pgvector.enabled", havingValue = "true")
public class PgVectorConfig {
    private static final Logger log = LoggerFactory.getLogger(PgVectorConfig.class);

    @Value("${devctx.llm.provider:openai}")
    private String provider;

    @Value("${devctx.llm.api-key:}")
    private String apiKey;

    @Value("${devctx.llm.embedding-model:text-embedding-3-small}")
    private String embeddingModel;

    @Value("${devctx.llm.base-url:}")
    private String configuredBaseUrl;

    @Value("${devctx.embedding.dimensions:1536}")
    private int dimensions;

    @Bean
    public EmbeddingModel embeddingModel() {
        String baseUrl = resolveEmbeddingBaseUrl();
        String key = resolveApiKey();

        log.info("Creating EmbeddingModel: provider={}, model={}, baseUrl={}",
            provider, embeddingModel, baseUrl);

        OpenAiApi openAiApi = OpenAiApi.builder()
            .baseUrl(baseUrl)
            .apiKey(key)
            .build();

        return new OpenAiEmbeddingModel(openAiApi);
    }

    @Bean
    public VectorStore vectorStore(JdbcTemplate jdbcTemplate, EmbeddingModel embeddingModel) {
        log.info("Creating PgVectorStore with dimensions={}", dimensions);

        return PgVectorStore.builder(jdbcTemplate, embeddingModel)
            .dimensions(dimensions)
            .distanceType(PgVectorStore.PgDistanceType.COSINE_DISTANCE)
            .indexType(PgVectorStore.PgIndexType.HNSW)
            .initializeSchema(true)
            .build();
    }

    private String resolveEmbeddingBaseUrl() {
        if (configuredBaseUrl != null && !configuredBaseUrl.isBlank()) {
            return stripTrailingSlash(configuredBaseUrl);
        }
        return switch (provider.toLowerCase()) {
            case "openai" -> "https://api.openai.com";
            case "gemini" -> "https://generativelanguage.googleapis.com";
            case "ollama" -> "http://localhost:11434";
            default -> "https://api.openai.com";
        };
    }

    private String resolveApiKey() {
        if (apiKey != null && !apiKey.isBlank()) {
            return apiKey;
        }
        return switch (provider.toLowerCase()) {
            case "ollama" -> "ollama";
            default -> "";
        };
    }

    private static String stripTrailingSlash(String url) {
        return url.endsWith("/") ? url.substring(0, url.length() - 1) : url;
    }
}
