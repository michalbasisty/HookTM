package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"hooktm/internal/store"

	"github.com/urfave/cli/v2"
)

func newDeleteCmd() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete webhooks",
		ArgsUsage: "[id]",
		Description: `Delete webhooks by ID or by filter criteria.

Examples:
  hooktm delete abc123                    # Delete by ID
  hooktm delete --older-than 7d           # Delete older than 7 days
  hooktm delete --provider stripe         # Delete all Stripe webhooks
  hooktm delete --status 500              # Delete failed webhooks
  hooktm delete --older-than 30d --yes    # Skip confirmation`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "older-than", Usage: "Delete webhooks older than duration (e.g., 1d, 7d, 30d)"},
			&cli.StringFlag{Name: "provider", Usage: "Delete by provider name"},
			&cli.IntFlag{Name: "status", Usage: "Delete by HTTP status code"},
			&cli.BoolFlag{Name: "yes", Usage: "Skip confirmation prompt"},
		},
		Action: runDelete,
	}
}

func runDelete(c *cli.Context) error {
	s, _, err := openStoreFromContext(c)
	if err != nil {
		return err
	}
	defer s.Close()

	id := strings.TrimSpace(c.Args().First())
	hasFilter := c.IsSet("older-than") || c.IsSet("provider") || c.IsSet("status")

	// Validate arguments
	if id == "" && !hasFilter {
		return fmt.Errorf("specify an ID or at least one filter (--older-than, --provider, --status)")
	}
	if id != "" && hasFilter {
		return fmt.Errorf("cannot use ID and filters together")
	}

	// Delete by ID
	if id != "" {
		if !c.Bool("yes") {
			fmt.Fprintf(c.App.Writer, "Delete webhook %s? [y/N] ", id)
			var resp string
			if _, err := fmt.Fscanln(c.App.Reader, &resp); err != nil {
				return fmt.Errorf("cancelled")
			}
			if strings.ToLower(strings.TrimSpace(resp)) != "y" {
				return fmt.Errorf("cancelled")
			}
		}
		if err := s.DeleteWebhook(c.Context, id); err != nil {
			return err
		}
		fmt.Fprintf(c.App.Writer, "Deleted: %s\n", id)
		return nil
	}

	// Delete by filter
	filter := store.DeleteFilter{
		Provider: strings.TrimSpace(c.String("provider")),
	}
	if c.IsSet("status") {
		filter.StatusCode = intPtr(c.Int("status"))
	}
	if c.IsSet("older-than") {
		d, err := parseDuration(c.String("older-than"))
		if err != nil {
			return fmt.Errorf("invalid --older-than: %w", err)
		}
		filter.OlderThan = d
	}

	// Confirm bulk delete
	if !c.Bool("yes") {
		desc := describeFilter(filter)
		fmt.Fprintf(c.App.Writer, "Delete webhooks matching: %s? [y/N] ", desc)
		var resp string
		if _, err := fmt.Fscanln(c.App.Reader, &resp); err != nil {
			return fmt.Errorf("cancelled")
		}
		if strings.ToLower(strings.TrimSpace(resp)) != "y" {
			return fmt.Errorf("cancelled")
		}
	}

	n, err := s.DeleteByFilter(c.Context, filter)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "Deleted %d webhook(s)\n", n)
	return nil
}

func describeFilter(f store.DeleteFilter) string {
	var parts []string
	if f.OlderThan > 0 {
		parts = append(parts, fmt.Sprintf("older than %s", f.OlderThan))
	}
	if f.Provider != "" {
		parts = append(parts, fmt.Sprintf("provider=%s", f.Provider))
	}
	if f.StatusCode != nil {
		parts = append(parts, fmt.Sprintf("status=%d", *f.StatusCode))
	}
	if len(parts) == 0 {
		return "(all)"
	}
	return strings.Join(parts, ", ")
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid days: %s", daysStr)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}


