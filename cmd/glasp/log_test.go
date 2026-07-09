package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogHandlerLevel(t *testing.T) {
	tests := []struct {
		level    string
		enabled  []slog.Level
		disabled []slog.Level
	}{
		{
			level:    "debug",
			enabled:  []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError},
			disabled: nil,
		},
		{
			level:    "info",
			enabled:  []slog.Level{slog.LevelInfo, slog.LevelWarn, slog.LevelError},
			disabled: []slog.Level{slog.LevelDebug},
		},
		{
			level:    "warn",
			enabled:  []slog.Level{slog.LevelWarn, slog.LevelError},
			disabled: []slog.Level{slog.LevelDebug, slog.LevelInfo},
		},
		{
			level:    "error",
			enabled:  []slog.Level{slog.LevelError},
			disabled: []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn},
		},
		{
			// kong's enum tag rejects unknown values before newLogHandler
			// runs, but the fallback branch must still behave like info.
			level:    "unknown",
			enabled:  []slog.Level{slog.LevelInfo, slog.LevelWarn, slog.LevelError},
			disabled: []slog.Level{slog.LevelDebug},
		},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			h := newLogHandler(&bytes.Buffer{}, tt.level, "text")
			for _, lvl := range tt.enabled {
				if !h.Enabled(ctx, lvl) {
					t.Errorf("level %q: expected %v to be enabled", tt.level, lvl)
				}
			}
			for _, lvl := range tt.disabled {
				if h.Enabled(ctx, lvl) {
					t.Errorf("level %q: expected %v to be disabled", tt.level, lvl)
				}
			}
		})
	}
}

func TestNewLogHandlerFormat(t *testing.T) {
	t.Run("json emits parseable JSON records", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(newLogHandler(&buf, "info", "json"))
		logger.Warn("json format check", "key", "value")

		var record map[string]any
		if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
			t.Fatalf("expected JSON output, got %q: %v", buf.String(), err)
		}
		if record["msg"] != "json format check" || record["key"] != "value" {
			t.Fatalf("unexpected JSON record: %v", record)
		}
	})

	t.Run("text emits key=value records", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(newLogHandler(&buf, "info", "text"))
		logger.Warn("text format check", "key", "value")

		out := buf.String()
		if !strings.Contains(out, `msg="text format check"`) || !strings.Contains(out, "key=value") {
			t.Fatalf("expected text output, got %q", out)
		}
		if json.Valid(buf.Bytes()) {
			t.Fatalf("text format should not be valid JSON: %q", out)
		}
	})
}

// TestDefaultLevelSuppressesDebug pins the migration's main behavior change:
// debug-level traces (e.g. OAuth flow progress) stay hidden at the default
// info level.
func TestDefaultLevelSuppressesDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(newLogHandler(&buf, "info", "text"))
	logger.Debug("hidden trace")
	logger.Info("visible notice")

	out := buf.String()
	if strings.Contains(out, "hidden trace") {
		t.Fatalf("debug log should be suppressed at info level, got %q", out)
	}
	if !strings.Contains(out, "visible notice") {
		t.Fatalf("info log should be emitted at info level, got %q", out)
	}
}
