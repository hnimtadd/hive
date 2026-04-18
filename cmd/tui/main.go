package main

import (
	"fmt"
	"log"

	"github.com/hnimtadd/hive/internal/tui/top"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive TUI for Hive",
	RunE:  runTUI,
}

func runTUI(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("tui: failed to load configuration: %w", err)
	}
	return top.Start(cfg)
}
