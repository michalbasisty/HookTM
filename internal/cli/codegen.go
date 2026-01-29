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
		Usage:     "Generate signature validation code from captured webhook",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "lang", Required: true, Usage: "go|ts|python|php|ruby"},
		},
		Action: func(c *cli.Context) error {
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
		},
	}
}
