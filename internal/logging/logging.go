// Package logging provides structured logging for the application
package logging

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var (
	// Logger is the global logger instance
	Logger zerolog.Logger
)

func init() {
	// Default to fatal-only logging until Init() is called
	// This prevents log spam before the app fully initializes
	Logger = zerolog.New(os.Stderr).Level(zerolog.FatalLevel)
}

// Config holds logging configuration
type Config struct {
	Level      string // debug, info, warn, error
	Console    bool   // Enable console output
	File       string // Log file path (empty to disable)
	TimeFormat string // Time format (empty for Unix timestamp)
}

// Init initializes the global logger
func Init(cfg Config) error {
	// Parse level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	var writers []io.Writer

	// Console output
	if cfg.Console {
		consoleWriter := zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		}
		writers = append(writers, consoleWriter)
	}

	// File output
	if cfg.File != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}

		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		writers = append(writers, file)
	}

	// Combine writers
	var output io.Writer
	if len(writers) == 0 {
		output = os.Stderr
	} else if len(writers) == 1 {
		output = writers[0]
	} else {
		output = zerolog.MultiLevelWriter(writers...)
	}

	// Create logger
	Logger = zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	return nil
}

// WithComponent returns a logger with a component field
func WithComponent(component string) zerolog.Logger {
	return Logger.With().Str("component", component).Logger()
}

// WithAccountID returns a logger with an account ID field
func WithAccountID(accountID string) zerolog.Logger {
	return Logger.With().Str("account_id", accountID).Logger()
}

// RedactEmail preserves a short local-part prefix and domain for diagnostics.
// Invalid values are never returned verbatim.
func RedactEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	if strings.Count(email, "@") != 1 {
		return "***"
	}
	parts := strings.SplitN(email, "@", 2)
	if parts[0] == "" || parts[1] == "" {
		return "***"
	}
	prefix := parts[0]
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	return prefix + "***@" + parts[1]
}

// ShortHash returns an opaque, stable reference suitable for diagnostic logs.
func ShortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

// Debug logs a debug message
func Debug() *zerolog.Event {
	return Logger.Debug()
}

// Info logs an info message
func Info() *zerolog.Event {
	return Logger.Info()
}

// Warn logs a warning message
func Warn() *zerolog.Event {
	return Logger.Warn()
}

// Error logs an error message
func Error() *zerolog.Event {
	return Logger.Error()
}

// Fatal logs a fatal message and exits
func Fatal() *zerolog.Event {
	return Logger.Fatal()
}
