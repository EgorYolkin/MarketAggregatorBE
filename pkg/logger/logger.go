// Package logger provides a structured slog logger with support for console and file output,
// along with OpenTelemetry trace context integration.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Log is the global logger instance. Initialized with a default JSON handler to avoid nil panics.
var Log *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Config holds the configuration for the logger.
type Config struct {
	Level           string
	Directory       string
	TimestampFormat string
	Format          string
	EnableConsole   bool
	EnableFile      bool
}

// InitLogger initializes the global logger instance with the given configuration.
// Returns an error if file output is requested but the log directory or file cannot be created.
func InitLogger(cfg Config) error {
	var writer io.Writer

	switch {
	case cfg.EnableConsole && cfg.EnableFile:
		fw, err := createFileWriter(cfg.Directory)
		if err != nil {
			return fmt.Errorf("create file writer: %w", err)
		}
		writer = io.MultiWriter(os.Stdout, fw)
	case cfg.EnableConsole:
		writer = os.Stdout
	case cfg.EnableFile:
		fw, err := createFileWriter(cfg.Directory)
		if err != nil {
			return fmt.Errorf("create file writer: %w", err)
		}
		writer = fw
	default:
		return fmt.Errorf("at least one output (console or file) must be enabled for logger")
	}

	opts := &slog.HandlerOptions{
		Level: ParseLogLevel(cfg.Level),
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && cfg.TimestampFormat != "" {
				return slog.String(a.Key, a.Value.Time().Format(cfg.TimestampFormat))
			}
			return a
		},
	}

	var baseHandler slog.Handler
	if strings.ToLower(cfg.Format) == "text" {
		baseHandler = slog.NewTextHandler(writer, opts)
	} else {
		baseHandler = slog.NewJSONHandler(writer, opts)
	}

	// Wrap handler to inject trace information from context.
	handler := &traceHandler{Handler: baseHandler}
	Log = slog.New(handler)

	// Set as the process-wide default so bare slog.Info() calls also use it.
	slog.SetDefault(Log)

	return nil
}

// createFileWriter opens (or creates) a date-stamped log file inside dir.
func createFileWriter(dir string) (io.Writer, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	logFileName := fmt.Sprintf("%s/app-%s.log", dir, time.Now().Format("2006-01-02"))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) //nolint:gosec // Path built from trusted config directory.
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return logFile, nil
}

// ParseLogLevel converts a string level to a slog.Level.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// traceHandler is a slog.Handler middleware that enriches every log record
// with trace_id and span_id extracted from the context, if present.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		sc := trace.SpanFromContext(ctx).SpanContext()
		if sc.IsValid() {
			r.AddAttrs(
				slog.String("trace_id", sc.TraceID().String()),
				slog.String("span_id", sc.SpanID().String()),
			)
		}
	}
	return h.Handler.Handle(ctx, r)
}

// Fatal logs at error level and terminates the process.
// Use sparingly — only in main() or top-level bootstrap code.
func Fatal(msg string, args ...any) {
	Log.Error(msg, args...)
	os.Exit(1)
}
