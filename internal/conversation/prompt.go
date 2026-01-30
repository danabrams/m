package conversation

import (
	"strings"
)

// PromptConfig configures how prompts are built from conversations.
type PromptConfig struct {
	// MaxTokens is the approximate maximum number of tokens for the prompt.
	// Uses a simple character-based estimate (4 chars ≈ 1 token).
	// Set to 0 for no limit.
	MaxTokens int

	// MaxMessages is the maximum number of messages to include.
	// Set to 0 for no limit.
	MaxMessages int

	// SystemPrompt is prepended to the conversation if set.
	SystemPrompt string

	// TruncationStrategy determines how to truncate when limits are exceeded.
	TruncationStrategy TruncationStrategy

	// PreserveSystemMessages keeps system messages even when truncating.
	PreserveSystemMessages bool
}

// TruncationStrategy determines how conversations are truncated.
type TruncationStrategy int

const (
	// TruncateOldest removes oldest messages first (keep recent context).
	TruncateOldest TruncationStrategy = iota

	// TruncateMiddle removes messages from the middle (keep first and recent).
	TruncateMiddle
)

// DefaultPromptConfig returns a sensible default configuration.
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{
		MaxTokens:              100000, // ~100k tokens
		MaxMessages:            100,
		TruncationStrategy:     TruncateOldest,
		PreserveSystemMessages: true,
	}
}

// PromptMessage represents a message formatted for a prompt.
type PromptMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// PromptBuilder builds prompts from conversations.
type PromptBuilder struct {
	config PromptConfig
}

// NewPromptBuilder creates a new prompt builder with the given config.
func NewPromptBuilder(config PromptConfig) *PromptBuilder {
	return &PromptBuilder{config: config}
}

// Build creates a list of prompt messages from a conversation.
func (b *PromptBuilder) Build(conv *Conversation) []PromptMessage {
	if conv == nil || len(conv.Messages) == 0 {
		return b.withSystemPrompt(nil)
	}

	messages := conv.Messages
	config := b.config

	// Apply message count limit
	if config.MaxMessages > 0 && len(messages) > config.MaxMessages {
		messages = b.truncateByCount(messages, config.MaxMessages)
	}

	// Apply token limit
	if config.MaxTokens > 0 {
		messages = b.truncateByTokens(messages, config.MaxTokens)
	}

	// Convert to prompt messages
	result := make([]PromptMessage, len(messages))
	for i, msg := range messages {
		result[i] = PromptMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return b.withSystemPrompt(result)
}

// BuildText creates a formatted text prompt from a conversation.
// This is useful for CLI-based interfaces that expect a single text input.
func (b *PromptBuilder) BuildText(conv *Conversation) string {
	messages := b.Build(conv)
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, msg := range messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		switch msg.Role {
		case RoleSystem:
			sb.WriteString("[System]: ")
		case RoleUser:
			sb.WriteString("[User]: ")
		case RoleAssistant:
			sb.WriteString("[Assistant]: ")
		}
		sb.WriteString(msg.Content)
	}
	return sb.String()
}

// EstimateTokens provides a rough token count estimate for a string.
// Uses the approximation that 1 token ≈ 4 characters.
func EstimateTokens(s string) int {
	return (len(s) + 3) / 4
}

// EstimateConversationTokens estimates the total tokens in a conversation.
func EstimateConversationTokens(conv *Conversation) int {
	if conv == nil {
		return 0
	}
	total := 0
	for _, msg := range conv.Messages {
		total += EstimateTokens(msg.Content)
		total += 4 // Role token overhead
	}
	return total
}

func (b *PromptBuilder) withSystemPrompt(messages []PromptMessage) []PromptMessage {
	if b.config.SystemPrompt == "" {
		if messages == nil {
			return []PromptMessage{}
		}
		return messages
	}

	systemMsg := PromptMessage{
		Role:    RoleSystem,
		Content: b.config.SystemPrompt,
	}

	if messages == nil {
		return []PromptMessage{systemMsg}
	}

	// Check if first message is already a system message
	if len(messages) > 0 && messages[0].Role == RoleSystem {
		// Prepend system prompt to existing system message
		messages[0].Content = b.config.SystemPrompt + "\n\n" + messages[0].Content
		return messages
	}

	return append([]PromptMessage{systemMsg}, messages...)
}

func (b *PromptBuilder) truncateByCount(messages []*Message, maxCount int) []*Message {
	if len(messages) <= maxCount {
		return messages
	}

	switch b.config.TruncationStrategy {
	case TruncateMiddle:
		return b.truncateMiddle(messages, maxCount)
	default: // TruncateOldest
		return b.truncateOldest(messages, maxCount)
	}
}

func (b *PromptBuilder) truncateOldest(messages []*Message, maxCount int) []*Message {
	if !b.config.PreserveSystemMessages {
		return messages[len(messages)-maxCount:]
	}

	// Preserve system messages
	var systemMessages []*Message
	var otherMessages []*Message

	for _, msg := range messages {
		if msg.Role == RoleSystem {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Calculate how many non-system messages we can keep
	remainingSlots := maxCount - len(systemMessages)
	if remainingSlots <= 0 {
		// Only system messages fit
		return systemMessages[:maxCount]
	}

	// Keep the most recent non-system messages
	if len(otherMessages) > remainingSlots {
		otherMessages = otherMessages[len(otherMessages)-remainingSlots:]
	}

	// Merge system messages at their original positions
	result := make([]*Message, 0, len(systemMessages)+len(otherMessages))
	otherIdx := 0
	for _, msg := range messages {
		if msg.Role == RoleSystem {
			result = append(result, msg)
		} else if otherIdx < len(otherMessages) && msg == otherMessages[otherIdx] {
			result = append(result, msg)
			otherIdx++
		}
	}

	// If result is still too long, just take last maxCount
	if len(result) > maxCount {
		return result[len(result)-maxCount:]
	}

	return result
}

func (b *PromptBuilder) truncateMiddle(messages []*Message, maxCount int) []*Message {
	// Keep first few and last few messages
	keepFirst := maxCount / 4
	keepLast := maxCount - keepFirst

	if keepFirst < 1 {
		keepFirst = 1
	}
	if keepLast < 1 {
		keepLast = 1
	}

	first := messages[:keepFirst]
	last := messages[len(messages)-keepLast:]

	result := make([]*Message, 0, len(first)+len(last))
	result = append(result, first...)
	result = append(result, last...)
	return result
}

func (b *PromptBuilder) truncateByTokens(messages []*Message, maxTokens int) []*Message {
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += EstimateTokens(msg.Content) + 4
	}

	if totalTokens <= maxTokens {
		return messages
	}

	// Remove messages from the beginning until we're under the limit
	for len(messages) > 1 && totalTokens > maxTokens {
		if b.config.PreserveSystemMessages && messages[0].Role == RoleSystem {
			// Skip system messages when preserving them
			if len(messages) > 1 {
				removed := messages[1]
				messages = append(messages[:1], messages[2:]...)
				totalTokens -= EstimateTokens(removed.Content) + 4
			} else {
				break
			}
		} else {
			removed := messages[0]
			messages = messages[1:]
			totalTokens -= EstimateTokens(removed.Content) + 4
		}
	}

	return messages
}
