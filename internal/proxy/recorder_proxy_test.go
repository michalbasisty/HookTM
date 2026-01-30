package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"hooktm/internal/store"
)

func TestRecorderProxy_RecordOnly(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	proxy := NewRecorderProxy(nil, s)

	// Create test request
	body := []byte(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "custom-value")

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Verify webhook was stored
	ctx := context.Background()
	rows, err := s.ListSummaries(ctx, store.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 webhook, got %d", len(rows))
	}

	wh, err := s.GetWebhook(ctx, rows[0].ID)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}

	if wh.Method != http.MethodPost {
		t.Errorf("Expected method POST, got %s", wh.Method)
	}
	if wh.Path != "/webhooks/test" {
		t.Errorf("Expected path /webhooks/test, got %s", wh.Path)
	}
	if string(wh.Body) != string(body) {
		t.Errorf("Expected body %s, got %s", body, wh.Body)
	}
}

func TestRecorderProxy_WithForward(t *testing.T) {
	// Create a target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request was forwarded correctly
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/webhooks/test" {
			t.Errorf("Expected path /webhooks/test, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type header")
		}

		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"test":"data"}` {
			t.Errorf("Expected body, got %s", body)
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer target.Close()

	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	targetURL, _ := url.Parse(target.URL)
	proxy := NewRecorderProxy(targetURL, s)

	body := []byte(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Check response from target was returned
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}
	if rr.Body.String() != `{"status":"ok"}` {
		t.Errorf("Expected body from target, got %s", rr.Body.String())
	}

	// Verify webhook was stored
	ctx := context.Background()
	rows, err := s.ListSummaries(ctx, store.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 webhook, got %d", len(rows))
	}
	if rows[0].StatusCode == nil || *rows[0].StatusCode != http.StatusCreated {
		t.Errorf("Expected status code 201, got %v", rows[0].StatusCode)
	}
}

func TestRecorderProxy_BodyTooLarge(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	proxy := NewRecorderProxy(nil, s)

	// Create a body larger than MaxRequestBodySize
	largeBody := make([]byte, MaxRequestBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader(largeBody))

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", rr.Code)
	}

	// Verify no webhook was stored
	ctx := context.Background()
	rows, err := s.ListSummaries(ctx, store.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Expected 0 webhooks, got %d", len(rows))
	}
}

func TestRecorderProxy_ForwardError(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	// Use a URL that will cause connection refused
	targetURL, _ := url.Parse("http://localhost:1")
	proxy := NewRecorderProxy(targetURL, s)

	body := []byte(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/test", bytes.NewReader(body))

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Should return 502 Bad Gateway
	if rr.Code != http.StatusBadGateway {
		t.Errorf("Expected status 502, got %d", rr.Code)
	}

	// But webhook should still be recorded
	ctx := context.Background()
	rows, err := s.ListSummaries(ctx, store.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("Expected 1 webhook even on forward error, got %d", len(rows))
	}
	if rows[0].StatusCode == nil || *rows[0].StatusCode != http.StatusBadGateway {
		t.Errorf("Expected status code 502, got %v", rows[0].StatusCode)
	}
}

func TestRecorderProxy_ProviderDetection(t *testing.T) {
	tests := []struct {
		name         string
		headers      map[string]string
		body         string
		wantProvider string
		wantEvent    string
	}{
		{
			name:         "Stripe webhook",
			headers:      map[string]string{"Stripe-Signature": "t=123,v1=abc"},
			body:         `{"type":"payment_intent.succeeded"}`,
			wantProvider: "stripe",
			wantEvent:    "payment_intent.succeeded",
		},
		{
			name:         "GitHub webhook",
			headers:      map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=abc"},
			body:         `{"ref":"refs/heads/main"}`,
			wantProvider: "github",
			wantEvent:    "push",
		},
		{
			name:         "Unknown provider",
			headers:      map[string]string{"Content-Type": "application/json"},
			body:         `{"test":"data"}`,
			wantProvider: "unknown",
			wantEvent:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.Open(":memory:")
			if err != nil {
				t.Fatalf("Open store: %v", err)
			}
			defer s.Close()

			proxy := NewRecorderProxy(nil, s)

			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(tt.body))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rr := httptest.NewRecorder()
			proxy.ServeHTTP(rr, req)

			ctx := context.Background()
			rows, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
			if len(rows) != 1 {
				t.Fatalf("Expected 1 webhook, got %d", len(rows))
			}

			wh, _ := s.GetWebhook(ctx, rows[0].ID)
			if wh.Provider != tt.wantProvider {
				t.Errorf("Expected provider %q, got %q", tt.wantProvider, wh.Provider)
			}
			if wh.EventType != tt.wantEvent {
				t.Errorf("Expected event %q, got %q", tt.wantEvent, wh.EventType)
			}
		})
	}
}

func TestRecorderProxy_HopByHopHeaders(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hop-by-hop headers should NOT be forwarded
		if r.Header.Get("Connection") != "" {
			t.Errorf("Connection header should not be forwarded")
		}
		if r.Header.Get("Upgrade") != "" {
			t.Errorf("Upgrade header should not be forwarded")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	s, _ := store.Open(":memory:")
	defer s.Close()

	targetURL, _ := url.Parse(target.URL)
	proxy := NewRecorderProxy(targetURL, s)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`))
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestCloneHeader(t *testing.T) {
	original := http.Header{
		"Content-Type": {"application/json"},
		"X-Multi":      {"value1", "value2"},
	}

	cloned := cloneHeader(original)

	// Verify clone has same values
	if len(cloned) != len(original) {
		t.Errorf("Expected %d headers, got %d", len(original), len(cloned))
	}

	// Verify modification doesn't affect original
	cloned["Content-Type"][0] = "text/plain"
	if original.Get("Content-Type") != "application/json" {
		t.Error("Modifying clone affected original")
	}

	// Verify slices are independent
	cloned["X-Multi"] = append(cloned["X-Multi"], "value3")
	if len(original["X-Multi"]) != 2 {
		t.Error("Appending to clone affected original")
	}
}

func TestIsHopByHopHeader(t *testing.T) {
	tests := []struct {
		header string
		want   bool
	}{
		{"Connection", true},
		{"connection", true}, // case insensitive
		{"CONNECTION", true},
		{"Upgrade", true},
		{"Proxy-Authorization", true},
		{"Content-Type", false},
		{"X-Custom-Header", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := isHopByHopHeader(tt.header)
			if got != tt.want {
				t.Errorf("isHopByHopHeader(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}

func TestExtractBodyText(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		want        string
	}{
		{
			name:        "JSON body",
			contentType: "application/json",
			body:        []byte(`{"key":"value"}`),
			want:        `{"key":"value"}`,
		},
		{
			name:        "Empty body",
			contentType: "application/json",
			body:        []byte{},
			want:        "",
		},
		{
			name:        "XML body",
			contentType: "application/xml",
			body:        []byte(`<root><item>value</item></root>`),
			want:        "<root><item>value</item></root>",
		},
		{
			name:        "Plain text",
			contentType: "text/plain",
			body:        []byte("hello world"),
			want:        "hello world",
		},
		{
			name:        "Binary content",
			contentType: "application/octet-stream",
			body:        []byte{0x00, 0x01, 0x02},
			want:        "",
		},
		{
			name:        "Large body is truncated",
			contentType: "application/json",
			body:        []byte(strings.Repeat("a", MaxBodyTextLength+100)),
			want:        strings.Repeat("a", MaxBodyTextLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBodyText(tt.contentType, tt.body)
			if got != tt.want {
				t.Errorf("extractBodyText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecorderProxy_ConcurrentRequests(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	proxy := NewRecorderProxy(nil, s)

	// Send multiple concurrent requests
	const numRequests = 10
	done := make(chan bool, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(idx int) {
			body := []byte(fmt.Sprintf(`{"idx":%d}`, idx))
			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
			rr := httptest.NewRecorder()
			proxy.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Request %d failed with status %d", idx, rr.Code)
			}
			done <- true
		}(i)
	}

	// Wait for all requests
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify all webhooks were stored
	ctx := context.Background()
	rows, err := s.ListSummaries(ctx, store.ListFilter{Limit: numRequests * 2})
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(rows) != numRequests {
		t.Errorf("Expected %d webhooks, got %d", numRequests, len(rows))
	}
}


