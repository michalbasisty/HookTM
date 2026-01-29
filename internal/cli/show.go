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
		Description: `Display full details of a captured webhook.

Examples:
  hooktm show abc123           # JSON output (default)
  hooktm show abc123 --format raw`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "json", Usage: "Output format: json|raw"},
		},
		Action: runShow,
	}
}

func runShow(c *cli.Context) error {
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

	switch strings.ToLower(c.String("format")) {
	case "raw":
		return showRaw(c, wh)
	case "json":
		return showJSON(c, wh)
	default:
		return fmt.Errorf("unknown format: %s (use json or raw)", c.String("format"))
	}
}

func showRaw(c *cli.Context, wh store.Webhook) error {
	_, _ = fmt.Fprintf(c.App.Writer, "%s %s%s\n", wh.Method, wh.Path, formatQuery(wh.Query))
	_, _ = fmt.Fprintf(c.App.Writer, "Time: %s\n", time.UnixMilli(wh.CreatedAt).UTC().Format(time.RFC3339))
	_, _ = fmt.Fprintf(c.App.Writer, "Provider: %s\n", defaultString(wh.Provider, "unknown"))
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
}

func showJSON(c *cli.Context, wh store.Webhook) error {
	enc := json.NewEncoder(c.App.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(wh)
}

func formatQuery(q string) string {
	if q == "" {
		return ""
	}
	if strings.HasPrefix(q, "?") {
		return q
	}
	return "?" + q
}

func defaultString(s, defaultVal string) string {
	if strings.TrimSpace(s) == "" {
		return defaultVal
	}
	return s
}
