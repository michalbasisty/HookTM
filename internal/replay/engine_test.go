package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hooktm/internal/store"
)

func TestReplayByID_Success(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	whID := "test-webhook-1"
	
	// Insert a test webhook
	err = s.InsertWebhook(ctx, store.InsertParams{
		ID:        whID,
		CreatedAt: time.Now().UnixMilli(),
		Method:    http.MethodPost,
		Path:      "/webhook",
		Query:     "token=abc",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
			"X-Custom":     {"value1", "value2"},
		},
		Body:       []byte(`{"message":"hello"}`),
		Provider:   "stripe",
		EventType:  "test.event",
		StatusCode: intPtr(200),
		ResponseMS: 100,
	})
	if err != nil {
		t.Fatalf("InsertWebhook: %v", err)
	}

	// Create a target server
	var receivedRequest *http.Request
	var receivedBody []byte
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r
		receivedBody, _ = io.ReadAll(r.Body)
		
		// Verify request details
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		if r.URL.Path != "/webhook" {
			t.Errorf("Expected path /webhook, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("token") != "abc" {
			t.Errorf("Expected query token=abc, got %s", r.URL.Query().Encode())
		}
		
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer target.Close()

	// Replay the webhook
	engine := NewEngine(s)
	result, err := engine.ReplayByID(ctx, whID, target.URL, "")
	if err != nil {
		t.Fatalf("ReplayByID: %v", err)
	}

	// Verify result
	if result.WebhookID != whID {
		t.Errorf("Expected WebhookID %s, got %s", whID, result.WebhookID)
	}
	if !result.Sent {
		t.Error("Expected Sent to be true")
	}
	if result.StatusCode != http.StatusAccepted {
		t.Errorf("Expected StatusCode %d, got %d", http.StatusAccepted, result.StatusCode)
	}
	if result.DurationMS < 0 {
		t.Error("Expected non-negative DurationMS")
	}

	// Verify forwarded request
	if receivedRequest == nil {
		t.Fatal("Target server did not receive request")
	}
	if string(receivedBody) != `{"message":"hello"}` {
		t.Errorf("Expected body, got %s", string(receivedBody))
	}
	if receivedRequest.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type header")
	}
}

func TestReplayByID_NotFound(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	engine := NewEngine(s)
	_, err := engine.ReplayByID(context.Background(), "nonexistent", "http://localhost:8080", "")
	
	if err == nil {
		t.Fatal("Expected error for non-existent webhook")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestReplayByID_InvalidTargetURL(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{}`),
	})

	engine := NewEngine(s)
	
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{"empty", "", "empty base url"},
		{"invalid", "://invalid", "invalid base url"},
		{"just host", "example.com", "invalid base url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.ReplayByID(ctx, "test", tt.target, "")
			if err == nil {
				t.Fatal("Expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got: %v", tt.want, err)
			}
		})
	}
}

func TestReplayByID_DryRun(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/webhook",
		Query:   "foo=bar",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{"test":"data"}`),
	})

	engine := NewEngine(s)
	engine.DryRun = true

	result, err := engine.ReplayByID(ctx, "test", "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("ReplayByID: %v", err)
	}

	if result.Sent {
		t.Error("Expected Sent to be false in dry-run mode")
	}
	if result.StatusCode != 0 {
		t.Errorf("Expected StatusCode 0 in dry-run, got %d", result.StatusCode)
	}
	if !strings.Contains(result.URL, "/webhook") {
		t.Errorf("Expected URL to contain path, got %s", result.URL)
	}
}

func TestReplayByID_WithJSONPatch(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{"amount": 100, "currency": "usd"}`),
	})

	var receivedBody []byte
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	patch := `{"amount": 500}`
	
	_, err := engine.ReplayByID(ctx, "test", target.URL, patch)
	if err != nil {
		t.Fatalf("ReplayByID: %v", err)
	}

	// Verify the patch was applied
	var result map[string]interface{}
	if err := json.Unmarshal(receivedBody, &result); err != nil {
		t.Fatalf("Failed to unmarshal body: %v", err)
	}
	if result["amount"] != float64(500) {
		t.Errorf("Expected amount=500, got %v", result["amount"])
	}
	if result["currency"] != "usd" {
		t.Errorf("Expected currency=usd, got %v", result["currency"])
	}
}

func TestReplayByID_NonJSONBodyWithPatch(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"text/plain"}},
		Body:    []byte(`plain text body`),
	})

	var receivedBody []byte
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	// Patch should be ignored for non-JSON body
	_, err := engine.ReplayByID(ctx, "test", target.URL, `{"foo":"bar"}`)
	if err != nil {
		t.Fatalf("ReplayByID: %v", err)
	}

	if string(receivedBody) != "plain text body" {
		t.Errorf("Expected unchanged body, got %s", string(receivedBody))
	}
}

func TestReplayByID_NetworkError(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{}`),
	})

	engine := NewEngine(s)
	// Use a port that's unlikely to be open
	_, err := engine.ReplayByID(ctx, "test", "http://localhost:1", "")
	
	if err == nil {
		t.Fatal("Expected error for connection refused")
	}
}

func TestReplayByID_PreservesAllHeaders(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:     "test",
		Method: http.MethodPost,
		Path:   "/",
		Headers: map[string][]string{
			"Authorization":   {"Bearer token123"},
			"X-Request-ID":    {"req-abc"},
			"X-Multi-Value":   {"val1", "val2", "val3"},
		},
		Body: []byte(`{}`),
	})

	receivedHeaders := make(http.Header)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range r.Header {
			receivedHeaders[k] = v
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	_, err := engine.ReplayByID(ctx, "test", target.URL, "")
	if err != nil {
		t.Fatalf("ReplayByID: %v", err)
	}

	// Check headers were preserved
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("Authorization header not preserved")
	}
	if receivedHeaders.Get("X-Request-Id") != "req-abc" {
		t.Errorf("X-Request-ID header not preserved (case-insensitive)")
	}
	
	// Check multi-value headers
	if len(receivedHeaders["X-Multi-Value"]) != 3 {
		t.Errorf("Expected 3 values for X-Multi-Value, got %d", len(receivedHeaders["X-Multi-Value"]))
	}
}

func TestReplayByID_PathJoining(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:     "test",
		Method: http.MethodPost,
		Path:   "/api/v1/webhooks",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:   []byte(`{}`),
	})

	receivedPaths := []string{}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPaths = append(receivedPaths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	
	// Test with various base URL formats
	bases := []string{
		target.URL,              // http://host:port
		target.URL + "/",        // http://host:port/
		target.URL + "/base",    // http://host:port/base
		target.URL + "/base/",   // http://host:port/base/
	}
	
	for _, base := range bases {
		_, err := engine.ReplayByID(ctx, "test", base, "")
		if err != nil {
			t.Fatalf("ReplayByID with base %s: %v", base, err)
		}
	}

	// Verify paths are correctly joined
	expectedPaths := []string{
		"/api/v1/webhooks",
		"/api/v1/webhooks",
		"/base/api/v1/webhooks",
		"/base/api/v1/webhooks",
	}
	
	for i, expected := range expectedPaths {
		if receivedPaths[i] != expected {
			t.Errorf("Base %q: expected path %q, got %q", bases[i], expected, receivedPaths[i])
		}
	}
}

func TestReplayByID_DifferentMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			s, _ := store.Open(":memory:")
			defer s.Close()
			
			ctx := context.Background()
			s.InsertWebhook(ctx, store.InsertParams{
				ID:      method + "-test",
				Method:  method,
				Path:    "/",
				Headers: map[string][]string{"Content-Type": {"application/json"}},
				Body:    []byte(`{}`),
			})
			
			var receivedMethod string
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer target.Close()
			
			engine := NewEngine(s)
			_, err := engine.ReplayByID(ctx, method+"-test", target.URL, "")
			if err != nil {
				t.Fatalf("ReplayByID: %v", err)
			}
			
			if receivedMethod != method {
				t.Errorf("Expected method %s, got %s", method, receivedMethod)
			}
		})
	}
}

func TestReplayByID_QueryStringHandling(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodGet,
		Path:    "/api",
		Query:   "foo=bar&baz=qux",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{}`),
	})

	var receivedQuery string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	_, err := engine.ReplayByID(ctx, "test", target.URL, "")
	if err != nil {
		t.Fatalf("ReplayByID: %v", err)
	}

	if receivedQuery != "foo=bar&baz=qux" {
		t.Errorf("Expected query 'foo=bar&baz=qux', got %q", receivedQuery)
	}
}

func TestApplyMergePatchIfJSON(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string][]string
		body     []byte
		patch    []byte
		expected []byte
		wantErr  bool
	}{
		{
			name:     "valid JSON patch",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     []byte(`{"name":"John","age":30}`),
			patch:    []byte(`{"age":31}`),
			expected: []byte(`{"name":"John","age":31}`),
			wantErr:  false,
		},
		{
			name:     "patch adds new field",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     []byte(`{"name":"John"}`),
			patch:    []byte(`{"age":30}`),
			expected: []byte(`{"name":"John","age":30}`),
			wantErr:  false,
		},
		{
			name:     "non-JSON content type",
			headers:  map[string][]string{"Content-Type": {"text/plain"}},
			body:     []byte(`plain text`),
			patch:    []byte(`{"foo":"bar"}`),
			expected: []byte(`plain text`),
			wantErr:  false,
		},
		{
			name:     "no content-type but looks like JSON",
			headers:  map[string][]string{},
			body:     []byte(`{"key":"value"}`),
			patch:    []byte(`{"key":"newvalue"}`),
			expected: []byte(`{"key":"newvalue"}`),
			wantErr:  false,
		},
		{
			name:     "empty body with patch",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     []byte{},
			patch:    []byte(`{"key":"value"}`),
			expected: []byte(`{"key":"value"}`),
			wantErr:  false,
		},
		{
			name:     "nil body with patch",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     nil,
			patch:    []byte(`{"key":"value"}`),
			expected: []byte(`{"key":"value"}`),
			wantErr:  false,
		},
		{
			name:     "invalid JSON body",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     []byte(`not json`),
			patch:    []byte(`{"key":"value"}`),
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "invalid patch",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     []byte(`{"key":"value"}`),
			patch:    []byte(`not a valid patch`),
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "array body - RFC 7396 merge patch behavior",
			headers:  map[string][]string{"Content-Type": {"application/json"}},
			body:     []byte(`[1,2,3]`),
			patch:    []byte(`{"0":99}`),
			expected: []byte(`{"0":99}`), // Arrays are replaced, not merged per RFC 7396
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyMergePatchIfJSON(tt.headers, tt.body, tt.patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyMergePatchIfJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.expected != nil && string(got) != string(tt.expected) {
				t.Errorf("applyMergePatchIfJSON() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestFirstHeader(t *testing.T) {
	tests := []struct {
		name string
		h    map[string][]string
		key  string
		want string
	}{
		{
			name: "exact match",
			h:    map[string][]string{"Content-Type": {"application/json"}},
			key:  "Content-Type",
			want: "application/json",
		},
		{
			name: "case insensitive",
			h:    map[string][]string{"content-type": {"text/plain"}},
			key:  "Content-Type",
			want: "text/plain",
		},
		{
			name: "not found",
			h:    map[string][]string{"Other": {"value"}},
			key:  "Content-Type",
			want: "",
		},
		{
			name: "first value only",
			h:    map[string][]string{"X-Multi": {"first", "second", "third"}},
			key:  "X-Multi",
			want: "first",
		},
		{
			name: "empty header map",
			h:    map[string][]string{},
			key:  "Anything",
			want: "",
		},
		{
			name: "empty values",
			h:    map[string][]string{"Empty": {}},
			key:  "Empty",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstHeader(tt.h, tt.key)
			if got != tt.want {
				t.Errorf("firstHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLooksLikeJSON(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		want bool
	}{
		{"empty", []byte{}, false},
		{"object", []byte(`{"key":"value"}`), true},
		{"array", []byte(`[1,2,3]`), true},
		{"object with space", []byte(`  {"key":"value"}`), true},
		{"array with space", []byte(`  [1,2,3]`), true},
		{"plain text", []byte(`hello`), false},
		{"xml", []byte(`<root></root>`), false},
		{"number", []byte(`123`), false},
		{"null", []byte(`null`), false},
		{"true", []byte(`true`), false},
		{"false", []byte(`false`), false},
		{"string in quotes", []byte(`"hello"`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeJSON(tt.b)
			if got != tt.want {
				t.Errorf("looksLikeJSON(%q) = %v, want %v", tt.b, got, tt.want)
			}
		})
	}
}

func TestReplayByID_InvalidPatch(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{"valid":"json"}`),
	})

	engine := NewEngine(s)
	_, err := engine.ReplayByID(ctx, "test", "http://localhost:8080", `not valid json patch`)
	
	if err == nil {
		t.Fatal("Expected error for invalid patch")
	}
}

func TestReplayByID_CustomHTTPClient(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()

	ctx := context.Background()
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "test",
		Method:  http.MethodPost,
		Path:    "/",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{}`),
	})

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	// Custom client with short timeout
	engine.HTTP = &http.Client{Timeout: 5 * time.Second}
	
	_, err := engine.ReplayByID(ctx, "test", target.URL, "")
	if err != nil {
		t.Fatalf("ReplayByID with custom client: %v", err)
	}
}

// Helper function
func intPtr(v int) *int {
	return &v
}

// Helper for error string formatting
func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Ensure errStr is used (avoid unused function warning)
var _ = errStr(fmt.Errorf("test"))
