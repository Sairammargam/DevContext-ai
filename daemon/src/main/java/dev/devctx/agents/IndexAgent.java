package dev.devctx.agents;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import dev.devctx.rag.VectorStoreService;
import dev.devctx.socket.Request;
import dev.devctx.socket.Response;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.document.Document;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.function.Consumer;

/**
 * Agent that handles indexing operations.
 * Receives chunks from CLI and stores them in the vector store.
 */
@Service
public class IndexAgent {
    private static final Logger log = LoggerFactory.getLogger(IndexAgent.class);

    private final VectorStoreService vectorStoreService;
    private final ObjectMapper objectMapper;

    @Autowired
    public IndexAgent(
            @Autowired(required = false) VectorStoreService vectorStoreService,
            ObjectMapper objectMapper) {
        this.vectorStoreService = vectorStoreService;
        this.objectMapper = objectMapper;
    }

    public void handle(Request request, Consumer<Response> responder) {
        try {
            if (vectorStoreService == null || !vectorStoreService.isAvailable()) {
                responder.accept(Response.error(
                    "Vector store not available. Enable PgVector with DEVCTX_PGVECTOR_ENABLED=true"));
                return;
            }

            // Get chunks from context
            Object chunksObj = request.context().get("chunks");
            if (chunksObj == null) {
                responder.accept(Response.error("No chunks provided in request"));
                return;
            }

            // Parse chunks
            List<Map<String, Object>> chunks = objectMapper.convertValue(
                chunksObj,
                new TypeReference<List<Map<String, Object>>>() {}
            );

            responder.accept(Response.token("Indexing " + chunks.size() + " chunks...\n"));

            // Convert to Documents
            List<Document> documents = new ArrayList<>();
            for (Map<String, Object> chunk : chunks) {
                String id = (String) chunk.get("id");
                String content = (String) chunk.get("content");
                String file = (String) chunk.get("file");
                String language = (String) chunk.getOrDefault("language", "unknown");
                int startLine = ((Number) chunk.getOrDefault("startLine", 0)).intValue();
                int endLine = ((Number) chunk.getOrDefault("endLine", 0)).intValue();

                Document doc = VectorStoreService.createDocument(
                    id, content, file, language, startLine, endLine);
                documents.add(doc);
            }

            // Store in vector store
            vectorStoreService.storeDocuments(documents);

            responder.accept(Response.token("Successfully indexed " + documents.size() + " chunks.\n"));
            responder.accept(Response.done());

        } catch (Exception e) {
            log.error("Error in IndexAgent: {}", e.getMessage());
            responder.accept(Response.error("Failed to index: " + e.getMessage()));
        }
    }

    /**
     * Check if indexing is available.
     */
    public boolean isAvailable() {
        return vectorStoreService != null && vectorStoreService.isAvailable();
    }
}
