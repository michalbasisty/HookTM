package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hooktm/internal/store"

	"github.com/urfave/cli/v2"
)

func newShowCmd() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "Show webhook details",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "json", Usage: "json|raw"},
		},
		Action: func(c *cli.Context) error {
			id, err := requireArg(c, 0, "id")
			if err != nil {
				return err
			}
			id = strings.TrimSpace(id)

			s, _, err := openStoreFromContext(c)
			if err != nil {
				return err
			}
			defer s.Close()

			wh, err := s.GetWebhook(c.Context, id)
			if err != nil {
				return err
			}

			switch strings.ToLower(strings.TrimSpace(c.String("format"))) {
			case "raw":
				_, _ = fmt.Fprintf(c.App.Writer, "%s %s%s\n", wh.Method, wh.Path, withQuery(wh.Query))
				_, _ = fmt.Fprintf(c.App.Writer, "Captured: %s\n", time.UnixMilli(wh.CreatedAt).UTC().Format(time.RFC3339Nano))
				_, _ = fmt.Fprintf(c.App.Writer, "Provider: %s\n", emptyTo(wh.Provider, "unknown"))
				_, _ = fmt.Fprintf(c.App.Writer, "Event: %s\n", wh.EventType)
				_, _ = fmt.Fprintf(c.App.Writer, "Status: %v\n", wh.StatusCode)
				_, _ = fmt.Fprintf(c.App.Writer, "Latency: %dms\n", wh.ResponseMS)
				_, _ = fmt.Fprintf(c.App.Writer, "\nHeaders:\n")
				for k, vs := range wh.Headers {
					for _, v := range vs {
						_, _ = fmt.Fprintf(c.App.Writer, "  %s: %s\n", k, v)
					}
				}
				_, _ = fmt.Fprintf(c.App.Writer, "\nBody (%d bytes):\n", len(wh.Body))
				_, _ = c.App.Writer.Write(wh.Body)
				_, _ = fmt.Fprintf(c.App.Writer, "\n")
				return nil
			case "json":
				enc := json.NewEncoder(c.App.Writer)
				enc.SetIndent("", "  ")
				return enc.Encode(wh)
			default:
				return fmt.Errorf("unknown format: %s", c.String("format"))
			}
		},
	}
}

func withQuery(q string) string {
	if q == "" {
		return ""
	}
	if strings.HasPrefix(q, "?") {
		return q
	}
	return "?" + q
}

func emptyTo(s, v string) string {
	if strings.TrimSpace(s) == "" {
		return v
	}
	return s
}

var _ store.Webhook = store.Webhook{}
