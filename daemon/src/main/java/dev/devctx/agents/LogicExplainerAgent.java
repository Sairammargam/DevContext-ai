package dev.devctx.agents;

import dev.devctx.rag.RagService;
import dev.devctx.socket.Request;
import dev.devctx.socket.Response;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.client.ChatClient;
import org.springframework.ai.chat.messages.AssistantMessage;
import org.springframework.ai.chat.messages.Message;
import org.springframework.ai.chat.messages.SystemMessage;
import org.springframework.ai.chat.messages.UserMessage;
import org.springframework.ai.chat.prompt.Prompt;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Flux;

import java.util.ArrayList;
import java.util.List;
import java.util.function.Consumer;

/**
 * Agent that explains code logic to developers.
 * Retrieves function bodies, callees, and provides step-by-step explanations.
 * Supports continuous conversation with history.
 */
@Service
public class LogicExplainerAgent {
    private static final Logger log = LoggerFactory.getLogger(LogicExplainerAgent.class);

    private static final String SYSTEM_PROMPT = """
        You are a code explanation expert. Your job is to explain code clearly and concisely
        to developers of all skill levels.

        When explaining code:
        1. Start with a high-level overview of what the code does
        2. Break down the logic step by step
        3. Explain WHY the code is written this way, not just what it does
        4. Highlight edge cases and error handling
        5. Use simple analogies when helpful
        6. Reference the actual code in your explanation

        Be concise but thorough. Format your response with markdown for readability.
        """;

    private final ChatClient chatClient;
    private final RagService ragService;

    public LogicExplainerAgent(ChatClient.Builder chatClientBuilder, RagService ragService) {
        this.chatClient = chatClientBuilder.build();
        this.ragService = ragService;
    }

    public void handle(Request request, Consumer<Response> responder) {
        try {
            // Retrieve relevant context from RAG
            String context = ragService.retrieveContext(request.prompt());

            // Build messages list with history
            List<Message> messages = buildMessages(request, context);

            // Create prompt with all messages
            Prompt prompt = new Prompt(messages);

            // Stream response from LLM
            Flux<String> responseStream = chatClient.prompt(prompt)
                .stream()
                .content();

            responseStream
                .doOnNext(token -> responder.accept(Response.token(token)))
                .doOnComplete(() -> responder.accept(Response.done()))
                .doOnError(e -> responder.accept(Response.error(e.getMessage())))
                .blockLast();

        } catch (Exception e) {
            log.error("Error in LogicExplainerAgent: {}", e.getMessage());
            responder.accept(Response.error("Failed to explain: " + e.getMessage()));
        }
    }

    /**
     * Builds the message list including system prompt, history, and current question.
     */
    private List<Message> buildMessages(Request request, String context) {
        List<Message> messages = new ArrayList<>();

        // Add system message
        messages.add(new SystemMessage(SYSTEM_PROMPT));

        // Add conversation history if present
        if (request.history() != null && !request.history().isEmpty()) {
            for (Request.Message historyMsg : request.history()) {
                if ("user".equals(historyMsg.role())) {
                    messages.add(new UserMessage(historyMsg.content()));
                } else if ("assistant".equals(historyMsg.role())) {
                    messages.add(new AssistantMessage(historyMsg.content()));
                }
            }
        }

        // Add current user message with context
        String userPrompt = buildPrompt(request.prompt(), context);
        messages.add(new UserMessage(userPrompt));

        log.debug("Built {} messages (including {} history messages)",
            messages.size(),
            request.history() != null ? request.history().size() : 0);

        return messages;
    }

    private String buildPrompt(String question, String context) {
        if (context == null || context.isEmpty()) {
            return question;
        }

        return """
            Here is relevant code context from the codebase:

            ```
            %s
            ```

            Question: %s
            """.formatted(context, question);
    }
}
