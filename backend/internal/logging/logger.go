package logging

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity.
type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

// Entry represents a structured log entry.
type Entry struct {
	Timestamp string                 `json:"timestamp"`
	Level     Level                  `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured JSON logging.
type Logger struct {
	mu     sync.Mutex
	output io.Writer
	level  Level
}

var levelOrder = map[Level]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
}

// NewLogger creates a new structured logger.
func NewLogger(output io.Writer, level Level) *Logger {
	if output == nil {
		output = os.Stdout
	}
	return &Logger{
		output: output,
		level:  level,
	}
}

func (l *Logger) shouldLog(level Level) bool {
	return levelOrder[level] >= levelOrder[l.level]
}

func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
		Fields:    fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.output.Write(data)
	l.output.Write([]byte("\n"))
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, fields map[string]interface{}) {
	l.log(LevelDebug, msg, fields)
}

// Info logs an info message.
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.log(LevelInfo, msg, fields)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, fields map[string]interface{}) {
	l.log(LevelWarn, msg, fields)
}

// Error logs an error message.
func (l *Logger) Error(msg string, fields map[string]interface{}) {
	l.log(LevelError, msg, fields)
}
