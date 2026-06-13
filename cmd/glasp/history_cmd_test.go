package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
)

func TestHistoryCLIParsesOptions(t *testing.T) {
	var cli CLI
	parser, err := kong.New(&cli)
	if err != nil {
		t.Fatalf("failed to build parser: %v", err)
	}
	_, err = parser.Parse([]string{"history", "--limit", "5", "--status", "error", "--command", "pull", "--order", "asc"})
	if err != nil {
		t.Fatalf("expected history to parse, got %v", err)
	}
	if cli.History.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", cli.History.Limit)
	}
	if cli.History.Status != "error" {
		t.Fatalf("expected status error, got %q", cli.History.Status)
	}
	if cli.History.Command != "pull" {
		t.Fatalf("expected command pull, got %q", cli.History.Command)
	}
	if cli.History.Order != "asc" {
		t.Fatalf("expected order asc, got %q", cli.History.Order)
	}
}

func TestHistoryCommandNoFileReturnsNil(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	out, err := captureStdout(t, func() error { return (&HistoryCmd{}).Run(nil) })
	if err != nil {
		t.Fatalf("HistoryCmd.Run failed: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Fatalf("expected [] output, got %q", out)
	}
}

func TestHistoryCommandFiltersByCommand(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "pull", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "push", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	entries, err := history.Read(root, history.ReadOptions{Command: "pull", Order: "asc"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Command != "pull" {
		t.Fatalf("unexpected filtered entries: %#v", entries)
	}
}

func TestHistoryCommandOutputsJSON(t *testing.T) {
	root := useTempDir(t)
	if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "script-id"}); err != nil {
		t.Fatalf("SaveClaspConfig failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "pull", Status: "success"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if err := history.Append(root, history.Entry{Command: "push", Status: "error", Error: "boom"}); err != nil {
		t.Fatalf("append failed: %v", err)
	}

	out, err := captureStdout(t, func() error {
		return (&HistoryCmd{Order: "asc"}).Run(nil)
	})
	if err != nil {
		t.Fatalf("HistoryCmd.Run failed: %v", err)
	}
	var entries []history.Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entries); err != nil {
		t.Fatalf("history output is not valid JSON: %v (out=%q)", err, out)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}
	if entries[0].Command != "pull" || entries[1].Command != "push" {
		t.Fatalf("unexpected history order/commands: %#v", entries)
	}
}
