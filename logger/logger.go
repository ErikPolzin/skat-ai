package logger

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/logging"
)

// Logger provides structured logging that works both locally and on GCP
type Logger struct {
	gcpLogger   *logging.Logger
	gcpClient   *logging.Client
	serviceName string
	local       bool
	disabled    bool
	minLevel    logging.Severity
}

var defaultLogger *Logger

// Initialize sets up the logger. Call this once at application startup.
// If projectID is empty or GCP credentials are not available, falls back to local logging.
// Set CLOUD_LOGGING_ENABLED=true to enable GCP Cloud Logging (requires GCP_PROJECT_ID).
// Set LOG_LEVEL to one of: DEBUG, INFO, WARNING, ERROR (defaults to INFO).
func Initialize(serviceName string) (*Logger, error) {
	l := &Logger{
		serviceName: serviceName,
		disabled:    false,
		minLevel:    logging.Info,
	}

	// Parse LOG_LEVEL environment variable
	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "DEBUG":
		l.minLevel = logging.Debug
	case "INFO":
		l.minLevel = logging.Info
	case "WARNING":
		l.minLevel = logging.Warning
	case "ERROR":
		l.minLevel = logging.Error
	default:
		l.minLevel = logging.Info
	}

	projectID := os.Getenv("GCP_PROJECT_ID")
	cloudLoggingEnabled := os.Getenv("CLOUD_LOGGING_ENABLED") == "true"
	l.local = projectID == "" || !cloudLoggingEnabled

	// Try to initialize GCP Cloud Logging if enabled and projectID is set
	if !l.local {
		ctx := context.Background()
		client, err := logging.NewClient(ctx, projectID)
		if err != nil {
			// Fall back to local logging if GCP is not available
			log.Printf("Warning: Failed to create GCP logging client, using local logging: %v", err)
			l.local = true
		} else {
			l.gcpClient = client
			l.gcpLogger = client.Logger(serviceName)
		}
	}

	if l.local {
		log.Printf("Using local logging for service: %s", serviceName)
	} else {
		log.Printf("Using GCP Cloud Logging for service: %s (project: %s)", serviceName, projectID)
	}

	defaultLogger = l
	return l, nil
}

// Close flushes and closes the GCP logging client
func (l *Logger) Close() error {
	if l.gcpClient != nil {
		return l.gcpClient.Close()
	}
	return nil
}

// Disable disables logging at runtime
func (l *Logger) Disable() {
	l.disabled = true
}

// Enable enables logging at runtime
func (l *Logger) Enable() {
	l.disabled = false
}

// Info logs an informational message
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.log(logging.Info, msg, fields...)
}

// Warning logs a warning message
func (l *Logger) Warning(msg string, fields ...interface{}) {
	l.log(logging.Warning, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.log(logging.Error, msg, fields...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.log(logging.Debug, msg, fields...)
}

// Fatal logs a fatal error and exits
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.log(logging.Critical, msg, fields...)
	l.Close()
	os.Exit(1)
}

// Printf provides compatibility with log.Printf
func (l *Logger) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Info(msg)
}

// log is the internal logging function
func (l *Logger) log(severity logging.Severity, msg string, fields ...interface{}) {
	// Skip logging if disabled or below minimum level
	if l.disabled || severity < l.minLevel {
		return
	}

	if l.local {
		// Local logging with standard log package
		prefix := severityPrefix(severity)
		log.Printf("%s %s", prefix, fmt.Sprintf(msg, fields...))
	} else {
		// GCP Cloud Logging with structured fields
		payload := map[string]interface{}{
			"message": msg,
		}

		// Add fields as key-value pairs
		for i := 0; i < len(fields)-1; i += 2 {
			key := fmt.Sprintf("%v", fields[i])
			payload[key] = fields[i+1]
		}

		entry := logging.Entry{
			Severity: severity,
			Payload:  payload,
		}

		l.gcpLogger.Log(entry)
	}
}

// severityPrefix returns a readable prefix for local logging
func severityPrefix(severity logging.Severity) string {
	switch severity {
	case logging.Debug:
		return "[DEBUG]"
	case logging.Info:
		return "[INFO]"
	case logging.Warning:
		return "[WARNING]"
	case logging.Error:
		return "[ERROR]"
	case logging.Critical:
		return "[CRITICAL]"
	default:
		return "[LOG]"
	}
}

// Package-level convenience functions using the default logger

// Info logs an informational message using the default logger
func Info(msg string, fields ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Info(msg, fields...)
	} else {
		log.Println("[INFO]", msg, fields)
	}
}

// Warning logs a warning message using the default logger
func Warning(msg string, fields ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Warning(msg, fields...)
	} else {
		log.Println("[WARNING]", msg, fields)
	}
}

// Error logs an error message using the default logger
func Error(msg string, fields ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Error(msg, fields...)
	} else {
		log.Println("[ERROR]", msg, fields)
	}
}

// Debug logs a debug message using the default logger
func Debug(msg string, fields ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Debug(msg, fields...)
	} else {
		log.Println("[DEBUG]", msg, fields)
	}
}

// Fatal logs a fatal error and exits using the default logger
func Fatal(msg string, fields ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Fatal(msg, fields...)
	} else {
		log.Fatal("[CRITICAL]", msg, fields)
	}
}

// Printf provides compatibility with log.Printf using the default logger
func Printf(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Disable disables logging at runtime using the default logger
func Disable() {
	if defaultLogger != nil {
		defaultLogger.Disable()
	}
}

// Enable enables logging at runtime using the default logger
func Enable() {
	if defaultLogger != nil {
		defaultLogger.Enable()
	}
}
