package dev.devctx.socket;

/**
 * Response from daemon to CLI.
 */
public record Response(
    String type,   // "token", "done", "error"
    String content,
    String error
) {
    public static Response token(String content) {
        return new Response("token", content, null);
    }

    public static Response done() {
        return new Response("done", null, null);
    }

    public static Response error(String message) {
        return new Response("error", null, message);
    }
}
