package replay

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"

	"hooktm/internal/store"
	"hooktm/internal/urlutil"
)

type Engine struct {
	store *store.Store
	HTTP  *http.Client

	DryRun bool
}

type Result struct {
	WebhookID  string `json:"webhook_id"`
	URL        string `json:"url"`
	Sent       bool   `json:"sent"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

func NewEngine(s *store.Store) *Engine {
	return &Engine{
		store: s,
		HTTP:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (e *Engine) ReplayByID(ctx context.Context, id string, targetBase string, mergePatch string) (Result, error) {
	wh, err := e.store.GetWebhook(ctx, id)
	if err != nil {
		return Result{}, err
	}
	base, err := parseBaseURL(targetBase)
	if err != nil {
		return Result{}, err
	}

	body := wh.Body
	if strings.TrimSpace(mergePatch) != "" {
		body, err = applyMergePatchIfJSON(wh.Headers, body, []byte(mergePatch))
		if err != nil {
			return Result{}, err
		}
	}

	u := *base
	u.Path = urlutil.SingleJoiningSlash(u.Path, wh.Path)
	u.RawQuery = strings.TrimPrefix(wh.Query, "?")

	if e.DryRun {
		return Result{WebhookID: id, URL: u.String(), Sent: false}, nil
	}

	req, err := http.NewRequestWithContext(ctx, wh.Method, u.String(), bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	for k, vs := range wh.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	start := time.Now()
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return Result{}, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	return Result{
		WebhookID:  id,
		URL:        u.String(),
		Sent:       true,
		StatusCode: resp.StatusCode,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

func parseBaseURL(s string) (*url.URL, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty base url")
	}
	if !strings.Contains(s, "://") && strings.Contains(s, ":") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid base url: %q", s)
	}
	return u, nil
}

func applyMergePatchIfJSON(headers map[string][]string, body []byte, patch []byte) ([]byte, error) {
	ct := strings.ToLower(firstHeader(headers, "Content-Type"))
	if !strings.Contains(ct, "application/json") && !looksLikeJSON(body) {
		return body, nil
	}
	if len(body) == 0 {
		body = []byte(`{}`)
	}
	return jsonpatch.MergePatch(body, patch)
}

func firstHeader(h map[string][]string, k string) string {
	for hk, vs := range h {
		if strings.EqualFold(hk, k) && len(vs) > 0 {
			return vs[0]
		}
	}
	return ""
}

func looksLikeJSON(b []byte) bool {
	b = bytes.TrimSpace(b)
	return len(b) > 0 && (b[0] == '{' || b[0] == '[')
}

