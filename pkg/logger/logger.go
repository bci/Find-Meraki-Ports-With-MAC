// Package logger provides a simple level-based logging system with file output support.
package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message.
type LogLevel int

const (
	// LevelDebug is for detailed diagnostic information.
	LevelDebug LogLevel = iota
	// LevelInfo is for general informational messages.
	LevelInfo
	// LevelWarning is for warning messages that may require attention.
	LevelWarning
	// LevelError is for error messages indicating problems.
	LevelError
)

// Logger provides structured logging with levels and file output.
type Logger struct {
	level  LogLevel
	writer io.Writer
}

// ParseLogLevel converts a string to a LogLevel.
// Accepts: "DEBUG", "INFO", "WARNING"/"WARN", "ERROR"
// Defaults to LevelDebug if the input is invalid.
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return LevelDebug
	case "INFO":
		return LevelInfo
	case "WARNING", "WARN":
		return LevelWarning
	case "ERROR":
		return LevelError
	default:
		return LevelDebug
	}
}

// New creates a new logger with the specified log file and level.
// If logFile is empty, logs are written to stderr only.
// If the file cannot be opened, logs are written to stderr with a warning.
func New(logFile string, level LogLevel) *Logger {
	var writer io.Writer = os.Stderr
	if strings.TrimSpace(logFile) != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			writer = io.MultiWriter(os.Stderr, file)
		} else {
			fmt.Fprintf(os.Stderr, "failed to open log file %s: %v\n", logFile, err)
		}
	}
	return &Logger{level: level, writer: writer}
}

// NewWriter creates a logger that writes to the provided io.Writer directly.
func NewWriter(w io.Writer, level LogLevel) *Logger {
	return &Logger{level: level, writer: w}
}

func (l *Logger) logf(level LogLevel, label string, format string, args ...interface{}) {
	if l == nil || !(level >= l.level) {
		return
	}
	timestamp := time.Now().Format(time.RFC3339)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.writer, "%s [%s] %s\n", timestamp, label, msg)
}

// Debugf logs a debug message with formatting.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logf(LevelDebug, "DEBUG", format, args...)
}

// Infof logs an info message with formatting.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf(LevelInfo, "INFO", format, args...)
}

// Warnf logs a warning message with formatting.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logf(LevelWarning, "WARNING", format, args...)
}

// Errorf logs an error message with formatting.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf(LevelError, "ERROR", format, args...)
}
