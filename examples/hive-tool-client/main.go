package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/hnimtadd/hive/pkg/hive"
	"github.com/spf13/cobra"
)

var (
	method  string
	input   string
	verbose bool
)

// rootCmd represents the base command.
var rootCmd = &cobra.Command{
	Use:   "hive-tool --method [inspect|invoke] [--input '{...}'] <tool-path>",
	Short: "CLI tool for interacting with Hive tools",
	Long: `hive-tool is a CLI application that allows you to interact with Hive tools.
It supports two methods:
  - inspect: Retrieves tool metadata (name, description, schema)
  - invoke:  Executes the tool with provided input`,
	Example: `	# Inspect a tool:
		hive-tool --method invoke --input '{"name":"World"}' ./examples/tools/hive/tool.go
	# Invoke a tool with input:
		hive-tool --method inspect go run ./examples/tools/hive/tool.go`,
	RunE: func(_ *cobra.Command, entrypoint []string) error {
		return executeTool(entrypoint)
	},
}

func executeTool(entrypoint []string) error {
	if verbose {
		log.Printf("Entrypoint: %s\n", entrypoint)
		log.Printf("Method: %s\n", method)
	}

	// Run the tool as a subprocess
	if verbose {
		log.Printf("Running tool: %s\n", entrypoint)
	}
	path, err := exec.LookPath(entrypoint[0])
	if err != nil {
		return fmt.Errorf("command is not executable: %s", entrypoint[0])
	}

	cmd := exec.Command(path, entrypoint[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tool: %w", err)
	}

	// Create client to communicate with the tool
	client := hive.NewToolClient(stdout, stdin)

	switch method {
	case "inspect":
	case "invoke":
		if input == "" {
			return errors.New("input is required for 'invoke' method")
		}
	default:
		return fmt.Errorf("unsupported method: %s", method)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var resp *hive.Response

	switch method {
	case "inspect":
		resp, err = client.Inspect(ctx)
		if err != nil {
			return fmt.Errorf("inspect failed: %w", err)
		}

		defer cmd.Wait() //nolint: errcheck // this is acceptable

	case "invoke":
		var inputData json.RawMessage
		if err = json.Unmarshal([]byte(input), &inputData); err != nil {
			return fmt.Errorf("invalid input JSON: %w", err)
		}
		resp, err = client.Invoke(ctx, inputData)
		if err != nil {
			return fmt.Errorf("invoke failed: %w", err)
		}
		defer cmd.Wait() //nolint: errcheck // this is acceptable
	}

	// Print response
	output, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(output)) //nolint: forbidigo// this is accepted
	return nil
}

func main() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Command-specific flags
	rootCmd.Flags().StringVarP(&method, "method", "m", "inspect", "Method to call: 'inspect' or 'invoke'")
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "JSON input for 'invoke' method")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
