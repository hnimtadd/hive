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

	"github.com/hnimtadd/hive/internal/tools/system"
	transportClient "github.com/hnimtadd/hive/internal/transport/client"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/hive"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "client [hive|system|session] input",
	Short:   "CLI tool for interacting with Hive tools",
	Example: `client hive --method invoke --input '{"name":"World"}' ./examples/tools/hive/tool.go`,
}

// hiveCmd represents the base command.
var hiveCmd = &cobra.Command{
	Use:   "hive",
	Short: "CLI tool for interacting with Hive tools",
	Long: `hive-tool is a CLI application that allows you to interact with Hive tools.
It supports two methods:
  - inspect: Retrieves tool metadata (name, description, schema)
  - invoke:  Executes the tool with provided input`,
	Example: `	# Inspect a tool:
		hive-tool --method invoke --input '{"name":"World"}' ./examples/tools/hive/tool.go
	# Invoke a tool with input:
		hive-tool --method inspect go run ./examples/tools/hive/tool.go`,
	RunE: executeTool,
}

func executeTool(cmd *cobra.Command, entrypoint []string) error {
	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		return err
	}
	method, err := cmd.Flags().GetString("method")
	if err != nil {
		return err
	}
	input, err := cmd.Flags().GetString("input")
	if err != nil {
		return err
	}

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

	execCmd := exec.Command(path, entrypoint[1:]...)
	stdin, err := execCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := execCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	execCmd.Env = os.Environ()
	if err = execCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tool: %w", err)
	}

	// Create client to communicate with the tool
	client := hive.NewToolClient(stdout, stdin, stderr)

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
			log.Println("debug logs:", client.DebugLog())
			return fmt.Errorf("inspect failed: %w", err)
		}

		defer execCmd.Wait() //nolint: errcheck // this is acceptable

	case "invoke":
		var inputData json.RawMessage
		if err = json.Unmarshal([]byte(input), &inputData); err != nil {
			return fmt.Errorf("invalid input JSON: %w", err)
		}
		resp, err = client.Invoke(ctx, inputData)
		if err != nil {
			log.Println("debug logs:", client.DebugLog())
			return fmt.Errorf("invoke failed: %w", err)
		}
		defer execCmd.Wait() //nolint: errcheck // this is acceptable
	}

	// Print response
	output, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	log.Println(string(output))
	log.Println("debug logs:", client.DebugLog())
	return nil
}

var systemCmd = &cobra.Command{
	Use:     "system",
	Short:   "Hive system tool",
	Example: `client system [list|invoke]`,
}

var systemList = &cobra.Command{
	Use:     "list",
	Short:   "List system tools",
	Example: `client system list`,
	RunE:    listTool,
}

// systemCmd represents the base command.
var systemInvoke = &cobra.Command{
	Use:     "invoke",
	Short:   "CLI tool for interacting with Hive system tools",
	Example: "client system invoke --name tool-name --input 'json-input'",
	RunE:    executeSystemTool,
}

func executeSystemTool(cmd *cobra.Command, _ []string) error {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	input, err := cmd.Flags().GetString("input")
	if err != nil {
		return err
	}
	tools, err := system.Tools()
	if err != nil {
		return err
	}
	tool, exists := tools[name]
	if !exists {
		return fmt.Errorf("tool %s is not exists", name)
	}
	output, err := tool.InvokableRun(context.Background(), input)
	if err != nil {
		return fmt.Errorf("execute failed: %w", err)
	}
	log.Println(output)
	return nil
}

func listTool(cmd *cobra.Command, _ []string) error {
	tools, err := system.Tools()
	if err != nil {
		return err
	}
	output := map[string]string{}
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			return err
		}
		output[info.Name] = info.Desc
	}
	outputBytes, err := json.MarshalIndent(output, "", "\t")
	if err != nil {
		return err
	}
	log.Println(string(outputBytes))
	return nil
}

// systemCmd represents the base command.
var systemHelp = &cobra.Command{
	Use:     "help",
	Short:   "CLI tool for interacting with Hive system tools",
	Example: "client system help --name tool-name ",
	RunE:    systemHelpTool,
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session API client commands",
}

var sessionInlineCmd = &cobra.Command{
	Use:   "inline [prompt]",
	Short: "Run one inline session round",
	Args:  cobra.ExactArgs(1),
	RunE:  executeSessionInline,
}

func executeSessionInline(_ *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client, err := transportClient.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create transport client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := client.ExecuteTaskInline(ctx, args[0])
	if err != nil {
		return fmt.Errorf("inline session request failed: %w", err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal inline result: %w", err)
	}

	log.Println(string(output))
	return nil
}

func systemHelpTool(cmd *cobra.Command, _ []string) error {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	tools, err := system.Tools()
	if err != nil {
		return err
	}
	tool, exists := tools[name]
	if !exists {
		return fmt.Errorf("tool %s is not exists", name)
	}

	info, err := tool.Info(context.Background())
	if err != nil {
		return fmt.Errorf("get info failed failed: %w", err)
	}

	data := map[string]any{}
	schema, err := info.ToJSONSchema()
	if err != nil {
		return err
	}
	data["name"] = info.Name
	data["description"] = info.Desc
	data["schema"] = schema

	outputBytes, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	log.Println(string(outputBytes))
	return nil
}

func main() {
	// Global flags
	systemCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")

	// Command-specific flags
	hiveCmd.Flags().String("method", "inspect", "Method to call: 'inspect' or 'invoke'")
	hiveCmd.Flags().String("input", "", "JSON input for 'invoke' method")
	systemInvoke.Flags().String("name", "", "Tool to call")
	systemInvoke.Flags().String("input", "", "JSON input for tool call")
	systemHelp.Flags().String("name", "", "Tool to help")

	systemCmd.AddCommand(systemInvoke)
	systemCmd.AddCommand(systemList)
	systemCmd.AddCommand(systemHelp)
	rootCmd.AddCommand(hiveCmd)
	rootCmd.AddCommand(systemCmd)
	sessionCmd.AddCommand(sessionInlineCmd)
	rootCmd.AddCommand(sessionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
