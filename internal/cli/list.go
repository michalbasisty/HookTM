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
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "limit", Value: 20, Usage: "Max rows to return"},
			&cli.StringFlag{Name: "provider", Usage: "Filter by provider"},
			&cli.IntFlag{Name: "status", Usage: "Filter by response status code"},
			&cli.StringFlag{Name: "search", Usage: "FTS search query (body text)"},
			&cli.StringFlag{Name: "from", Usage: "Filter from date/time (ISO 8601, YYYY-MM-DD, or relative like 1d, 7d)"},
			&cli.StringFlag{Name: "to", Usage: "Filter to date/time (ISO 8601, YYYY-MM-DD, or relative like 1d, 7d)"},
			&cli.BoolFlag{Name: "json", Usage: "Output as JSON"},
		},
		Action: func(c *cli.Context) error {
			s, _, err := openStoreFromContext(c)
			if err != nil {
				return err
			}
			defer s.Close()

			limit := c.Int("limit")
			f := store.ListFilter{
				Limit:    limit,
				Provider: strings.TrimSpace(c.String("provider")),
			}
			if c.IsSet("status") {
				f.StatusCode = ptrInt(c.Int("status"))
			}
			if c.IsSet("from") {
				t, err := parseTime(c.String("from"), true)
				if err != nil {
					return fmt.Errorf("invalid --from: %w", err)
				}
				f.From = t
			}
			if c.IsSet("to") {
				t, err := parseTime(c.String("to"), false)
				if err != nil {
					return fmt.Errorf("invalid --to: %w", err)
				}
				f.To = t
			}
			search := strings.TrimSpace(c.String("search"))

			var rows []store.WebhookSummary
			if search != "" {
				rows, err = s.SearchSummaries(c.Context, search, limit)
			} else {
				rows, err = s.ListSummaries(c.Context, f)
			}
			if err != nil {
				return err
			}

			if c.Bool("json") {
				enc := json.NewEncoder(c.App.Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			for _, r := range rows {
				ts := time.UnixMilli(r.CreatedAt).Local().Format("15:04:05")
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
		},
	}
}

func ptrInt(v int) *int { return &v }

// parseTime parses a time string. It supports:
// - ISO 8601 format: 2024-01-15T10:30:00Z
// - Date only: 2024-01-15 (interprets as start of day for from, end of day for to)
// - Relative duration: 1d, 7d, 1h (relative to now)
func parseTime(s string, isFrom bool) (*time.Time, error) {
	return parseTimeWithReference(s, isFrom, time.Now())
}

// parseTimeWithReference parses a time string relative to a reference time (for testing).
func parseTimeWithReference(s string, isFrom bool, ref time.Time) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty time string")
	}

	// Try relative duration first (ends with d, h, m, s)
	if isRelativeDuration(s) {
		d, err := parseDuration(s)
		if err != nil {
			return nil, err
		}
		t := ref.Add(-d)
		return &t, nil
	}

	// Try ISO 8601 formats
	formats := []string{
		time.RFC3339,           // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05Z", // 2006-01-02T15:04:05Z
		"2006-01-02 15:04:05",  // 2006-01-02 15:04:05
		"2006-01-02",           // Date only
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, time.Local); err == nil {
			// For date-only format, adjust to start/end of day
			if format == "2006-01-02" {
				if isFrom {
					// Start of day in UTC
					t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
				} else {
					// End of day in UTC
					t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
				}
			}
			return &t, nil
		}
	}

	return nil, fmt.Errorf("unable to parse time: %q (supported: ISO 8601, YYYY-MM-DD, or relative like 1d, 7d)", s)
}

// isRelativeDuration checks if the string looks like a relative duration.
func isRelativeDuration(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) < 2 {
		return false
	}
	last := s[len(s)-1]
	if last != 'd' && last != 'h' && last != 'm' && last != 's' {
		return false
	}
	// Check that the prefix is numeric
	prefix := s[:len(s)-1]
	if _, err := strconv.Atoi(prefix); err != nil {
		return false
	}
	return true
}
