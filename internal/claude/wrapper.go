package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"` // Message content
}

// Response represents the result of a claude invocation.
type Response struct {
	Content   string        // The assistant's response text
	ToolCalls []ToolCall    // Any tool calls made
	Error     string        // Error message if any
	Duration  time.Duration // Time taken for the request
}

// ToolCall represents a tool invocation by the assistant.
type ToolCall struct {
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Output string          `json:"output,omitempty"`
}

// StreamEvent represents a streaming event from claude CLI.
type StreamEvent struct {
	Type    string          `json:"type"`
	Content string          `json:"content,omitempty"`
	Tool    *ToolCall       `json:"tool,omitempty"`
	Error   string          `json:"error,omitempty"`
	Raw     json.RawMessage `json:"-"`
}

// Wrapper provides a high-level interface to the Claude Code CLI.
type Wrapper struct {
	binaryPath   string
	defaultModel string
	timeout      time.Duration
}

// WrapperConfig holds configuration for the Wrapper.
type WrapperConfig struct {
	BinaryPath   string        // Path to claude binary (empty to auto-detect)
	DefaultModel string        // Default model to use
	Timeout      time.Duration // Default timeout for requests
}

// NewWrapper creates a new Wrapper with the given configuration.
func NewWrapper(cfg WrapperConfig) (*Wrapper, error) {
	binaryPath, err := FindBinary(cfg.BinaryPath)
	if err != nil {
		return nil, err
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	return &Wrapper{
		binaryPath:   binaryPath,
		defaultModel: cfg.DefaultModel,
		timeout:      timeout,
	}, nil
}

// BinaryPath returns the path to the claude binary.
func (w *Wrapper) BinaryPath() string {
	return w.binaryPath
}

// SendMessage sends a message to claude and returns the response.
// This is a synchronous call that blocks until the response is complete.
func (w *Wrapper) SendMessage(ctx context.Context, workDir string, prompt string) (*Response, error) {
	return w.SendConversation(ctx, workDir, nil, prompt)
}

// SendConversation sends a prompt with conversation history to claude.
// The history is passed as context, and the prompt is the new user message.
func (w *Wrapper) SendConversation(ctx context.Context, workDir string, history []Message, prompt string) (*Response, error) {
	start := time.Now()

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	cfg := ProcessConfig{
		BinaryPath:   w.binaryPath,
		WorkDir:      workDir,
		Model:        w.defaultModel,
		OutputFormat: "text",
	}

	proc, err := SpawnProcess(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("spawn process: %w", err)
	}
	defer proc.Close()

	// Build input: conversation history + new prompt
	var input strings.Builder
	for _, msg := range history {
		if msg.Role == "user" {
			input.WriteString("Human: ")
			input.WriteString(msg.Content)
			input.WriteString("\n\n")
		} else if msg.Role == "assistant" {
			input.WriteString("Assistant: ")
			input.WriteString(msg.Content)
			input.WriteString("\n\n")
		}
	}
	input.WriteString(prompt)

	// Write input and close stdin
	if _, err := io.WriteString(proc.Stdin(), input.String()); err != nil {
		proc.Kill()
		return nil, fmt.Errorf("write input: %w", err)
	}
	proc.Stdin().Close()

	// Read stdout and stderr concurrently
	var stdout, stderr bytes.Buffer
	errCh := make(chan error, 2)

	go func() {
		_, err := io.Copy(&stdout, proc.Stdout())
		errCh <- err
	}()

	go func() {
		_, err := io.Copy(&stderr, proc.Stderr())
		errCh <- err
	}()

	// Wait for reads to complete
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			proc.Kill()
			return nil, fmt.Errorf("read output: %w", err)
		}
	}

	// Wait for process to exit
	procErr := proc.Wait()

	response := &Response{
		Content:  strings.TrimSpace(stdout.String()),
		Duration: time.Since(start),
	}

	// Check for errors
	if procErr != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = procErr.Error()
		}
		response.Error = errMsg
	} else if stderr.Len() > 0 {
		// Some stderr output even on success (warnings, etc)
		// Don't treat as error, but could log
	}

	return response, nil
}

// StreamCallback is called for each event during streaming.
type StreamCallback func(event StreamEvent)

// SendMessageStreaming sends a message and streams the response.
func (w *Wrapper) SendMessageStreaming(ctx context.Context, workDir string, prompt string, callback StreamCallback) (*Response, error) {
	return w.SendConversationStreaming(ctx, workDir, nil, prompt, callback)
}

// SendConversationStreaming sends a conversation and streams the response.
func (w *Wrapper) SendConversationStreaming(ctx context.Context, workDir string, history []Message, prompt string, callback StreamCallback) (*Response, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	cfg := ProcessConfig{
		BinaryPath:   w.binaryPath,
		WorkDir:      workDir,
		Model:        w.defaultModel,
		OutputFormat: "stream-json",
	}

	proc, err := SpawnProcess(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("spawn process: %w", err)
	}
	defer proc.Close()

	// Build and send input
	var input strings.Builder
	for _, msg := range history {
		if msg.Role == "user" {
			input.WriteString("Human: ")
			input.WriteString(msg.Content)
			input.WriteString("\n\n")
		} else if msg.Role == "assistant" {
			input.WriteString("Assistant: ")
			input.WriteString(msg.Content)
			input.WriteString("\n\n")
		}
	}
	input.WriteString(prompt)

	if _, err := io.WriteString(proc.Stdin(), input.String()); err != nil {
		proc.Kill()
		return nil, fmt.Errorf("write input: %w", err)
	}
	proc.Stdin().Close()

	// Read stderr in background
	var stderr bytes.Buffer
	go func() {
		io.Copy(&stderr, proc.Stderr())
	}()

	// Parse streaming JSON output
	var fullContent strings.Builder
	var toolCalls []ToolCall

	scanner := bufio.NewScanner(proc.Stdout())
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Not JSON, treat as raw text
			event = StreamEvent{
				Type:    "text",
				Content: string(line),
			}
		}
		event.Raw = line

		// Accumulate content
		if event.Content != "" {
			fullContent.WriteString(event.Content)
		}
		if event.Tool != nil {
			toolCalls = append(toolCalls, *event.Tool)
		}

		if callback != nil {
			callback(event)
		}
	}

	if err := scanner.Err(); err != nil {
		proc.Kill()
		return nil, fmt.Errorf("read stream: %w", err)
	}

	procErr := proc.Wait()

	response := &Response{
		Content:   strings.TrimSpace(fullContent.String()),
		ToolCalls: toolCalls,
		Duration:  time.Since(start),
	}

	if procErr != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = procErr.Error()
		}
		response.Error = errMsg
	}

	return response, nil
}
