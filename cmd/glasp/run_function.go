package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

// RunFunctionCmd represents the 'run-function' subcommand.
type RunFunctionCmd struct {
	FunctionName string `arg:"" help:"Function name to run."`
	NonDev       bool   `name:"nondev" help:"Run in non-dev mode (devMode=false)."`
	Params       string `name:"params" short:"p" help:"A JSON string array of parameters."`
	Auth         string `help:"Path to .clasprc.json used for authentication."`
}

// Run executes the run-function command.
func (c *RunFunctionCmd) Run(ctx *kong.Context) error {
	authPath, err := optionalAuthPath(c.Auth)
	if err != nil {
		return err
	}
	functionName := strings.TrimSpace(c.FunctionName)
	if functionName == "" {
		return fmt.Errorf("function name is required")
	}
	projectRoot, err := findExistingProjectRoot()
	if err != nil {
		return err
	}
	scriptID, err := scriptIDFromConfig(projectRoot)
	if err != nil {
		return err
	}
	params, err := parseRunParams(c.Params)
	if err != nil {
		return err
	}
	client, err := newProjectScriptClient(context.Background(), projectRoot, authPath)
	if err != nil {
		return err
	}
	op, err := client.RunFunction(context.Background(), scriptID, functionName, params, !c.NonDev)
	if err != nil {
		return err
	}
	if op == nil {
		return fmt.Errorf("empty execution response")
	}
	if !op.Done {
		return fmt.Errorf("script execution is still in progress")
	}
	if op.Error != nil {
		message := strings.TrimSpace(op.Error.Message)
		if message != "" || op.Error.Code != 0 {
			return fmt.Errorf("script execution failed: code=%d message=%s", op.Error.Code, message)
		}
		return fmt.Errorf("script execution failed")
	}
	if len(op.Response) == 0 {
		fmt.Println("{}")
		return nil
	}
	fmt.Printf("%s\n", string(op.Response))
	return nil
}

func parseRunParams(raw string) ([]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil, fmt.Errorf("params must be a JSON array: %w", err)
	}
	params, ok := decoded.([]any)
	if !ok {
		return nil, fmt.Errorf("params must be a JSON array")
	}
	return params, nil
}
