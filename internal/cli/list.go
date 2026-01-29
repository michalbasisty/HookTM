package cli

import (
	"encoding/json"
	"fmt"
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
