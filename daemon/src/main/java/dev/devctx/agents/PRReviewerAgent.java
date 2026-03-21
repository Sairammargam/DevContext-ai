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
 * Agent that reviews code changes (git diffs).
 * Provides feedback on changes, affected components, and risk assessment.
 */
@Service
public class PRReviewerAgent {
    private static final Logger log = LoggerFactory.getLogger(PRReviewerAgent.class);

    private static final String SYSTEM_PROMPT = """
        You are an experienced code reviewer. Your job is to review code changes
        and provide constructive feedback.

        When reviewing code:
        1. Summarize what the changes do at a high level
        2. Identify any potential bugs or issues
        3. Check for proper error handling
        4. Look for security vulnerabilities
        5. Assess the impact on other parts of the codebase
        6. Suggest improvements where appropriate
        7. Provide a risk assessment (Low/Medium/High)

        Format your review with clear sections:
        - **Summary**: What the changes do
        - **Impact**: What components are affected
        - **Issues**: Any problems found (or "None found")
        - **Suggestions**: Improvements to consider
        - **Risk Level**: Low/Medium/High with explanation

        Be constructive and helpful. Focus on significant issues, not style nitpicks.
        """;

    private final ChatClient chatClient;
    private final RagService ragService;

    public PRReviewerAgent(ChatClient.Builder chatClientBuilder, RagService ragService) {
        this.chatClient = chatClientBuilder.build();
        this.ragService = ragService;
    }

    public void handle(Request request, Consumer<Response> responder) {
        try {
            // Get diff from context
            String diff = request.context() != null ?
                (String) request.context().get("diff") : null;

            if (diff == null || diff.isEmpty()) {
                responder.accept(Response.error("No diff provided for review"));
                return;
            }

            // Retrieve context for changed files
            String context = ragService.retrieveContextForDiff(diff);

            // Build prompt
            String userPrompt = buildPrompt(diff, context);

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
            log.error("Error in PRReviewerAgent: {}", e.getMessage());
            responder.accept(Response.error("Failed to review: " + e.getMessage()));
        }
    }

    private String buildPrompt(String diff, String context) {
        StringBuilder sb = new StringBuilder();

        sb.append("Please review the following code changes:\n\n");
        sb.append("```diff\n");
        sb.append(diff);
        sb.append("\n```\n\n");

        if (context != null && !context.isEmpty()) {
            sb.append("Here is additional context about the affected code:\n\n```\n");
            sb.append(context);
            sb.append("\n```\n");
        }

        return sb.toString();
    }
}
