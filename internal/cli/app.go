package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:  "hooktm",
		Usage: "Local-first webhook development companion",
		Commands: []*cli.Command{
			newListenCmd(),
			newListCmd(),
			newShowCmd(),
			newReplayCmd(),
			newUICmd(),
			newCodegenCmd(),
		},
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "db",
			Usage:   "Path to SQLite database",
			EnvVars: []string{"HOOKTM_DB"},
		},
		&cli.StringFlag{
			Name:    "config",
			Usage:   "Path to config file (optional)",
			EnvVars: []string{"HOOKTM_CONFIG"},
		},
	}

	app.Before = func(c *cli.Context) error {
		// Validation here later if needed.
		return nil
	}

	app.CommandNotFound = func(c *cli.Context, command string) {
		_, _ = fmt.Fprintf(c.App.ErrWriter, "Unknown command: %s\n", command)
	}

	return app
}
