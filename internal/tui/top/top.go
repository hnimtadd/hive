package top

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/hnimtadd/hive/pkg/config"
)

func Start(cfg *config.Config) error {
	model, err := newModel(cfg)
	if err != nil {
		return fmt.Errorf("failed to create new model: %w", err)
	}
	p := tea.NewProgram(model)
	defer p.Kill()

	_, err = p.Run()
	if err != nil {
		return fmt.Errorf("failed to run the program: %w", err)
	}
	return err
}
