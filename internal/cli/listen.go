package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"hooktm/internal/proxy"

	"github.com/urfave/cli/v2"
)

func newListenCmd() *cli.Command {
	return &cli.Command{
		Name:      "listen",
		Usage:     "Start webhook capture server",
		ArgsUsage: "<port>",
		Description: `Start a server that captures incoming webhooks.

The server records all incoming requests to the database for later inspection.
Use --forward to proxy requests to your local development server.

Examples:
  hooktm listen 8080                           # Record only
  hooktm listen 8080 --forward localhost:3000  # Proxy to local server
  hooktm listen 8080 --forward http://api.example.com/webhook`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "forward",
				Usage: "Forward requests to this URL (e.g., localhost:3000 or http://host:port)",
			},
		},
		Action: runListen,
	}
}

func runListen(c *cli.Context) error {
	port, err := requireArg(c, 0, "port")
	if err != nil {
		return err
	}
	port = strings.TrimSpace(port)

	s, cfg, err := openStoreFromContext(c)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := ensureDirForFile(s.Path()); err != nil {
		return err
	}

	// Get forward target
	target := c.String("forward")
	if target == "" {
		target = cfg.Forward
	}

	var targetURL *url.URL
	if target != "" {
		targetURL, err = parseForwardTarget(target)
		if err != nil {
			return err
		}
	}

	// Start server
	addr := net.JoinHostPort("", port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           proxy.NewRecorderProxy(targetURL, s),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Print status
	if targetURL != nil {
		_, _ = fmt.Fprintf(c.App.Writer, "Listening on :%s → %s\n", port, targetURL.String())
	} else {
		_, _ = fmt.Fprintf(c.App.Writer, "Listening on :%s (record-only)\n", port)
	}
	_, _ = fmt.Fprintf(c.App.Writer, "Press Ctrl+C to stop\n")

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func parseForwardTarget(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty forward target")
	}

	// Allow host:port shorthand
	if !strings.Contains(s, "://") && strings.Contains(s, ":") {
		s = "http://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid forward target: %q", s)
	}
	return u, nil
}
