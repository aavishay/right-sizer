package logger

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("info", "test")

	assert.NotNil(t, logger)
	assert.Equal(t, INFO, logger.level)
	assert.Equal(t, "test", logger.prefix)
	assert.NotNil(t, logger.logger)
}

func TestInit(t *testing.T) {
	// Save original global logger
	original := Global

	// Test init
	Init("debug")

	assert.NotNil(t, Global)
	assert.Equal(t, DEBUG, Global.level)
	assert.Empty(t, Global.prefix)

	// Restore original
	Global = original
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"info", INFO},
		{"INFO", INFO},
		{"warn", WARN},
		{"warning", WARN},
		{"WARN", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"unknown", INFO}, // default
		{"", INFO},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  DEBUG,
		logger: log.New(&buf, "", 0),
	}

	logger.Debug("test message %s", "arg")

	output := buf.String()
	assert.Contains(t, output, "test message arg")
	assert.Contains(t, output, "DEBUG")
}

func TestLogger_Debug_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO, // Higher than DEBUG
		logger: log.New(&buf, "", 0),
	}

	logger.Debug("test message")

	output := buf.String()
	assert.Empty(t, output) // Should not log
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	logger.Info("test message %s", "arg")

	output := buf.String()
	assert.Contains(t, output, "test message arg")
	// Info should not have [INFO] prefix for cleaner output
	assert.NotContains(t, output, "[INFO]")
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  WARN,
		logger: log.New(&buf, "", 0),
	}

	logger.Warn("test warning %s", "arg")

	output := buf.String()
	assert.Contains(t, output, "test warning arg")
	assert.Contains(t, output, "WARN")
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  ERROR,
		logger: log.New(&buf, "", 0),
	}

	logger.Error("test error %s", "arg")

	output := buf.String()
	assert.Contains(t, output, "test error arg")
	assert.Contains(t, output, "ERROR")
}

func TestLogger_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	logger.Success("test success %s", "arg")

	output := buf.String()
	assert.Contains(t, output, "test success arg")
	// Success should not have [SUCCESS] prefix for cleaner output
	assert.NotContains(t, output, "[SUCCESS]")
}

func TestLogger_WithPrefix(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	prefixedLogger := logger.WithPrefix("TEST")

	assert.NotNil(t, prefixedLogger)
	assert.Equal(t, "TEST", prefixedLogger.prefix)
	assert.Equal(t, logger.level, prefixedLogger.level)
	assert.Equal(t, logger.logger, prefixedLogger.logger)
}

func TestLogger_WithPrefix_Logging(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		prefix: "PREFIX",
		logger: log.New(&buf, "", 0),
	}

	logger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "[PREFIX] test message")
}

func TestLogger_SetLevel(t *testing.T) {
	logger := &Logger{
		level: INFO,
	}

	logger.SetLevel("debug")
	assert.Equal(t, DEBUG, logger.level)

	logger.SetLevel("error")
	assert.Equal(t, ERROR, logger.level)
}

func TestLogger_FormatMessage_WithColor(t *testing.T) {
	// Mock terminal output by temporarily setting stdout to a pipe
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger := &Logger{
		level:  INFO,
		logger: log.New(os.Stdout, "", 0),
	}

	// This should use colors when outputting to terminal
	logger.formatMessage("TEST", "\033[31m", "test message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should contain ANSI color codes
	assert.Contains(t, output, "\033[31m")
	assert.Contains(t, output, "\033[0m")
}

func TestLogger_FormatMessage_WithoutColor(t *testing.T) {
	// Mock non-terminal output
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	// Force non-terminal mode by checking file mode
	message := logger.formatMessage("TEST", "\033[31m", "test message")

	// Should not contain ANSI color codes when not outputting to terminal
	assert.NotContains(t, message, "\033[31m")
	assert.NotContains(t, message, "\033[0m")
	assert.Contains(t, message, "[TEST]")
	assert.Contains(t, message, "test message")
}

func TestGlobalFunctions(t *testing.T) {
	// Save original global logger
	original := Global
	defer func() { Global = original }()

	var buf bytes.Buffer
	Global = &Logger{
		level:  DEBUG,
		logger: log.New(&buf, "", 0),
	}

	// Test all global functions
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")
	Success("success message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "success message")
}

func TestGlobalFunctions_NoGlobalLogger(t *testing.T) {
	// Save original global logger
	original := Global
	Global = nil
	defer func() { Global = original }()

	// These should not panic, but use default log package
	assert.NotPanics(t, func() { Debug("test") })
	assert.NotPanics(t, func() { Info("test") })
	assert.NotPanics(t, func() { Warn("test") })
	assert.NotPanics(t, func() { Error("test") })
	assert.NotPanics(t, func() { Success("test") })
}

func TestNew(t *testing.T) {
	logger := New(INFO)

	assert.NotNil(t, logger)
	assert.Equal(t, INFO, logger.level)
	assert.Empty(t, logger.prefix)
	assert.NotNil(t, logger.logger)
}

func TestGetLogger(t *testing.T) {
	// Save original global logger
	original := Global
	defer func() { Global = original }()

	Global = nil

	logger := GetLogger()

	assert.NotNil(t, logger)
	assert.Equal(t, INFO, logger.level)
	assert.NotNil(t, Global) // Should create global instance
}

func TestGetLogger_Existing(t *testing.T) {
	// Save original global logger
	original := Global
	defer func() { Global = original }()

	expectedLogger := &Logger{level: DEBUG}
	Global = expectedLogger

	logger := GetLogger()

	assert.Equal(t, expectedLogger, logger)
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		loggerLevel LogLevel
		logLevel    LogLevel
		shouldLog   bool
	}{
		{DEBUG, DEBUG, true},
		{DEBUG, INFO, true},
		{DEBUG, WARN, true},
		{DEBUG, ERROR, true},
		{INFO, DEBUG, false},
		{INFO, INFO, true},
		{INFO, WARN, true},
		{INFO, ERROR, true},
		{WARN, DEBUG, false},
		{WARN, INFO, false},
		{WARN, WARN, true},
		{WARN, ERROR, true},
		{ERROR, DEBUG, false},
		{ERROR, INFO, false},
		{ERROR, WARN, false},
		{ERROR, ERROR, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("logger_%d_log_%d", tt.loggerLevel, tt.logLevel), func(t *testing.T) {
			var buf bytes.Buffer
			logger := &Logger{
				level:  tt.loggerLevel,
				logger: log.New(&buf, "", 0),
			}

			// Log a message at the specified level
			switch tt.logLevel {
			case DEBUG:
				logger.Debug("test")
			case INFO:
				logger.Info("test")
			case WARN:
				logger.Warn("test")
			case ERROR:
				logger.Error("test")
			}

			output := buf.String()
			if tt.shouldLog {
				assert.NotEmpty(t, output)
			} else {
				assert.Empty(t, output)
			}
		})
	}
}

func TestLogger_MultiplePrefixes(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		prefix: "PARENT",
		logger: log.New(&buf, "", 0),
	}

	childLogger := logger.WithPrefix("CHILD")

	logger.Info("parent message")
	childLogger.Info("child message")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], "[PARENT] parent message")
	assert.Contains(t, lines[1], "[CHILD] child message")
}

func TestLogger_EmptyFormatString(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	logger.Info("")
	logger.Info("simple message")

	output := buf.String()
	assert.Contains(t, output, "simple message")
}

func TestLogger_FormatMessage_Timestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  INFO,
		logger: log.New(&buf, "", 0),
	}

	logger.Info("test message")

	output := buf.String()
	// Should contain timestamp in format YYYY/MM/DD HH:MM:SS
	assert.Regexp(t, `\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`, output)
}

func TestLogger_Success_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		level:  ERROR, // Higher level
		logger: log.New(&buf, "", 0),
	}

	logger.Success("test message")

	output := buf.String()
	assert.Empty(t, output) // Success should respect log level
}
