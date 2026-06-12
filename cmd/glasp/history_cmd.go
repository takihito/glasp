package main

import (
	"fmt"
	"strings"

	"github.com/takihito/glasp/internal/config"
	"github.com/takihito/glasp/internal/history"
)

// HistoryCmd represents the 'history' subcommand.
type HistoryCmd struct {
	Limit   int    `name:"limit" help:"Maximum number of entries to show." default:"20"`
	Status  string `name:"status" help:"Filter by status (all|success|error)." default:"all"`
	Command string `name:"command" help:"Filter by exact command name."`
	Order   string `name:"order" help:"Sort order (desc|asc)." default:"desc"`
}

// Run executes the history command.
func (c *HistoryCmd) Run(rc *runContext) error {
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	if projectRoot == "" {
		return fmt.Errorf(".clasp.json not found in current or parent directories")
	}
	entries, err := history.Read(projectRoot, history.ReadOptions{
		Limit:   c.Limit,
		Status:  strings.TrimSpace(c.Status),
		Command: strings.TrimSpace(c.Command),
		Order:   strings.TrimSpace(c.Order),
	})
	if err != nil {
		return err
	}
	data, err := marshalJSONFn(entries)
	if err != nil {
		return fmt.Errorf("failed to marshal history entries: %w", err)
	}
	fmt.Fprintln(stdout, string(data))
	return nil
}
