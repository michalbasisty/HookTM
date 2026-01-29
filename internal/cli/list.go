package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"hooktm/internal/store"

	"github.com/urfave/cli/v2"
)

func newListCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List captured webhooks",
		Description: `Display captured webhooks with optional filtering.

Date formats for --from and --to:
  YYYY-MM-DD           Date only (e.g., 2024-01-15)
  ISO 8601             Full timestamp (e.g., 2024-01-15T10:30:00Z)
  Relative             1d, 7d, 30d, 1h (e.g., --from 7d for last 7 days)

Examples:
  hooktm list                                    # Show recent 20 webhooks
  hooktm list --limit 50                         # Show 50 webhooks
  hooktm list --provider stripe                  # Filter by provider
  hooktm list --from 7d                          # Last 7 days
  hooktm list --from 2024-01-01 --to 2024-01-31  # Date range
  hooktm list --search "payment"                 # Search body text
  hooktm list --json                             # JSON output`,
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Value: 20, Usage: "Maximum number of results"},
			&cli.StringFlag{Name: "provider", Usage: "Filter by provider (stripe, github, etc.)"},
			&cli.IntFlag{Name: "status", Usage: "Filter by HTTP status code"},
			&cli.StringFlag{Name: "search", Usage: "Search in webhook body text"},
			&cli.StringFlag{Name: "from", Usage: "Start date/time"},
			&cli.StringFlag{Name: "to", Usage: "End date/time"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: runList,
	}
}

func runList(c *cli.Context) error {
	s, _, err := openStoreFromContext(c)
	if err != nil {
		return err
	}
	defer s.Close()

	limit := c.Int("limit")
	filter := store.ListFilter{
		Limit:    limit,
		Provider: strings.TrimSpace(c.String("provider")),
	}

	if c.IsSet("status") {
		filter.StatusCode = intPtr(c.Int("status"))
	}

	// Parse date filters
	if c.IsSet("from") {
		t, err := parseTime(c.String("from"), true)
		if err != nil {
			return fmt.Errorf("invalid --from: %w", err)
		}
		filter.From = t
	}

	if c.IsSet("to") {
		t, err := parseTime(c.String("to"), false)
		if err != nil {
			return fmt.Errorf("invalid --to: %w", err)
		}
		filter.To = t
	}

	search := strings.TrimSpace(c.String("search"))

	// Execute query
	var rows []store.WebhookSummary
	if search != "" {
		rows, err = s.SearchSummaries(c.Context, search, limit)
	} else {
		rows, err = s.ListSummaries(c.Context, filter)
	}
	if err != nil {
		return err
	}

	// Output
	if c.Bool("json") {
		enc := json.NewEncoder(c.App.Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	for _, r := range rows {
		ts := formatTimestamp(r.CreatedAt)
		status := "-"
		if r.StatusCode != nil {
			status = fmt.Sprintf("%d", *r.StatusCode)
		}
		prov := r.Provider
		if prov == "" {
			prov = "unknown"
		}
		_, _ = fmt.Fprintf(c.App.Writer, "%s  %s  %s  %s  [%s]  %dms\n",
			r.ID, ts, r.Method, r.Path, prov+"/"+status, r.ResponseMS)
	}
	return nil
}

func formatTimestamp(ms int64) string {
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05")
}

func intPtr(v int) *int { return &v }

// parseTime parses a time string supporting:
// - ISO 8601: 2024-01-15T10:30:00Z
// - Date only: 2024-01-15 (start/end of day based on isFrom)
// - Relative: 1d, 7d, 1h (relative to now)
func parseTime(s string, isFrom bool) (*time.Time, error) {
	return parseTimeWithReference(s, isFrom, time.Now())
}

func parseTimeWithReference(s string, isFrom bool, ref time.Time) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty time string")
	}

	// Try relative duration first
	if isRelativeDuration(s) {
		d, err := parseDuration(s)
		if err != nil {
			return nil, err
		}
		t := ref.Add(-d)
		return &t, nil
	}

	// Try various formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, time.Local); err == nil {
			// Adjust date-only to start/end of day
			if format == "2006-01-02" {
				if isFrom {
					t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
				} else {
					t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
				}
			}
			return &t, nil
		}
	}

	return nil, fmt.Errorf("invalid time format: %q (use YYYY-MM-DD, ISO 8601, or relative like 7d)", s)
}

func isRelativeDuration(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 2 {
		return false
	}
	last := s[len(s)-1]
	if last != 'd' && last != 'h' && last != 'm' && last != 's' {
		return false
	}
	prefix := s[:len(s)-1]
	_, err := strconv.Atoi(prefix)
	return err == nil
}
