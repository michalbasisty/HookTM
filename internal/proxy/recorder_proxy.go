package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hooktm/internal/logger"
	"hooktm/internal/provider"
	"hooktm/internal/store"
	"hooktm/internal/urlutil"

	nanoid "github.com/matoous/go-nanoid/v2"
)

const (
	// MaxRequestBodySize limits incoming request bodies to prevent memory exhaustion.
	MaxRequestBodySize = 10 * 1024 * 1024 // 10 MB
	// MaxBodyTextLength is the maximum length of body text to index for search.
	MaxBodyTextLength = 200_000 // 200 KB
)

type RecorderProxy struct {
	target *url.URL
	store  *store.Store
	client *http.Client
	log    logger.Logger
}

func NewRecorderProxy(target *url.URL, s *store.Store, log logger.Logger, nil) *RecorderProxy {
	if log == nil {
		log = logger.NopLogger{}
	}
	return &RecorderProxy{
		target: target,
		store:  s,
		client: &http.Client{Timeout: 60 * time.Second},
		log:    log,
	}
}

func (p *RecorderProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now()

	// Check for early cancellation before processing.
	if err := ctx.Err(); err != nil {
		http.Error(w, "request cancelled", http.StatusServiceUnavailable)
		return
	}

	// Limit request body size to prevent memory exhaustion.
	limitedReader := io.LimitReader(r.Body, MaxRequestBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		p.log.WithContext(ctx).Errorf("failed to read request body: %v", err)
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	if err := r.Body.Close(); err != nil {
		p.log.WithContext(ctx).Warnf("failed to close request body: %v", err)
	}

	// Check if body exceeded limit.
	if len(body) > MaxRequestBodySize {
		p.log.WithContext(ctx).Warnf("request body too large: %d bytes", len(body))
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	id, err := nanoid.New()
	if err != nil {
		p.log.WithContext(ctx).Errorf("failed to generate ID: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	prov, eventType, sig := provider.Detect(r.Header, body)
	bodyText := extractBodyText(r.Header.Get("Content-Type"), body)

	var statusCode *int
	var respMS int64

	if p.target != nil {
		statusCode, respMS, err = p.forward(w, r, body)
		if err != nil {
			p.log.WithContext(ctx).WithFields(logger.Fields{
			"error": err.Error(),
			"target": p.target.String(),
		}).Warn("forward failed")
			http.Error(w, err.Error(), http.StatusBadGateway)
			sc := http.StatusBadGateway
			statusCode = &sc
			respMS = time.Since(now).Milliseconds()
		}
	} else {
		// Record-only mode: return 200 OK without forwarding.
		w.WriteHeader(http.StatusOK)
		sc := http.StatusOK
		statusCode = &sc
		respMS = time.Since(now).Milliseconds()
	}

	if err := p.store.InsertWebhook(ctx, store.InsertParams{
		ID:         id,
		CreatedAt:  now.UnixMilli(),
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		Headers:    cloneHeader(r.Header),
		Body:       body,
		Provider:   prov,
		EventType:  eventType,
		Signature:  sig,
		StatusCode: statusCode,
		ResponseMS: respMS,
		BodyText:   bodyText,
	}); err != nil {
		p.log.WithContext(ctx).Errorf("failed to store webhook: %v", err)
	}
}

func (p *RecorderProxy) forward(w http.ResponseWriter, r *http.Request, body []byte) (*int, int64, error) {
	start := time.Now()

	outURL := *p.target
	outURL.Path = urlutil.SingleJoiningSlash(outURL.Path, r.URL.Path)
	outURL.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(r.Context(), r.Method, outURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	// Copy headers (excluding hop-by-hop).
	req.Header = make(http.Header, len(r.Header))
	for k, vs := range r.Header {
		if isHopByHopHeader(k) {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	// Preserve Host for apps that rely on it.
	req.Host = p.target.Host

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	for k, vs := range resp.Header {
		if isHopByHopHeader(k) {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)

	ms := time.Since(start).Milliseconds()
	sc := resp.StatusCode
	return &sc, ms, nil
}

func cloneHeader(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, vs := range h {
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}

func isHopByHopHeader(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func extractBodyText(contentType string, body []byte) string {
	if len(body) == 0 {
		return ""
	}
	if len(body) > MaxBodyTextLength {
		// cap for MVP; keep DB usable.
		body = body[:MaxBodyTextLength]
	}
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "application/json"),
		strings.Contains(ct, "application/xml"),
		strings.Contains(ct, "text/"),
		strings.Contains(ct, "application/x-www-form-urlencoded"):
		return string(body)
	default:
		return ""
	}
}
