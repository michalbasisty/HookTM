package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"

	"hooktm/internal/replay"
	"hooktm/internal/store"

	"github.com/urfave/cli/v2"
)

func newReplayCmd() *cli.Command {
	return &cli.Command{
		Name:      "replay",
		Usage:     "Replay captured webhooks",
		ArgsUsage: "[id]",
		Description: `Replay captured webhooks to a target URL.

Exit codes (when using --ci):
  0  Success (2xx response)
  1  Connection error (network/DNS/timeout)
  2  HTTP error (4xx/5xx response)
  3  Other error (not found, invalid input, etc.)`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "to", Usage: "Override replay target base URL (default: forward target from config)"},
			&cli.StringFlag{Name: "patch", Usage: "RFC7396 JSON merge patch applied to JSON bodies"},
			&cli.IntFlag{Name: "last", Usage: "Replay last N webhooks (newest first)"},
			&cli.BoolFlag{Name: "dry-run", Usage: "Print what would be sent, without sending"},
			&cli.BoolFlag{Name: "json", Usage: "Output results as JSON"},
			&cli.BoolFlag{Name: "ci", Usage: "CI mode: exit with non-zero code on failure"},
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
			ciMode := c.Bool("ci")

			var results []replay.Result
			var replayErrors []error
			var anySuccess bool

			if c.IsSet("last") && c.Int("last") > 0 {
				n := c.Int("last")
				rows, err := s.ListSummaries(c.Context, store.ListFilter{Limit: n})
				if err != nil {
					return err
				}
				for _, r := range rows {
					res, err := engine.ReplayByID(c.Context, r.ID, target, patch)
					results = append(results, res)
					if err != nil {
						replayErrors = append(replayErrors, err)
					} else if res.Sent && res.StatusCode >= 200 && res.StatusCode < 300 {
						anySuccess = true
					}
				}
			} else {
				id, err := requireArg(c, 0, "id")
				if err != nil {
					return err
				}
				res, err := engine.ReplayByID(c.Context, strings.TrimSpace(id), target, patch)
				results = append(results, res)
				if err != nil {
					replayErrors = append(replayErrors, err)
				} else if res.Sent && res.StatusCode >= 200 && res.StatusCode < 300 {
					anySuccess = true
				}
			}

			if c.Bool("json") {
				enc := json.NewEncoder(c.App.Writer)
				enc.SetIndent("", "  ")
				if err := enc.Encode(results); err != nil {
					return err
				}
			} else {
				for _, r := range results {
					if r.Sent {
						_, _ = fmt.Fprintf(c.App.Writer, "Replayed %s → %s (%d)\n", r.WebhookID, r.URL, r.StatusCode)
					} else {
						_, _ = fmt.Fprintf(c.App.Writer, "Dry-run %s → %s\n", r.WebhookID, r.URL)
					}
				}
			}

			// CI mode: determine exit code
			if ciMode {
				exitCode := 0
				for _, err := range replayErrors {
					code := getExitCodeFromError(err)
					if code > exitCode {
						exitCode = code
					}
				}
				// Also check for HTTP errors in results
				for _, r := range results {
					if r.Sent {
						code := getExitCodeFromStatus(r.StatusCode)
						if code > exitCode {
							exitCode = code
						}
					}
				}
				if exitCode != 0 {
					return cli.Exit("", exitCode)
				}
			} else {
				// Non-CI mode: return first error if any
				if len(replayErrors) > 0 {
					return replayErrors[0]
				}
			}
			return nil
		},
	}
}
