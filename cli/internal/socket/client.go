package socket

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

// Request represents a request to the daemon
type Request struct {
	Agent   string                 `json:"agent"` // ask, explain, why, review, onboard
	Prompt  string                 `json:"prompt"`
	Context map[string]interface{} `json:"context,omitempty"`
	Session string                 `json:"session,omitempty"`
	History []Message              `json:"history,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"` // user, assistant
	Content string `json:"content"`
}

// Response represents a response from the daemon
type Response struct {
	Type    string `json:"type"` // token, done, error
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// Client handles communication with the daemon over Unix socket
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new socket client
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    30 * time.Second,
	}
}

// IsConnected checks if the daemon socket is available
func (c *Client) IsConnected() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// WaitForConnection waits for the daemon to become available
func (c *Client) WaitForConnection(maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if c.IsConnected() {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("daemon not available after %v", maxWait)
}

// Send sends a request and returns a channel for streaming responses
func (c *Client) Send(req Request) (<-chan Response, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	// Send request
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Create response channel
	respChan := make(chan Response, 100)

	// Read responses in goroutine
	go func() {
		defer conn.Close()
		defer close(respChan)

		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				respChan <- Response{Type: "error", Error: err.Error()}
				return
			}

			var resp Response
			if err := json.Unmarshal(line, &resp); err != nil {
				continue
			}

			respChan <- resp

			if resp.Type == "done" || resp.Type == "error" {
				return
			}
		}
	}()

	return respChan, nil
}

// Ask sends an ask request to the daemon
func (c *Client) Ask(prompt string, history []Message, session string) (<-chan Response, error) {
	return c.Send(Request{
		Agent:   "ask",
		Prompt:  prompt,
		History: history,
		Session: session,
	})
}

// Explain sends an explain request
func (c *Client) Explain(target string, context map[string]interface{}) (<-chan Response, error) {
	return c.Send(Request{
		Agent:   "explain",
		Prompt:  target,
		Context: context,
	})
}

// Why sends a why request for dependency tracing
func (c *Client) Why(source, target string) (<-chan Response, error) {
	return c.Send(Request{
		Agent:  "why",
		Prompt: fmt.Sprintf("%s -> %s", source, target),
		Context: map[string]interface{}{
			"source": source,
			"target": target,
		},
	})
}

// Review sends a review request with git diff
func (c *Client) Review(diff string) (<-chan Response, error) {
	return c.Send(Request{
		Agent:  "review",
		Prompt: "Review these changes",
		Context: map[string]interface{}{
			"diff": diff,
		},
	})
}

// Onboard sends an onboard request
func (c *Client) Onboard() (<-chan Response, error) {
	return c.Send(Request{
		Agent:  "onboard",
		Prompt: "Generate onboarding guide",
	})
}

// Index sends chunks to the daemon for vector store indexing
func (c *Client) Index(chunks []map[string]interface{}) (<-chan Response, error) {
	return c.Send(Request{
		Agent:  "index",
		Prompt: "Index code chunks",
		Context: map[string]interface{}{
			"chunks": chunks,
		},
	})
}

// SocketExists checks if the socket file exists
func SocketExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
