package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name       string
		logLevel   Level
		writeLevel Level
		shouldLog  bool
	}{
		{"debug logs debug", LevelDebug, LevelDebug, true},
		{"debug logs info", LevelDebug, LevelInfo, true},
		{"debug logs error", LevelDebug, LevelError, true},
		{"info skips debug", LevelInfo, LevelDebug, false},
		{"info logs info", LevelInfo, LevelInfo, true},
		{"info logs warn", LevelInfo, LevelWarn, true},
		{"error skips info", LevelError, LevelInfo, false},
		{"error skips warn", LevelError, LevelWarn, false},
		{"error logs error", LevelError, LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(&buf, tt.logLevel)

			switch tt.writeLevel {
			case LevelDebug:
				logger.Debug("test message", nil)
			case LevelInfo:
				logger.Info("test message", nil)
			case LevelWarn:
				logger.Warn("test message", nil)
			case LevelError:
				logger.Error("test message", nil)
			}

			hasOutput := buf.Len() > 0
			if hasOutput != tt.shouldLog {
				t.Errorf("shouldLog=%v but hasOutput=%v", tt.shouldLog, hasOutput)
			}
		})
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, LevelDebug)

	fields := map[string]interface{}{
		"user_id": "abc-123",
		"action":  "login",
		"count":   42,
	}

	logger.Info("user logged in", fields)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Level != LevelInfo {
		t.Errorf("level = %s, want INFO", entry.Level)
	}
	if entry.Message != "user logged in" {
		t.Errorf("message = %q, want %q", entry.Message, "user logged in")
	}
	if entry.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
	if entry.Fields["user_id"] != "abc-123" {
		t.Errorf("field user_id = %v, want abc-123", entry.Fields["user_id"])
	}
	if entry.Fields["action"] != "login" {
		t.Errorf("field action = %v, want login", entry.Fields["action"])
	}
}

func TestLoggerNilFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, LevelDebug)

	logger.Info("no fields", nil)

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Fields != nil {
		t.Errorf("fields should be nil/omitted, got %v", entry.Fields)
	}
}

func TestLoggerNewline(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, LevelDebug)

	logger.Info("line1", nil)
	logger.Info("line2", nil)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

func TestLoggerDefaultOutput(t *testing.T) {
	// NewLogger with nil output should not panic
	logger := NewLogger(nil, LevelInfo)
	logger.Info("should not panic", nil) // writes to os.Stdout
}
