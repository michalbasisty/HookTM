package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"hooktm/internal/proxy"
	"hooktm/internal/urlutil"

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

	if err := validatePort(port); err != nil {
		return err
	}

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
	u, err := urlutil.ParseURL(s)
	if err != nil {
		if err.Error() == "empty URL" {
			return nil, fmt.Errorf("empty forward target")
		}
		return nil, fmt.Errorf("invalid forward target: %w", err)
	}
	return u, nil
}

// validatePort validates that the port is a valid port number (1-65535).
func validatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port %q: must be a number", port)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", n)
	}
	return nil
}
