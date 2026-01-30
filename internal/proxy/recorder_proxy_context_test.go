package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hooktm/internal/store"
)

func TestRecorderProxy_ContextCancellation(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	proxy := NewRecorderProxy(nil, s, nil)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create request with cancellable context
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"test":"data"}`))
	req = req.WithContext(ctx)

	// Cancel context before sending request
	cancel()

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Should return 503 Service Unavailable when context is cancelled
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for cancelled context, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	// Verify no webhook was stored
	storedCtx := context.Background()
	rows, _ := s.ListSummaries(storedCtx, store.ListFilter{Limit: 10})
	if len(rows) != 0 {
		t.Errorf("Expected 0 webhooks for cancelled request, got %d", len(rows))
	}
}

func TestRecorderProxy_ContextTimeout(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	proxy := NewRecorderProxy(nil, s, nil)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for timeout
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"test":"data"}`))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Should return 503 when context deadline exceeded
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d for timed out context, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestRecorderProxy_ContextPropagation(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	proxy := NewRecorderProxy(nil, s, nil)

	// Create request with normal context
	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"test":"data"}`))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	// Should succeed
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Verify webhook was stored
	storedCtx := context.Background()
	rows, _ := s.ListSummaries(storedCtx, store.ListFilter{Limit: 10})
	if len(rows) != 1 {
		t.Errorf("Expected 1 webhook, got %d", len(rows))
	}
}
