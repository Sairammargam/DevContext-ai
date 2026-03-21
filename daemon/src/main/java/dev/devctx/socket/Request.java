package dev.devctx.socket;

import java.util.List;
import java.util.Map;

/**
 * Request from CLI to daemon.
 */
public record Request(
    String agent,
    String prompt,
    Map<String, Object> context,
    String session,
    List<Message> history
) {
    public record Message(String role, String content) {}
}
