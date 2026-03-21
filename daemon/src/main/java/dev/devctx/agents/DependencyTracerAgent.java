package dev.devctx.agents;

import dev.devctx.rag.RagService;
import dev.devctx.socket.Request;
import dev.devctx.socket.Response;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.client.ChatClient;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;

import java.util.function.Consumer;

/**
 * Agent that traces dependencies between services/components.
 * Generates Mermaid diagrams showing call chains.
 */
@Service
public class DependencyTracerAgent {
    private static final Logger log = LoggerFactory.getLogger(DependencyTracerAgent.class);

    private static final String SYSTEM_PROMPT = """
        You are a dependency analysis expert. Your job is to explain the relationships
        and call chains between different parts of a codebase.

        When analyzing dependencies:
        1. Identify the call chain from source to target
        2. Explain WHY each component calls the next
        3. Describe what data flows through the chain
        4. Generate a Mermaid diagram showing the relationships
        5. Highlight any interesting patterns or potential issues

        Always include a Mermaid diagram in your response using this format:
        ```mermaid
        graph LR
            A[ServiceA] --> B[ServiceB]
            B --> C[ServiceC]
        ```

        Be thorough but concise. Use business terms alongside technical ones.
        """;

    private final ChatClient chatClient;
    private final RagService ragService;

    public DependencyTracerAgent(ChatClient.Builder chatClientBuilder, RagService ragService) {
        this.chatClient = chatClientBuilder.build();
        this.ragService = ragService;
    }

    public void handle(Request request, Consumer<Response> responder) {
        try {
            // Extract source and target from context
            String source = request.context() != null ?
                (String) request.context().get("source") : null;
            String target = request.context() != null ?
                (String) request.context().get("target") : null;

            // Retrieve dependency context
            String context = ragService.retrieveDependencyContext(source, target);

            // Build prompt
            String userPrompt = buildPrompt(source, target, context);

            // Stream response
            Flux<String> responseStream = chatClient.prompt()
                .system(SYSTEM_PROMPT)
                .user(userPrompt)
                .stream()
                .content();

            responseStream
                .doOnNext(token -> responder.accept(Response.token(token)))
                .doOnComplete(() -> responder.accept(Response.done()))
                .doOnError(e -> responder.accept(Response.error(e.getMessage())))
                .blockLast();

        } catch (Exception e) {
            log.error("Error in DependencyTracerAgent: {}", e.getMessage());
            responder.accept(Response.error("Failed to trace dependencies: " + e.getMessage()));
        }
    }

    private String buildPrompt(String source, String target, String context) {
        StringBuilder sb = new StringBuilder();

        if (context != null && !context.isEmpty()) {
            sb.append("Here is the relevant code and dependency information:\n\n```\n");
            sb.append(context);
            sb.append("\n```\n\n");
        }

        if (source != null && target != null) {
            sb.append("Explain why ").append(source).append(" depends on or calls ").append(target);
            sb.append(". Show the call chain and data flow between them.");
        } else {
            sb.append("Analyze the dependencies in this code and explain the relationships.");
        }

        return sb.toString();
    }
}
