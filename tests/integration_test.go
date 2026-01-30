package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"hooktm/internal/proxy"
	"hooktm/internal/replay"
	"hooktm/internal/store"
)

// TestFullFlow_CaptureAndReplay tests the complete webhook lifecycle:
// 1. Webhook arrives at proxy
// 2. Proxy stores webhook in database
// 3. User replays webhook to target
// 4. Target receives correct request
func TestFullFlow_CaptureAndReplay(t *testing.T) {
	ctx := context.Background()

	// Setup: Create in-memory database
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	// Setup: Create target server that will receive the replay
	var replayedRequest *http.Request
	var replayedBody []byte
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		replayedRequest = r
		replayedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	}))
	defer targetServer.Close()

	// Step 1: Create proxy (record-only mode for simplicity)
	recorder := proxy.NewRecorderProxy(nil, s)

	// Step 2: Send webhook to proxy
	webhookBody := []byte(`{"event":"payment.succeeded","amount":5000,"currency":"usd"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", "t=1234567890,v1=abc123")

	rr := httptest.NewRecorder()
	recorder.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Proxy returned status %d, expected 200", rr.Code)
	}

	// Step 3: Verify webhook was stored
	summaries, err := s.ListSummaries(ctx, store.ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to list webhooks: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("Expected 1 webhook in store, got %d", len(summaries))
	}

	storedWebhookID := summaries[0].ID

	// Verify stored details
	wh, err := s.GetWebhook(ctx, storedWebhookID)
	if err != nil {
		t.Fatalf("Failed to get webhook: %v", err)
	}

	if wh.Method != http.MethodPost {
		t.Errorf("Expected method POST, got %s", wh.Method)
	}
	if wh.Path != "/webhooks/stripe" {
		t.Errorf("Expected path /webhooks/stripe, got %s", wh.Path)
	}
	if wh.Provider != "stripe" {
		t.Errorf("Expected provider stripe, got %s", wh.Provider)
	}
	if string(wh.Body) != string(webhookBody) {
		t.Errorf("Body mismatch: expected %s, got %s", webhookBody, wh.Body)
	}

	// Step 4: Replay the webhook to target server
	engine := replay.NewEngine(s)
	result, err := engine.ReplayByID(ctx, storedWebhookID, targetServer.URL, "")
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if !result.Sent {
		t.Error("Expected Sent=true in replay result")
	}
	if result.StatusCode != http.StatusAccepted {
		t.Errorf("Expected status %d, got %d", http.StatusAccepted, result.StatusCode)
	}

	// Step 5: Verify target received the replayed webhook
	if replayedRequest == nil {
		t.Fatal("Target server did not receive replayed request")
	}

	if replayedRequest.Method != http.MethodPost {
		t.Errorf("Replayed method: expected POST, got %s", replayedRequest.Method)
	}
	if replayedRequest.URL.Path != "/webhooks/stripe" {
		t.Errorf("Replayed path: expected /webhooks/stripe, got %s", replayedRequest.URL.Path)
	}
	if string(replayedBody) != string(webhookBody) {
		t.Errorf("Replayed body mismatch: expected %s, got %s", webhookBody, replayedBody)
	}
	if replayedRequest.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type header not preserved in replay")
	}
	if replayedRequest.Header.Get("Stripe-Signature") != "t=1234567890,v1=abc123" {
		t.Error("Stripe-Signature header not preserved in replay")
	}

	t.Logf("✓ Full flow completed: captured webhook %s and replayed successfully", storedWebhookID)
}

// TestFullFlow_CaptureWithForward tests proxy in forward mode:
// 1. Webhook arrives at proxy
// 2. Proxy forwards to target app
// 3. Proxy stores webhook
// 4. Response from target is returned to original sender
func TestFullFlow_CaptureWithForward(t *testing.T) {
	ctx := context.Background()

	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	// Create the "user application" that receives forwarded webhooks
	var forwardedRequest *http.Request
	var forwardedBody []byte
	userApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedRequest = r
		forwardedBody, _ = io.ReadAll(r.Body)
		
		// Simulate app processing
		w.Header().Set("X-Processed-By", "user-app")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"processed":true}`))
	}))
	defer userApp.Close()

	// Create proxy with forward target
	targetURL, _ := parseURL(userApp.URL)
	recorder := proxy.NewRecorderProxy(targetURL, s)

	// Send webhook to proxy
	webhookBody := []byte(`{"event":"user.created","user_id":"usr_123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "member_added")

	rr := httptest.NewRecorder()
	recorder.ServeHTTP(rr, req)

	// Verify response from user app was returned
	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201 from user app, got %d", rr.Code)
	}
	if rr.Header().Get("X-Processed-By") != "user-app" {
		t.Error("Response headers from user app not preserved")
	}
	if !strings.Contains(rr.Body.String(), `"processed":true`) {
		t.Error("Response body from user app not preserved")
	}

	// Verify webhook was stored with correct status
	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
	if len(summaries) != 1 {
		t.Fatal("Webhook not stored")
	}

	if summaries[0].StatusCode == nil || *summaries[0].StatusCode != http.StatusCreated {
		t.Error("Stored webhook doesn't have correct status code")
	}

	// Verify user app received the forwarded request
	if forwardedRequest == nil {
		t.Fatal("User app did not receive forwarded request")
	}
	if string(forwardedBody) != string(webhookBody) {
		t.Error("Forwarded body doesn't match original")
	}

	t.Log("✓ Forward mode flow completed successfully")
}

// TestFullFlow_CaptureSearchReplay tests the search and replay workflow:
// 1. Capture multiple webhooks
// 2. Search for specific webhook
// 3. Replay found webhook
func TestFullFlow_CaptureSearchReplay(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture multiple webhooks
	webhooks := []struct {
		path string
		body string
		text string
	}{
		{"/webhooks", `{"event":"payment","amount":100}`, "payment 100"},
		{"/webhooks", `{"event":"refund","amount":50}`, "refund 50"},
		{"/webhooks", `{"event":"payment","amount":200}`, "payment 200"},
	}

	for _, wh := range webhooks {
		req := httptest.NewRequest(http.MethodPost, wh.path, strings.NewReader(wh.body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		recorder.ServeHTTP(rr, req)
	}

	// Search for payment webhooks
	results, err := s.SearchSummaries(ctx, "payment", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 payment webhooks, got %d", len(results))
	}

	// Create target for replay
	var receivedCount int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Replay all found webhooks
	engine := replay.NewEngine(s)
	for _, result := range results {
		_, err := engine.ReplayByID(ctx, result.ID, target.URL, "")
		if err != nil {
			t.Errorf("Failed to replay %s: %v", result.ID, err)
		}
	}

	if receivedCount != 2 {
		t.Errorf("Expected 2 replays, got %d", receivedCount)
	}

	t.Log("✓ Search and replay flow completed successfully")
}

// TestFullFlow_ReplayWithModification tests capturing and replaying with JSON patch:
// 1. Capture webhook
// 2. Replay with modification (JSON patch)
// 3. Verify modified body received
func TestFullFlow_ReplayWithModification(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture webhook
	originalBody := `{"amount":100,"currency":"usd","test_mode":true}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(originalBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	recorder.ServeHTTP(rr, req)

	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
	whID := summaries[0].ID

	// Create target that verifies modified body
	var receivedBody map[string]interface{}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Replay with patch to change amount and remove test_mode
	engine := replay.NewEngine(s)
	patch := `{"amount":500,"test_mode":null}`
	_, err := engine.ReplayByID(ctx, whID, target.URL, patch)
	if err != nil {
		t.Fatalf("Replay with patch failed: %v", err)
	}

	// Verify modifications were applied
	if receivedBody["amount"] != float64(500) {
		t.Errorf("Expected amount=500, got %v", receivedBody["amount"])
	}
	if receivedBody["currency"] != "usd" {
		t.Errorf("Expected currency=usd preserved, got %v", receivedBody["currency"])
	}
	// RFC 7396: null means delete
	if _, exists := receivedBody["test_mode"]; exists {
		t.Error("test_mode should have been removed by patch")
	}

	t.Log("✓ Replay with modification completed successfully")
}

// TestFullFlow_ConcurrentWebhooks tests thread safety:
// 1. Send multiple webhooks concurrently
// 2. Verify all are stored correctly
// 3. Replay all concurrently
func TestFullFlow_ConcurrentWebhooks(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	const numWebhooks = 50
	var wg sync.WaitGroup
	wg.Add(numWebhooks)

	// Capture webhooks concurrently
	for i := 0; i < numWebhooks; i++ {
		go func(idx int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"index":%d,"timestamp":%d}`, idx, time.Now().UnixNano())
			req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			recorder.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Webhook %d failed with status %d", idx, rr.Code)
			}
		}(i)
	}
	wg.Wait()

	// Verify all stored
	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: numWebhooks * 2})
	if len(summaries) != numWebhooks {
		t.Errorf("Expected %d webhooks stored, got %d", numWebhooks, len(summaries))
	}

	// Create target for concurrent replays
	var replayCount int
	var replayMutex sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		replayMutex.Lock()
		replayCount++
		replayMutex.Unlock()
		// Add small delay to increase concurrency stress
		time.Sleep(time.Millisecond * 5)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Replay all concurrently
	wg.Add(len(summaries))
	engine := replay.NewEngine(s)
	for _, summary := range summaries {
		go func(id string) {
			defer wg.Done()
			_, err := engine.ReplayByID(ctx, id, target.URL, "")
			if err != nil {
				t.Errorf("Replay %s failed: %v", id, err)
			}
		}(summary.ID)
	}
	wg.Wait()

	if replayCount != numWebhooks {
		t.Errorf("Expected %d replays, got %d", numWebhooks, replayCount)
	}

	t.Logf("✓ Concurrent flow with %d webhooks completed successfully", numWebhooks)
}

// TestFullFlow_TargetUnavailable tests error handling when replay target is down:
// 1. Capture webhook
// 2. Replay to unavailable target
// 3. Verify error is returned but webhook remains stored
func TestFullFlow_TargetUnavailable(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture webhook
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	recorder.ServeHTTP(rr, req)

	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
	whID := summaries[0].ID

	// Try to replay to unavailable target (port 1 is typically closed)
	engine := replay.NewEngine(s)
	_, err := engine.ReplayByID(ctx, whID, "http://localhost:1", "")
	
	// Should get error
	if err == nil {
		t.Error("Expected error for unavailable target")
	}

	// But webhook should still be in store
	wh, err := s.GetWebhook(ctx, whID)
	if err != nil {
		t.Error("Webhook should still be in store after failed replay")
	}
	if wh.ID != whID {
		t.Error("Retrieved wrong webhook")
	}

	t.Log("✓ Target unavailable error handling works correctly")
}

// TestFullFlow_ProviderDetectionAndReplay tests provider-specific handling:
// 1. Capture webhooks from different providers
// 2. Verify correct provider detection
// 3. Replay all
func TestFullFlow_ProviderDetectionAndReplay(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture webhooks from different providers
	providerTests := []struct {
		name           string
		headers        map[string]string
		body           string
		expectedProv   string
		expectedEvent  string
	}{
		{
			name: "Stripe",
			headers: map[string]string{
				"Stripe-Signature": "t=123,v1=abc",
				"Content-Type":     "application/json",
			},
			body:          `{"type":"invoice.paid","id":"in_123"}`,
			expectedProv:  "stripe",
			expectedEvent: "invoice.paid",
		},
		{
			name: "GitHub",
			headers: map[string]string{
				"X-GitHub-Event":      "push",
				"X-Hub-Signature-256": "sha256=abc123",
				"Content-Type":        "application/json",
			},
			body:          `{"ref":"refs/heads/main","repository":{"name":"test"}}`,
			expectedProv:  "github",
			expectedEvent: "push",
		},
		{
			name: "Unknown",
			headers: map[string]string{
				"Content-Type":       "application/json",
				"X-Custom-Signature": "custom",
			},
			body:         `{"event":"test"}`,
			expectedProv: "unknown",
		},
	}

	webhookIDs := make(map[string]string) // name -> ID

	for _, pt := range providerTests {
		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(pt.body))
		for k, v := range pt.headers {
			req.Header.Set(k, v)
		}
		rr := httptest.NewRecorder()
		recorder.ServeHTTP(rr, req)

		// Add small delay to ensure unique timestamps
		time.Sleep(time.Millisecond * 10)

		// Query for the most recent webhook
		summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
		if len(summaries) == 0 {
			t.Fatalf("%s: no webhooks found after capture", pt.name)
		}
		whID := summaries[0].ID
		webhookIDs[pt.name] = whID

		wh, _ := s.GetWebhook(ctx, whID)
		if wh.Provider != pt.expectedProv {
			t.Errorf("%s: expected provider %s, got %s", pt.name, pt.expectedProv, wh.Provider)
		}
		if wh.EventType != pt.expectedEvent {
			t.Errorf("%s: expected event %s, got %s", pt.name, pt.expectedEvent, wh.EventType)
		}
	}

	// Replay all webhooks
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := replay.NewEngine(s)
	for name, whID := range webhookIDs {
		_, err := engine.ReplayByID(ctx, whID, target.URL, "")
		if err != nil {
			t.Errorf("Failed to replay %s webhook: %v", name, err)
		}
	}

	t.Log("✓ Provider detection and replay completed successfully")
}

// TestFullFlow_DeleteAfterReplay tests the delete workflow:
// 1. Capture webhook
// 2. Replay it
// 3. Delete webhook
// 4. Verify it's gone
func TestFullFlow_DeleteAfterReplay(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder.ServeHTTP(httptest.NewRecorder(), req)

	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
	whID := summaries[0].ID

	// Replay
	engine := replay.NewEngine(s)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	_, err := engine.ReplayByID(ctx, whID, target.URL, "")
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	// Delete
	err = s.DeleteWebhook(ctx, whID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = s.GetWebhook(ctx, whID)
	if err == nil {
		t.Error("Expected error when getting deleted webhook")
	}

	// List should be empty
	summaries, _ = s.ListSummaries(ctx, store.ListFilter{Limit: 10})
	if len(summaries) != 0 {
		t.Error("Expected no webhooks after deletion")
	}

	t.Log("✓ Delete after replay flow completed successfully")
}

// TestFullFlow_DryRunDoesNotAffectTarget tests dry-run mode:
// 1. Capture webhook
// 2. Replay in dry-run mode
// 3. Verify target was NOT called
func TestFullFlow_DryRunDoesNotAffectTarget(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	recorder.ServeHTTP(httptest.NewRecorder(), req)

	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
	whID := summaries[0].ID

	// Create target that fails if called
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Target should not be called in dry-run mode")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer target.Close()

	// Replay in dry-run mode
	engine := replay.NewEngine(s)
	engine.DryRun = true

	result, err := engine.ReplayByID(ctx, whID, target.URL, "")
	if err != nil {
		t.Fatalf("Dry-run replay failed: %v", err)
	}

	if result.Sent {
		t.Error("Sent should be false in dry-run mode")
	}
	if result.StatusCode != 0 {
		t.Error("StatusCode should be 0 in dry-run mode")
	}

	t.Log("✓ Dry-run mode works correctly")
}

// TestFullFlow_LargePayload tests handling of large webhooks:
// 1. Capture large webhook (within limits)
// 2. Replay it
// 3. Verify complete payload received
func TestFullFlow_LargePayload(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	// Create a large but valid payload (1 MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}
	payload := fmt.Sprintf(`{"data":"%s","checksum":"%d"}`, string(largeData), len(largeData))

	recorder := proxy.NewRecorderProxy(nil, s)

	// Capture
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	recorder.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Large payload capture failed with status %d", rr.Code)
	}

	summaries, _ := s.ListSummaries(ctx, store.ListFilter{Limit: 1})
	whID := summaries[0].ID

	// Replay
	var receivedLen int
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedLen = len(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	engine := replay.NewEngine(s)
	_, err := engine.ReplayByID(ctx, whID, target.URL, "")
	if err != nil {
		t.Fatalf("Large payload replay failed: %v", err)
	}

	if receivedLen != len(payload) {
		t.Errorf("Payload size mismatch: sent %d, received %d", len(payload), receivedLen)
	}

	t.Logf("✓ Large payload (%d bytes) handled correctly", len(payload))
}

// Helper function to parse URL (copied to avoid import cycles)
func parseURL(s string) (*url.URL, error) {
	return url.Parse(s)
}
