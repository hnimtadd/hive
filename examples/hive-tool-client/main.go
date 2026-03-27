package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hnimtadd/hive/pkg/hive"
	"github.com/spf13/cobra"
)

var (
	method  string
	input   string
	verbose bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "hive-tool --method [inspect|invoke] [--input '{...}'] <tool-path>",
	Short: "CLI tool for interacting with Hive tools",
	Long: `hive-tool is a CLI application that allows you to interact with Hive tools.

It supports two methods:
  - inspect: Retrieves tool metadata (name, description, schema)
  - invoke:  Executes the tool with provided input

Examples:
  # Inspect a tool
  hive-tool --method inspect ./examples/tools/hive/tool.go

  # Invoke a tool with input
  hive-tool --method invoke --input '{"name":"World"}' ./examples/tools/hive/tool.go`,
	RunE: func(_ *cobra.Command, entrypoint []string) error {
		return executeTool(entrypoint)
	},
}

func executeTool(entrypoint []string) error {
	if verbose {
		log.Printf("Entrypoint: %s\n", entrypoint)
		log.Printf("Method: %s\n", method)
	}

	// Validate method
	if method != "inspect" && method != "invoke" {
		return fmt.Errorf("invalid method: %s. Must be 'inspect' or 'invoke'", method)
	}

	// If method is invoke, input is required
	if method == "invoke" && input == "" {
		return fmt.Errorf("input is required for 'invoke' method. Use --input flag")
	}

	// Run the tool as a subprocess
	if verbose {
		log.Printf("Running tool: %s\n", entrypoint)
	}

	cmd := exec.Command(entrypoint[0], entrypoint[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tool: %w", err)
	}
	defer cmd.Wait() // nolint: errcheck

	// Create client to communicate with the tool
	client := hive.NewToolClient(stdout, stdin)

	ctx := context.Background()
	var resp *hive.Response

	switch method {
	case "inspect":
		resp, err = client.Inspect(ctx)
		if err != nil {
			return fmt.Errorf("inspect failed: %w", err)
		}
	case "invoke":
		var inputData json.RawMessage
		if err := json.Unmarshal([]byte(input), &inputData); err != nil {
			return fmt.Errorf("invalid input JSON: %w", err)
		}
		resp, err = client.InvokeRaw(ctx, inputData)
		if err != nil {
			return fmt.Errorf("invoke failed: %w", err)
		}
	}

	// Close stdin to signal we're done
	stdin.Close()

	// Print response
	output, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

// compileTool compiles a Go tool file and returns the path to the binary
func compileTool(toolPath string) (string, error) {
	// Create temporary file for the binary
	tmpDir := os.TempDir()
	binaryName := fmt.Sprintf("hive-tool-%d", os.Getpid())
	binaryPath := filepath.Join(tmpDir, binaryName)

	// Build the tool
	cmd := exec.Command("go", "build", "-o", binaryPath, toolPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build failed: %w", err)
	}

	return binaryPath, nil
}

func main() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Command-specific flags
	rootCmd.Flags().StringVarP(&method, "method", "m", "inspect", "Method to call: 'inspect' or 'invoke'")
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "JSON input for 'invoke' method")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
