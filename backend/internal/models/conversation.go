package models

// ConversationMessage for multi-turn NLP conversation.
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
