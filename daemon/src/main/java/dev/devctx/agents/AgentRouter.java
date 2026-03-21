package dev.devctx.agents;

import dev.devctx.socket.Request;
import dev.devctx.socket.Response;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

import java.util.function.Consumer;

/**
 * Routes requests to the appropriate agent based on the agent field.
 */
@Component
public class AgentRouter {
    private static final Logger log = LoggerFactory.getLogger(AgentRouter.class);

    private final LogicExplainerAgent logicExplainer;
    private final DependencyTracerAgent dependencyTracer;
    private final PRReviewerAgent prReviewer;
    private final OnboardingAgent onboarding;

    public AgentRouter(
            LogicExplainerAgent logicExplainer,
            DependencyTracerAgent dependencyTracer,
            PRReviewerAgent prReviewer,
            OnboardingAgent onboarding) {
        this.logicExplainer = logicExplainer;
        this.dependencyTracer = dependencyTracer;
        this.prReviewer = prReviewer;
        this.onboarding = onboarding;
    }

    /**
     * Routes a request to the appropriate agent.
     *
     * @param request   The incoming request
     * @param responder Consumer to send responses back to the client
     */
    public void route(Request request, Consumer<Response> responder) {
        try {
            switch (request.agent()) {
                case "ask", "explain" -> logicExplainer.handle(request, responder);
                case "why" -> dependencyTracer.handle(request, responder);
                case "review" -> prReviewer.handle(request, responder);
                case "onboard" -> onboarding.handle(request, responder);
                default -> {
                    log.warn("Unknown agent: {}", request.agent());
                    responder.accept(Response.error("Unknown agent: " + request.agent()));
                }
            }
        } catch (Exception e) {
            log.error("Error routing request to agent {}: {}", request.agent(), e.getMessage());
            responder.accept(Response.error("Internal error: " + e.getMessage()));
        }
    }
}
