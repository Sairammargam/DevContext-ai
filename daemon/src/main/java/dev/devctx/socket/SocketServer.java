package dev.devctx.socket;

import com.fasterxml.jackson.databind.ObjectMapper;
import dev.devctx.agents.AgentRouter;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.*;
import java.net.StandardProtocolFamily;
import java.net.UnixDomainSocketAddress;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Unix domain socket server for CLI communication.
 * Uses Java 21 virtual threads for scalable connection handling.
 */
@Component
public class SocketServer {
    private static final Logger log = LoggerFactory.getLogger(SocketServer.class);

    private final ObjectMapper objectMapper;
    private final AgentRouter agentRouter;

    @Value("${devctx.socket.path:${user.home}/.devctx/daemon.sock}")
    private String socketPath;

    private ServerSocketChannel serverChannel;
    private ExecutorService executor;
    private volatile boolean running = true;

    public SocketServer(ObjectMapper objectMapper, AgentRouter agentRouter) {
        this.objectMapper = objectMapper;
        this.agentRouter = agentRouter;
    }

    @PostConstruct
    public void start() {
        executor = Executors.newVirtualThreadPerTaskExecutor();
        executor.submit(this::runServer);
        log.info("Socket server starting on {}", socketPath);
    }

    @PreDestroy
    public void stop() {
        running = false;
        try {
            if (serverChannel != null) {
                serverChannel.close();
            }
            Path path = Path.of(socketPath);
            Files.deleteIfExists(path);
        } catch (IOException e) {
            log.warn("Error closing socket: {}", e.getMessage());
        }
        if (executor != null) {
            executor.shutdownNow();
        }
        log.info("Socket server stopped");
    }

    private void runServer() {
        try {
            Path path = Path.of(socketPath);

            // Ensure parent directory exists
            Files.createDirectories(path.getParent());

            // Remove existing socket file
            Files.deleteIfExists(path);

            // Create Unix domain socket
            UnixDomainSocketAddress address = UnixDomainSocketAddress.of(path);
            serverChannel = ServerSocketChannel.open(StandardProtocolFamily.UNIX);
            serverChannel.bind(address);

            log.info("Socket server listening on {}", socketPath);

            while (running) {
                try {
                    SocketChannel clientChannel = serverChannel.accept();
                    executor.submit(() -> handleConnection(clientChannel));
                } catch (IOException e) {
                    if (running) {
                        log.error("Error accepting connection: {}", e.getMessage());
                    }
                }
            }
        } catch (IOException e) {
            log.error("Failed to start socket server: {}", e.getMessage());
        }
    }

    private void handleConnection(SocketChannel channel) {
        try (channel;
             BufferedReader reader = new BufferedReader(
                 new InputStreamReader(java.nio.channels.Channels.newInputStream(channel)));
             PrintWriter writer = new PrintWriter(
                 new OutputStreamWriter(java.nio.channels.Channels.newOutputStream(channel)), true)) {

            // Read request
            String line = reader.readLine();
            if (line == null) return;

            Request request = objectMapper.readValue(line, Request.class);
            String promptPreview = request.prompt() != null
                ? request.prompt().substring(0, Math.min(50, request.prompt().length()))
                : "(empty)";
            log.debug("Received request: agent={}, prompt={}", request.agent(), promptPreview);

            // Route to appropriate agent and stream response
            agentRouter.route(request, response -> {
                try {
                    String json = objectMapper.writeValueAsString(response);
                    writer.println(json);
                } catch (Exception e) {
                    log.error("Error writing response: {}", e.getMessage());
                }
            });

        } catch (Exception e) {
            log.error("Error handling connection: {}", e.getMessage());
        }
    }
}
