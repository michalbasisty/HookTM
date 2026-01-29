package cli

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// NewApp creates the CLI application with all commands.
func NewApp() *cli.App {
	app := &cli.App{
		Name:    "hooktm",
		Usage:   "Capture, inspect, and replay webhooks",
		Version: "0.2.0",
		Commands: []*cli.Command{
			newListenCmd(),
			newListCmd(),
			newShowCmd(),
			newReplayCmd(),
			newCodegenCmd(),
			newDeleteCmd(),
			newUICmd(),
		},
	}

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "db",
			Usage:   "Database file path",
			EnvVars: []string{"HOOKTM_DB"},
		},
		&cli.StringFlag{
			Name:    "config",
			Usage:   "Config file path",
			EnvVars: []string{"HOOKTM_CONFIG"},
		},
	}

	app.CommandNotFound = func(c *cli.Context, command string) {
		_, _ = fmt.Fprintf(c.App.ErrWriter, "Unknown command: %s\n", command)
	}

	return app
}
