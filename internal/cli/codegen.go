package cli

import (
	"fmt"
	"strings"

	"hooktm/internal/codegen"

	"github.com/urfave/cli/v2"
)

func newCodegenCmd() *cli.Command {
	return &cli.Command{
		Name:      "codegen",
		Usage:     "Generate validation code",
		ArgsUsage: "<id>",
		Description: `Generate signature validation code from a captured webhook.

Supported languages:
  go, golang     Go code
  ts, typescript TypeScript
  py, python     Python
  php            PHP
  rb, ruby       Ruby

Examples:
  hooktm codegen abc123 --lang go
  hooktm codegen abc123 --lang python`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "lang",
				Required: true,
				Usage:    "Language: go|ts|python|php|ruby",
			},
		},
		Action: runCodegen,
	}
}

func runCodegen(c *cli.Context) error {
	id, err := requireArg(c, 0, "id")
	if err != nil {
		return err
	}

	s, _, err := openStoreFromContext(c)
	if err != nil {
		return err
	}
	defer s.Close()

	wh, err := s.GetWebhook(c.Context, strings.TrimSpace(id))
	if err != nil {
		return err
	}

	out, err := codegen.RenderFromWebhook(wh, strings.TrimSpace(c.String("lang")))
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(c.App.Writer, out)
	return err
}
