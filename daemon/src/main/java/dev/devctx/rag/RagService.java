package dev.devctx.rag;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

/**
 * RAG (Retrieval Augmented Generation) service.
 * Retrieves relevant context from indexed codebase.
 */
@Service
public class RagService {
    private static final Logger log = LoggerFactory.getLogger(RagService.class);

    @Value("${devctx.index.store:${user.home}/.devctx/index}")
    private String indexPath;

    /**
     * Retrieves relevant context for a query.
     */
    public String retrieveContext(String query) {
        try {
            // Load chunks from index
            Path chunksFile = Path.of(indexPath, "chunks.json");
            if (!Files.exists(chunksFile)) {
                log.warn("No index found at {}", chunksFile);
                return "";
            }

            // For now, return a sample of chunks
            // TODO: Implement vector similarity search with pgvector
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
        // TODO: Query DuckDB graph for path between source and target
        // For now, return general context
        return retrieveContext(source + " " + target);
    }

    /**
     * Retrieves context for files in a diff.
     */
    public String retrieveContextForDiff(String diff) {
        // Extract file names from diff
        // TODO: Parse diff and retrieve context for each file
        return retrieveContext(diff.substring(0, Math.min(500, diff.length())));
    }

    /**
     * Retrieves codebase overview for onboarding.
     */
    public String retrieveCodebaseOverview() {
        try {
            Path chunksFile = Path.of(indexPath, "chunks.json");
            if (!Files.exists(chunksFile)) {
                return "No codebase index available. Please run 'devctx index' first.";
            }

            // TODO: Retrieve top-level overview (most important files, main entry points)
            String content = Files.readString(chunksFile);

            // Return summary
            if (content.length() > 10000) {
                content = content.substring(0, 10000);
            }

            return content;
        } catch (IOException e) {
            log.error("Error reading index: {}", e.getMessage());
            return "";
        }
    }
}
