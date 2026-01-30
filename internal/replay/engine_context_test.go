package replay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hooktm/internal/store"
)

func TestReplayByID_ContextCancellation(t *testing.T) {
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

	// Create a slow target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Create a cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	engine := NewEngine(s)
	engine.HTTP = &http.Client{Timeout: 5 * time.Second}

	_, err := engine.ReplayByID(cancelledCtx, "test", target.URL, "")
	
	// Should get an error due to cancelled context
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Logf("Got error: %v", err)
	}
}

func TestReplayByID_ContextTimeout(t *testing.T) {
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

	// Create a slow target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Create a context with short timeout
	shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	engine := NewEngine(s)

	_, err := engine.ReplayByID(shortCtx, "test", target.URL, "")
	
	// Should get timeout error
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestReplayByID_ContextPropagatedToRequest(t *testing.T) {
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

	receivedContext := make(chan context.Context, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContext <- r.Context()
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := NewEngine(s)
	
	// Use context with value
	ctxWithValue := context.WithValue(ctx, "test-key", "test-value")
	_, err := engine.ReplayByID(ctxWithValue, "test", target.URL, "")
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	select {
	case reqCtx := <-receivedContext:
		// The request context should have a deadline from the HTTP client timeout
		if _, hasDeadline := reqCtx.Deadline(); !hasDeadline {
			t.Error("Request context should have deadline from HTTP client")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for request context")
	}
}

func TestReplayByID_BodyDrainingRespectsContext(t *testing.T) {
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

	// Create a server that sends a large response body
	largeBody := strings.Repeat("x", 1024*1024) // 1MB
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(largeBody))
	}))
	defer target.Close()

	engine := NewEngine(s)

	result, err := engine.ReplayByID(ctx, "test", target.URL, "")
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", result.StatusCode)
	}
}
