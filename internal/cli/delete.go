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
		Usage:     "Delete captured webhooks",
		ArgsUsage: "[id]",
		Description: `Delete webhooks by ID or by filter criteria.

Examples:
  hooktm delete abc123                    # Delete specific webhook
  hooktm delete --older-than 7d           # Delete webhooks older than 7 days
  hooktm delete --provider slack          # Delete all Slack webhooks
  hooktm delete --status 500              # Delete webhooks with 500 status`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "older-than",
				Usage: "Delete webhooks older than duration (e.g., 1h, 1d, 7d, 30d)",
			},
			&cli.StringFlag{
				Name:  "provider",
				Usage: "Delete webhooks by provider (stripe, github, etc.)",
			},
			&cli.IntFlag{
				Name:  "status",
				Usage: "Delete webhooks by response status code",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "Skip confirmation prompt",
			},
		},
		Action: func(c *cli.Context) error {
			s, _, err := openStoreFromContext(c)
			if err != nil {
				return err
			}
			defer s.Close()

			id := strings.TrimSpace(c.Args().First())
			hasFilter := c.IsSet("older-than") || c.IsSet("provider") || c.IsSet("status")

			// Must specify either an ID or at least one filter
			if id == "" && !hasFilter {
				return fmt.Errorf("specify either an ID or at least one filter (--older-than, --provider, --status)")
			}
			if id != "" && hasFilter {
				return fmt.Errorf("cannot specify both ID and filters")
			}

			// Single ID delete
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

			// Bulk delete by filter
			filter := store.DeleteFilter{
				Provider: strings.TrimSpace(c.String("provider")),
			}
			if c.IsSet("status") {
				filter.StatusCode = ptrInt(c.Int("status"))
			}
			if c.IsSet("older-than") {
				d, err := parseDuration(c.String("older-than"))
				if err != nil {
					return fmt.Errorf("invalid --older-than: %w", err)
				}
				filter.OlderThan = d
			}

			if !c.Bool("yes") {
				var desc []string
				if filter.OlderThan > 0 {
					desc = append(desc, fmt.Sprintf("older than %s", filter.OlderThan))
				}
				if filter.Provider != "" {
					desc = append(desc, fmt.Sprintf("provider=%s", filter.Provider))
				}
				if filter.StatusCode != nil {
					desc = append(desc, fmt.Sprintf("status=%d", *filter.StatusCode))
				}
				fmt.Fprintf(c.App.Writer, "Delete all webhooks matching: %s? [y/N] ", strings.Join(desc, ", "))
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
		},
	}
}

// parseDuration extends time.ParseDuration with support for 'd' (days) suffix.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// Handle days suffix
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
