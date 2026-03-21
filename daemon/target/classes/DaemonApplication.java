package dev.devctx;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration;

/**
 * DevContext Daemon Application
 *
 * AI-powered codebase intelligence daemon that provides:
 * - Semantic code search via embeddings
 * - AI-powered code explanations
 * - Code review suggestions
 * - Codebase Q&A
 */
@SpringBootApplication(exclude = {
    DataSourceAutoConfiguration.class
})
public class DaemonApplication {

    public static void main(String[] args) {
        SpringApplication.run(DaemonApplication.class, args);
        System.out.println("\n✓ DevContext daemon started successfully");
    }
}
