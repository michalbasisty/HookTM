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
		Usage:     "Replay webhooks to a target URL",
		ArgsUsage: "[id]",
		Description: `Replay captured webhooks to a target URL.

Exit codes (with --ci flag):
  0  Success (2xx response)
  1  Connection error
  2  HTTP error (4xx/5xx)
  3  Other error

Examples:
  hooktm replay abc123 --to localhost:3000
  hooktm replay abc123 --to http://api.example.com/webhook --dry-run
  hooktm replay --last 5 --to localhost:3000
  hooktm replay abc123 --to localhost:3000 --ci --json`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "to", Usage: "Target URL to replay to"},
			&cli.StringFlag{Name: "patch", Usage: "JSON merge patch to apply (RFC 7396)"},
			&cli.IntFlag{Name: "last", Usage: "Replay last N webhooks (newest first)"},
			&cli.BoolFlag{Name: "dry-run", Usage: "Show what would be sent without sending"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
			&cli.BoolFlag{Name: "ci", Usage: "CI mode: return non-zero exit code on failure"},
		},
		Action: runReplay,
	}
}

func runReplay(c *cli.Context) error {
	s, cfg, err := openStoreFromContext(c)
	if err != nil {
		return err
	}
	defer s.Close()

	// Get target URL
	target := strings.TrimSpace(c.String("to"))
	if target == "" {
		target = cfg.Forward
	}
	if target == "" {
		return fmt.Errorf("no target URL: use --to or set forward in config")
	}

	// Setup replay engine
	engine := replay.NewEngine(s)
	engine.DryRun = c.Bool("dry-run")

	patch := strings.TrimSpace(c.String("patch"))
	ciMode := c.Bool("ci")

	// Collect results and errors
	var results []replay.Result
	var replayErrors []error

	// Replay by --last or by ID
	if c.IsSet("last") && c.Int("last") > 0 {
		n := c.Int("last")
		rows, err := s.ListSummaries(c.Context, store.ListFilter{Limit: n})
		if err != nil {
			return err
		}
		for _, r := range rows {
			res, err := engine.ReplayByID(c.Context, r.ID, target, patch)
			if err != nil {
				replayErrors = append(replayErrors, err)
			} else {
				results = append(results, res)
			}
		}
	} else {
		id, err := requireArg(c, 0, "id")
		if err != nil {
			return err
		}
		res, err := engine.ReplayByID(c.Context, strings.TrimSpace(id), target, patch)
		if err != nil {
			replayErrors = append(replayErrors, err)
		} else {
			results = append(results, res)
		}
	}

	// Output results
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

	// Handle CI mode exit codes
	if ciMode {
		exitCode := 0
		for _, err := range replayErrors {
			code := getExitCodeFromError(err)
			if code > exitCode {
				exitCode = code
			}
		}
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
	} else if len(replayErrors) > 0 {
		return replayErrors[0]
	}
	return nil
}

func getExitCodeFromStatus(statusCode int) int {
	if statusCode >= 200 && statusCode < 300 {
		return 0
	}
	if statusCode >= 400 && statusCode < 600 {
		return 2
	}
	return 3
}

func getExitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if strings.Contains(err.Error(), "not found") {
		return 3
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return 1
	}
	var syscallErr *syscall.Errno
	if errors.As(err, &syscallErr) {
		return 1
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return 1
	}
	if urlErr, ok := err.(*url.Error); ok {
		if urlErr.Timeout() {
			return 1
		}
		return getExitCodeFromError(urlErr.Err)
	}
	return 3
}
