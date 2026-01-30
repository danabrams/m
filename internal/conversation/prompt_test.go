package conversation

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"ab", 1},
		{"abc", 1},
		{"abcd", 1},
		{"abcde", 2},
		{"Hello, world!", 4}, // 13 chars -> ~4 tokens
	}

	for _, tc := range tests {
		result := EstimateTokens(tc.input)
		if result != tc.expected {
			t.Errorf("EstimateTokens(%q) = %d, expected %d", tc.input, result, tc.expected)
		}
	}
}

func TestEstimateConversationTokens(t *testing.T) {
	conv := New()
	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi there!")

	tokens := EstimateConversationTokens(conv)
	// "Hello" = 2 tokens + 4 overhead = 6
	// "Hi there!" = 3 tokens + 4 overhead = 7
	// Total = 13
	if tokens != 13 {
		t.Errorf("expected 13 tokens, got %d", tokens)
	}

	// Nil conversation
	if EstimateConversationTokens(nil) != 0 {
		t.Error("expected 0 for nil conversation")
	}
}

func TestPromptBuilderBasic(t *testing.T) {
	builder := NewPromptBuilder(DefaultPromptConfig())
	conv := New()
	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi!")

	messages := builder.Build(conv)

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != RoleUser {
		t.Errorf("expected first role %s, got %s", RoleUser, messages[0].Role)
	}
	if messages[1].Role != RoleAssistant {
		t.Errorf("expected second role %s, got %s", RoleAssistant, messages[1].Role)
	}
}

func TestPromptBuilderWithSystemPrompt(t *testing.T) {
	config := DefaultPromptConfig()
	config.SystemPrompt = "You are a helpful assistant."
	builder := NewPromptBuilder(config)

	conv := New()
	conv.AddUserMessage("Hello")

	messages := builder.Build(conv)

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != RoleSystem {
		t.Errorf("expected first role %s, got %s", RoleSystem, messages[0].Role)
	}
	if messages[0].Content != "You are a helpful assistant." {
		t.Errorf("unexpected system prompt content: %s", messages[0].Content)
	}
}

func TestPromptBuilderEmptyConversation(t *testing.T) {
	builder := NewPromptBuilder(DefaultPromptConfig())
	conv := New()

	messages := builder.Build(conv)

	if len(messages) != 0 {
		t.Errorf("expected 0 messages for empty conversation, got %d", len(messages))
	}
}

func TestPromptBuilderNilConversation(t *testing.T) {
	builder := NewPromptBuilder(DefaultPromptConfig())

	messages := builder.Build(nil)

	if len(messages) != 0 {
		t.Errorf("expected 0 messages for nil conversation, got %d", len(messages))
	}
}

func TestPromptBuilderSystemPromptWithEmpty(t *testing.T) {
	config := DefaultPromptConfig()
	config.SystemPrompt = "System prompt"
	builder := NewPromptBuilder(config)

	messages := builder.Build(nil)

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "System prompt" {
		t.Errorf("expected system prompt content")
	}
}

func TestPromptBuilderTruncateByCount(t *testing.T) {
	config := DefaultPromptConfig()
	config.MaxMessages = 3
	config.PreserveSystemMessages = false
	builder := NewPromptBuilder(config)

	conv := New()
	conv.AddUserMessage("Message 1")
	conv.AddAssistantMessage("Response 1")
	conv.AddUserMessage("Message 2")
	conv.AddAssistantMessage("Response 2")
	conv.AddUserMessage("Message 3")

	messages := builder.Build(conv)

	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
	// Should keep the last 3 messages: Message 2, Response 2, Message 3
	if messages[0].Content != "Message 2" {
		t.Errorf("expected 'Message 2', got '%s'", messages[0].Content)
	}
	if messages[2].Content != "Message 3" {
		t.Errorf("expected 'Message 3', got '%s'", messages[2].Content)
	}
}

func TestPromptBuilderPreserveSystemMessages(t *testing.T) {
	config := DefaultPromptConfig()
	config.MaxMessages = 3
	config.PreserveSystemMessages = true
	builder := NewPromptBuilder(config)

	conv := New()
	conv.AddSystemMessage("System message")
	conv.AddUserMessage("Message 1")
	conv.AddAssistantMessage("Response 1")
	conv.AddUserMessage("Message 2")
	conv.AddAssistantMessage("Response 2")

	messages := builder.Build(conv)

	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
	// System message should be preserved
	if messages[0].Role != RoleSystem {
		t.Errorf("expected system message to be preserved")
	}
}

func TestPromptBuilderTruncateMiddle(t *testing.T) {
	config := DefaultPromptConfig()
	config.MaxMessages = 4
	config.TruncationStrategy = TruncateMiddle
	config.PreserveSystemMessages = false
	builder := NewPromptBuilder(config)

	conv := New()
	for i := 0; i < 10; i++ {
		conv.AddUserMessage("Message")
	}

	messages := builder.Build(conv)

	if len(messages) != 4 {
		t.Errorf("expected 4 messages, got %d", len(messages))
	}
}

func TestPromptBuilderTruncateByTokens(t *testing.T) {
	config := PromptConfig{
		MaxTokens:              20, // Very small limit
		PreserveSystemMessages: false,
	}
	builder := NewPromptBuilder(config)

	conv := New()
	conv.AddUserMessage("This is a long message that should be truncated")
	conv.AddAssistantMessage("Short")
	conv.AddUserMessage("Last")

	messages := builder.Build(conv)

	// Should have removed some messages to fit token limit
	if len(messages) >= 3 {
		t.Errorf("expected fewer than 3 messages due to token limit, got %d", len(messages))
	}
}

func TestBuildText(t *testing.T) {
	builder := NewPromptBuilder(DefaultPromptConfig())
	conv := New()
	conv.AddSystemMessage("System")
	conv.AddUserMessage("Hello")
	conv.AddAssistantMessage("Hi!")

	text := builder.BuildText(conv)

	if !strings.Contains(text, "[System]: System") {
		t.Error("expected system prefix in text")
	}
	if !strings.Contains(text, "[User]: Hello") {
		t.Error("expected user prefix in text")
	}
	if !strings.Contains(text, "[Assistant]: Hi!") {
		t.Error("expected assistant prefix in text")
	}
}

func TestBuildTextEmpty(t *testing.T) {
	builder := NewPromptBuilder(DefaultPromptConfig())

	text := builder.BuildText(nil)
	if text != "" {
		t.Errorf("expected empty string, got '%s'", text)
	}

	text = builder.BuildText(New())
	if text != "" {
		t.Errorf("expected empty string for empty conversation, got '%s'", text)
	}
}

func TestMergeSystemPromptWithExisting(t *testing.T) {
	config := DefaultPromptConfig()
	config.SystemPrompt = "Prepended"
	builder := NewPromptBuilder(config)

	conv := New()
	conv.AddSystemMessage("Original system")
	conv.AddUserMessage("Hello")

	messages := builder.Build(conv)

	// System prompts should be merged
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if !strings.Contains(messages[0].Content, "Prepended") {
		t.Error("expected prepended system prompt")
	}
	if !strings.Contains(messages[0].Content, "Original system") {
		t.Error("expected original system content to be preserved")
	}
}
