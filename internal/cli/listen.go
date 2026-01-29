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
		Usage:     "Start proxy and capture webhooks",
		ArgsUsage: "<port>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "forward",
				Usage: "Forward target (host:port or URL); omit to record-only",
			},
		},
		Action: func(c *cli.Context) error {
			portStr, err := requireArg(c, 0, "port")
			if err != nil {
				return err
			}
			portStr = strings.TrimSpace(portStr)

			s, cfg, err := openStoreFromContext(c)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := ensureDirForFile(s.Path()); err != nil {
				return err
			}

			forward := c.String("forward")
			if forward == "" {
				forward = cfg.Forward
			}
			var targetURL *url.URL
			if forward != "" {
				var err error
				targetURL, err = parseForwardTarget(forward)
				if err != nil {
					return err
				}
			}

			addr := net.JoinHostPort("", portStr)
			srv := &http.Server{
				Addr:              addr,
				Handler:           proxy.NewRecorderProxy(targetURL, s),
				ReadHeaderTimeout: 10 * time.Second,
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if targetURL != nil {
				_, _ = fmt.Fprintf(c.App.Writer, "Listening on :%s → %s\n", portStr, targetURL.String())
			} else {
				_, _ = fmt.Fprintf(c.App.Writer, "Listening on :%s (record-only mode)\n", portStr)
			}
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
		},
	}
}

func parseForwardTarget(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty forward target")
	}
	// Allow host:port shorthand.
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
