package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"hooktm/internal/replay"
	"hooktm/internal/store"

	"github.com/urfave/cli/v2"
)

func newReplayCmd() *cli.Command {
	return &cli.Command{
		Name:      "replay",
		Usage:     "Replay captured webhooks",
		ArgsUsage: "[id]",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "to", Usage: "Override replay target base URL (default: forward target from config)"},
			&cli.StringFlag{Name: "patch", Usage: "RFC7396 JSON merge patch applied to JSON bodies"},
			&cli.IntFlag{Name: "last", Usage: "Replay last N webhooks (newest first)"},
			&cli.BoolFlag{Name: "dry-run", Usage: "Print what would be sent, without sending"},
			&cli.BoolFlag{Name: "json", Usage: "Output results as JSON"},
		},
		Action: func(c *cli.Context) error {
			s, cfg, err := openStoreFromContext(c)
			if err != nil {
				return err
			}
			defer s.Close()

			target := strings.TrimSpace(c.String("to"))
			if target == "" {
				target = cfg.Forward
			}
			if target == "" {
				return fmt.Errorf("no replay target: set --to or config forward")
			}

			engine := replay.NewEngine(s)
			engine.DryRun = c.Bool("dry-run")

			patch := strings.TrimSpace(c.String("patch"))

			var results []replay.Result
			if c.IsSet("last") && c.Int("last") > 0 {
				n := c.Int("last")
				rows, err := s.ListSummaries(c.Context, store.ListFilter{Limit: n})
				if err != nil {
					return err
				}
				for _, r := range rows {
					res, err := engine.ReplayByID(c.Context, r.ID, target, patch)
					if err != nil {
						return err
					}
					results = append(results, res)
				}
			} else {
				id, err := requireArg(c, 0, "id")
				if err != nil {
					return err
				}
				res, err := engine.ReplayByID(c.Context, strings.TrimSpace(id), target, patch)
				if err != nil {
					return err
				}
				results = append(results, res)
			}

			if c.Bool("json") {
				enc := json.NewEncoder(c.App.Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			for _, r := range results {
				if r.Sent {
					_, _ = fmt.Fprintf(c.App.Writer, "Replayed %s → %s (%d)\n", r.WebhookID, r.URL, r.StatusCode)
				} else {
					_, _ = fmt.Fprintf(c.App.Writer, "Dry-run %s → %s\n", r.WebhookID, r.URL)
				}
			}
			return nil
		},
	}
}
