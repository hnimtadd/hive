package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hnimtadd/hive/internal/transport/client"
	"github.com/hnimtadd/hive/internal/tui/top"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "client for Hive",
		RunE: func(cmd *cobra.Command, args []string) error {
			inlineQuery, err := cmd.Flags().GetString("prompt")
			if err != nil {
				return err
			}

			if inlineQuery != "" {
				return runCli(inlineQuery)
			}
			return runTUI(cmd, args)
		},
	}
	cmd.Flags().StringP("prompt", "p", "", "inline string query")
	return cmd
}

func runCli(question string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("cli: failed to load configuration: %w", err)
	}
	c, err := client.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("cli: failed to init new client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := c.ExecuteTaskInline(ctx, question)
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

func runTUI(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("tui: failed to load configuration: %w", err)
	}
	return top.Start(cfg)
}

func main() {
	rootCmd := rootCmd()
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
