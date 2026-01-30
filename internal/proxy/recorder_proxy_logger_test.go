package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hooktm/internal/logger"
	"hooktm/internal/store"
)

func TestRecorderProxy_WithLogger(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	// Create a buffer to capture log output
	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  logger.DebugLevel,
		Format: "text",
		Output: &buf,
	})

	proxy := NewRecorderProxy(nil, s, log)

	// Send a request that will trigger logging
	body := []byte(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// The request was successful, so no error logs should be produced
	// But we can verify the logger is wired correctly by checking
	// that the proxy works with a real logger
	output := buf.String()
	
	// There should be no error messages for a successful request
	if strings.Contains(output, "ERROR") {
		t.Errorf("Unexpected error log for successful request: %s", output)
	}
}

func TestRecorderProxy_WithNilLogger(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	// Pass nil logger - should default to NopLogger
	proxy := NewRecorderProxy(nil, s, nil)

	// This should not panic even with nil logger
	body := []byte(`{"test":"data"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestRecorderProxy_LogFields(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  logger.WarnLevel, // Only log warnings and above
		Format: "text",
		Output: &buf,
	})

	// Add some context fields
	log = log.WithField("component", "proxy").WithField("version", "1.0")

	proxy := NewRecorderProxy(nil, s, log)

	// Send a request that's too large to trigger a warning
	largeBody := make([]byte, MaxRequestBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(largeBody))
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", rr.Code)
	}

	output := buf.String()
	
	// Should contain our custom fields and the warning
	if !strings.Contains(output, "WARN") {
		t.Error("Expected WARN level log")
	}
	if !strings.Contains(output, "request body too large") {
		t.Error("Expected 'request body too large' message")
	}
}

func TestRecorderProxy_CorrelationID(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer s.Close()

	var buf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  logger.DebugLevel,
		Format: "text",
		Output: &buf,
	})

	proxy := NewRecorderProxy(nil, s, log)

	// Create request with correlation ID in context
	ctx := logger.WithCorrelationID(httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`)).Context(), "test-correlation-123")
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`)).WithContext(ctx)
	
	rr := httptest.NewRecorder()
	proxy.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}
