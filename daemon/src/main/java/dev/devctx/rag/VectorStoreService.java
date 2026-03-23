package dev.devctx.rag;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.document.Document;
import org.springframework.ai.vectorstore.SearchRequest;
import org.springframework.ai.vectorstore.VectorStore;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

/**
 * Service for vector store operations using PgVector.
 * Handles storing and searching code embeddings for semantic search.
 */
@Service
@ConditionalOnProperty(name = "devctx.pgvector.enabled", havingValue = "true")
public class VectorStoreService {
    private static final Logger log = LoggerFactory.getLogger(VectorStoreService.class);

    private final VectorStore vectorStore;

    @Value("${devctx.rag.top-k:5}")
    private int defaultTopK;

    @Value("${devctx.rag.similarity-threshold:0.7}")
    private double similarityThreshold;

    @Autowired(required = false)
    public VectorStoreService(VectorStore vectorStore) {
        this.vectorStore = vectorStore;
        if (vectorStore != null) {
            log.info("VectorStoreService initialized with PgVector");
        } else {
            log.warn("VectorStore not available - falling back to file-based search");
        }
    }

    /**
     * Check if vector store is available.
     */
    public boolean isAvailable() {
        return vectorStore != null;
    }

    /**
     * Store documents (code chunks) in the vector store.
     */
    public void storeDocuments(List<Document> documents) {
        if (vectorStore == null) {
            log.warn("VectorStore not available, cannot store documents");
            return;
        }

        try {
            vectorStore.add(documents);
            log.info("Stored {} documents in vector store", documents.size());
        } catch (Exception e) {
            log.error("Failed to store documents: {}", e.getMessage());
            throw new RuntimeException("Failed to store documents", e);
        }
    }

    /**
     * Search for similar documents based on a query.
     */
    public List<Document> search(String query) {
        return search(query, defaultTopK);
    }

    /**
     * Search for similar documents with custom top-k.
     */
    public List<Document> search(String query, int topK) {
        if (vectorStore == null) {
            log.warn("VectorStore not available, returning empty results");
            return List.of();
        }

        try {
            SearchRequest request = SearchRequest.builder()
                .query(query)
                .topK(topK)
                .similarityThreshold(similarityThreshold)
                .build();

            List<Document> results = vectorStore.similaritySearch(request);
            log.debug("Found {} similar documents for query: {}",
                results.size(), query.substring(0, Math.min(50, query.length())));

            return results;
        } catch (Exception e) {
            log.error("Search failed: {}", e.getMessage());
            return List.of();
        }
    }

    /**
     * Search and return formatted context string.
     */
    public String searchAsContext(String query) {
        List<Document> docs = search(query);

        if (docs.isEmpty()) {
            return "";
        }

        return docs.stream()
            .map(doc -> {
                String file = doc.getMetadata().getOrDefault("file", "unknown").toString();
                String content = doc.getText();
                return "// File: " + file + "\n" + content;
            })
            .collect(Collectors.joining("\n\n---\n\n"));
    }

    /**
     * Delete all documents from the vector store.
     */
    public void clear() {
        if (vectorStore == null) {
            return;
        }

        try {
            // Note: PgVectorStore doesn't have a clear method,
            // would need to use JDBC to truncate the table
            log.info("Clear operation requested - manual table truncation may be needed");
        } catch (Exception e) {
            log.error("Failed to clear vector store: {}", e.getMessage());
        }
    }

    /**
     * Create a Document from a code chunk.
     */
    public static Document createDocument(String id, String content, String file,
            String language, int startLine, int endLine) {
        Map<String, Object> metadata = Map.of(
            "file", file,
            "language", language,
            "startLine", startLine,
            "endLine", endLine
        );
        return new Document(id, content, metadata);
    }
}
