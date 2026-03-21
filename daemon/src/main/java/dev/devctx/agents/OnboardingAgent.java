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
 * Agent that generates onboarding guides for new developers.
 * Creates a "Start Here" guide based on codebase analysis.
 */
@Service
public class OnboardingAgent {
    private static final Logger log = LoggerFactory.getLogger(OnboardingAgent.class);

    private static final String SYSTEM_PROMPT = """
        You are a developer onboarding expert. Your job is to create a helpful
        "Start Here" guide for developers new to a codebase.

        When creating an onboarding guide:
        1. Identify the main purpose/domain of the project
        2. List the key concepts a new developer needs to understand
        3. Identify the most important files/modules to read first
        4. Create a suggested reading order
        5. Define an internal glossary of project-specific terms
        6. Highlight common patterns used in the codebase
        7. List any prerequisites or setup steps

        Format your guide with clear sections:
        - **Project Overview**: What this project does
        - **Key Concepts**: The 5 most important things to understand
        - **Architecture Overview**: High-level structure
        - **Reading Order**: Where to start (numbered list)
        - **Glossary**: Project-specific terms
        - **Common Patterns**: Recurring patterns to recognize
        - **Getting Started**: How to set up and run locally

        Make the guide welcoming and encouraging. Remember that new developers
        may feel overwhelmed, so be clear and supportive.
        """;

    private final ChatClient chatClient;
    private final RagService ragService;

    public OnboardingAgent(ChatClient.Builder chatClientBuilder, RagService ragService) {
        this.chatClient = chatClientBuilder.build();
        this.ragService = ragService;
    }

    public void handle(Request request, Consumer<Response> responder) {
        try {
            // Retrieve codebase overview
            String context = ragService.retrieveCodebaseOverview();

            // Build prompt
            String userPrompt = buildPrompt(context);

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
            log.error("Error in OnboardingAgent: {}", e.getMessage());
            responder.accept(Response.error("Failed to generate onboarding guide: " + e.getMessage()));
        }
    }

    private String buildPrompt(String context) {
        StringBuilder sb = new StringBuilder();

        sb.append("Generate an onboarding guide for this codebase.\n\n");

        if (context != null && !context.isEmpty()) {
            sb.append("Here is an overview of the codebase structure and key components:\n\n");
            sb.append(context);
            sb.append("\n\n");
        }

        sb.append("Create a comprehensive but concise guide that helps new developers ");
        sb.append("get up to speed quickly.");

        return sb.toString();
    }
}
