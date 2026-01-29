package cli

import (
	"hooktm/internal/tui"

	"github.com/urfave/cli/v2"
)

func newUICmd() *cli.Command {
	return &cli.Command{
		Name:  "ui",
		Usage: "Open interactive UI",
		Description: `Launch the interactive terminal UI for browsing webhooks.

Navigation:
  ↑/↓ or j/k    Move up/down
  Enter         View details
  /             Search
  q             Quit`,
		Action: runUI,
	}
}

func runUI(c *cli.Context) error {
	s, cfg, err := openStoreFromContext(c)
	if err != nil {
		return err
	}
	defer s.Close()

	return tui.Run(c.Context, s, cfg.Forward)
}
