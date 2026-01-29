package cli

import (
	"hooktm/internal/tui"

	"github.com/urfave/cli/v2"
)

func newUICmd() *cli.Command {
	return &cli.Command{
		Name:  "ui",
		Usage: "Interactive TUI",
		Action: func(c *cli.Context) error {
			s, cfg, err := openStoreFromContext(c)
			if err != nil {
				return err
			}
			defer s.Close()

			return tui.Run(c.Context, s, cfg.Forward)
		},
	}
}
