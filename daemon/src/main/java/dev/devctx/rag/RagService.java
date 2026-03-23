package dev.devctx.rag;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * RAG (Retrieval Augmented Generation) service.
 * Retrieves relevant context from indexed codebase.
 * Uses VectorStore when available, falls back to file-based search.
 */
@Service
public class RagService {
    private static final Logger log = LoggerFactory.getLogger(RagService.class);

    @Value("${devctx.index.store:${user.home}/.devctx/index}")
    private String indexPath;

    private final VectorStoreService vectorStoreService;

    @Autowired
    public RagService(@Autowired(required = false) VectorStoreService vectorStoreService) {
        this.vectorStoreService = vectorStoreService;
        if (vectorStoreService != null && vectorStoreService.isAvailable()) {
            log.info("RagService initialized with VectorStore (PgVector)");
        } else {
            log.info("RagService initialized with file-based fallback");
        }
    }

    /**
     * Retrieves relevant context for a query.
     */
    public String retrieveContext(String query) {
        // Try vector store first
        if (vectorStoreService != null && vectorStoreService.isAvailable()) {
            String context = vectorStoreService.searchAsContext(query);
            if (!context.isEmpty()) {
                return context;
            }
        }

        // Fall back to file-based search
        return retrieveFromFiles(query);
    }

    /**
     * File-based fallback when vector store is not available.
     */
    private String retrieveFromFiles(String query) {
        try {
            Path chunksFile = Path.of(indexPath, "chunks.json");
            if (!Files.exists(chunksFile)) {
                log.warn("No index found at {}", chunksFile);
                return "";
            }

            String content = Files.readString(chunksFile);

            // Return first 8000 chars as context (simple fallback)
            if (content.length() > 8000) {
                content = content.substring(0, 8000);
            }

            return content;
        } catch (IOException e) {
            log.error("Error reading index: {}", e.getMessage());
            return "";
        }
    }

    /**
     * Retrieves dependency context for source->target analysis.
     */
    public String retrieveDependencyContext(String source, String target) {
        return retrieveContext(source + " " + target);
    }

    /**
     * Retrieves context for files in a diff.
     */
    public String retrieveContextForDiff(String diff) {
        // Extract first part of diff for context retrieval
        String query = diff.length() > 500 ? diff.substring(0, 500) : diff;
        return retrieveContext(query);
    }

    /**
     * Retrieves codebase overview for onboarding.
     */
    public String retrieveCodebaseOverview() {
        // For onboarding, try to get high-level context
        if (vectorStoreService != null && vectorStoreService.isAvailable()) {
            String context = vectorStoreService.searchAsContext(
                "main entry point architecture overview configuration setup");
            if (!context.isEmpty()) {
                return context;
            }
        }

        // Fall back to file-based
        try {
            Path chunksFile = Path.of(indexPath, "chunks.json");
            if (!Files.exists(chunksFile)) {
                return "No codebase index available. Please run 'devctx index' first.";
            }

            String content = Files.readString(chunksFile);

            if (content.length() > 10000) {
                content = content.substring(0, 10000);
            }

            return content;
        } catch (IOException e) {
            log.error("Error reading index: {}", e.getMessage());
            return "";
        }
    }

    /**
     * Check if vector store is enabled and available.
     */
    public boolean isVectorStoreEnabled() {
        return vectorStoreService != null && vectorStoreService.isAvailable();
    }
}
