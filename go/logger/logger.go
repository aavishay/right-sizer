// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger represents a logger with configurable level
type Logger struct {
	level  LogLevel
	prefix string
	logger *log.Logger
}

var (
	// Global logger instance
	Global *Logger

	// Color codes for different log levels
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorGreen  = "\033[32m"
)

// NewLogger creates a new logger with the specified level
func NewLogger(levelStr string, prefix string) *Logger {
	level := parseLogLevel(levelStr)
	return &Logger{
		level:  level,
		prefix: prefix,
		logger: log.New(os.Stdout, "", 0),
	}
}

// Init initializes the global logger
func Init(levelStr string) {
	Global = NewLogger(levelStr, "")
}

// parseLogLevel converts a string log level to LogLevel
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return DEBUG
	case "info":
		return INFO
	case "warn", "warning":
		return WARN
	case "error":
		return ERROR
	default:
		return INFO
	}
}

// formatMessage formats a log message with timestamp and level
func (l *Logger) formatMessage(level string, color string, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	message := fmt.Sprintf(format, args...)

	// Add prefix if set
	if l.prefix != "" {
		message = fmt.Sprintf("[%s] %s", l.prefix, message)
	}

	// Check if we should use colors (when outputting to terminal)
	useColor := false
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		useColor = true
	}
	// Force color if explicitly requested (e.g., tests or forced CI rendering)
	if !useColor && os.Getenv("FORCE_LOG_COLOR") == "1" {
		useColor = true
	}

	if useColor {
		return fmt.Sprintf("%s %s[%s]%s %s", timestamp, color, level, colorReset, message)
	}
	return fmt.Sprintf("%s [%s] %s", timestamp, level, message)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= DEBUG {
		msg := l.formatMessage("DEBUG", colorGray, format, args...)
		l.logger.Println(msg)
	}
}

// Info logs an info message (without level prefix for cleaner output)
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= INFO {
		timestamp := time.Now().Format("2006/01/02 15:04:05")
		message := fmt.Sprintf(format, args...)

		// Add prefix if set
		if l.prefix != "" {
			message = fmt.Sprintf("[%s] %s", l.prefix, message)
		}

		// Format without [INFO] prefix for cleaner logs
		msg := fmt.Sprintf("%s %s", timestamp, message)
		l.logger.Println(msg)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= WARN {
		msg := l.formatMessage("WARN", colorYellow, format, args...)
		l.logger.Println(msg)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level <= ERROR {
		msg := l.formatMessage("ERROR", colorRed, format, args...)
		l.logger.Println(msg)
	}
}

// Success logs a success message (always shown, without level prefix for cleaner output)
func (l *Logger) Success(format string, args ...interface{}) {
	if l.level <= INFO {
		timestamp := time.Now().Format("2006/01/02 15:04:05")
		message := fmt.Sprintf(format, args...)

		// Add prefix if set
		if l.prefix != "" {
			message = fmt.Sprintf("[%s] %s", l.prefix, message)
		}

		// Format without [INFO] prefix for cleaner logs
		msg := fmt.Sprintf("%s %s", timestamp, message)
		l.logger.Println(msg)
	}
}

// SetLevel changes the log level
func (l *Logger) SetLevel(levelStr string) {
	l.level = parseLogLevel(levelStr)
}

// WithPrefix creates a new logger with a prefix
func (l *Logger) WithPrefix(prefix string) *Logger {
	return &Logger{
		level:  l.level,
		prefix: prefix,
		logger: l.logger,
	}
}

// Global logging functions that use the global logger

// Debug logs a debug message using the global logger
func Debug(format string, args ...interface{}) {
	if Global != nil {
		Global.Debug(format, args...)
	} else {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs an info message using the global logger
func Info(format string, args ...interface{}) {
	if Global != nil {
		Global.Info(format, args...)
	} else {
		log.Printf("[INFO] "+format, args...)
	}
}

// Warn logs a warning message using the global logger
func Warn(format string, args ...interface{}) {
	if Global != nil {
		Global.Warn(format, args...)
	} else {
		log.Printf("[WARN] "+format, args...)
	}
}

// Error logs an error message using the global logger
func Error(format string, args ...interface{}) {
	if Global != nil {
		Global.Error(format, args...)
	} else {
		log.Printf("[ERROR] "+format, args...)
	}
}

// Success logs a success message using the global logger
func Success(format string, args ...interface{}) {
	if Global != nil {
		Global.Success(format, args...)
	} else {
		log.Printf("[SUCCESS] "+format, args...)
	}
}

// New creates a new logger with the specified level
func New(level LogLevel) *Logger {
	return &Logger{
		level:  level,
		prefix: "",
		logger: log.New(os.Stdout, "", 0),
	}
}

// GetLogger returns the global logger instance, creating it if necessary
func GetLogger() *Logger {
	if Global == nil {
		Global = New(INFO)
	}
	return Global
}
