package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"hooktm/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

// TestNewModel tests model initialization
func TestNewModel(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "localhost:3000")

	if m.ctx != ctx {
		t.Error("Context not set correctly")
	}
	if m.store != s {
		t.Error("Store not set correctly")
	}
	if m.defaultTarget != "localhost:3000" {
		t.Errorf("Expected target localhost:3000, got %s", m.defaultTarget)
	}
	if m.sel != 0 {
		t.Errorf("Expected selection 0, got %d", m.sel)
	}
	if len(m.rows) != 0 {
		t.Errorf("Expected empty rows, got %d", len(m.rows))
	}
}

// TestModelInit tests the Init command
func TestModelInit(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	cmd := m.Init()

	// Init should return a command (loadListCmd)
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

// TestUpdateWindowSize tests window resize handling
func TestUpdateWindowSize(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")

	// Send window size message
	newM, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updatedModel := newM.(model)

	if updatedModel.width != 100 {
		t.Errorf("Expected width 100, got %d", updatedModel.width)
	}
	if updatedModel.height != 30 {
		t.Errorf("Expected height 30, got %d", updatedModel.height)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should not return a command")
	}
}

// TestUpdateQuit tests quit key handling
func TestUpdateQuit(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")

	// Test 'q' key
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("Expected quit command for 'q' key")
	}

	// Verify it's a quit command by checking if it produces tea.QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("Expected tea.QuitMsg, got %T", msg)
	}

	// Test ctrl+c
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Expected quit command for Ctrl+C")
	}
}

// TestUpdateNavigation tests navigation keys
func TestUpdateNavigation(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	// Insert test webhooks
	for i := 0; i < 5; i++ {
		s.InsertWebhook(ctx, store.InsertParams{
			ID:       fmt.Sprintf("test-%d", i),
			Method:   "POST",
			Path:     "/webhook",
			Headers:  map[string][]string{},
			Body:     []byte(`{}`),
			BodyText: `{}`,
		})
	}

	m := newModel(ctx, s, "")
	m.rows = []store.WebhookSummary{
		{ID: "test-0", Method: "POST", Path: "/a"},
		{ID: "test-1", Method: "POST", Path: "/b"},
		{ID: "test-2", Method: "POST", Path: "/c"},
	}

	// Test moving down with 'j'
	m.sel = 0
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated := newM.(model)
	if updated.sel != 1 {
		t.Errorf("Expected selection 1 after 'j', got %d", updated.sel)
	}
	if cmd == nil {
		t.Error("Expected loadDetailCmd after navigation")
	}

	// Test moving up with 'k'
	m.sel = 1
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated = newM.(model)
	if updated.sel != 0 {
		t.Errorf("Expected selection 0 after 'k', got %d", updated.sel)
	}

	// Test moving down at boundary
	m.sel = 2 // Last item
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated = newM.(model)
	if updated.sel != 2 {
		t.Errorf("Expected selection to stay at 2 at boundary, got %d", updated.sel)
	}

	// Test moving up at boundary
	m.sel = 0
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated = newM.(model)
	if updated.sel != 0 {
		t.Errorf("Expected selection to stay at 0 at boundary, got %d", updated.sel)
	}
}

// TestUpdateArrowKeys tests arrow key navigation
func TestUpdateArrowKeys(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	m.rows = []store.WebhookSummary{
		{ID: "test-0"},
		{ID: "test-1"},
	}

	// Test down arrow
	m.sel = 0
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := newM.(model)
	if updated.sel != 1 {
		t.Errorf("Expected selection 1 after down arrow, got %d", updated.sel)
	}

	// Test up arrow
	m.sel = 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	updated = newM.(model)
	if updated.sel != 0 {
		t.Errorf("Expected selection 0 after up arrow, got %d", updated.sel)
	}
}

// TestUpdateSearch tests search functionality
func TestUpdateSearch(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")

	// Press '/' to clear search
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updated := newM.(model)
	if updated.search != "" {
		t.Errorf("Expected empty search after '/', got %q", updated.search)
	}

	// Type search query
	newM, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	newM, _ = newM.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	newM, _ = newM.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated = newM.(model)
	if updated.search != "pay" {
		t.Errorf("Expected search 'pay', got %q", updated.search)
	}

	// Press Enter to execute search
	newM, cmd := updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("Expected loadListCmd after Enter with search")
	}

	// Test backspace
	m.search = "test"
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updated = newM.(model)
	if updated.search != "tes" {
		t.Errorf("Expected search 'tes' after backspace, got %q", updated.search)
	}
}

// TestUpdateListLoaded tests handling of listLoadedMsg
func TestUpdateListLoaded(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	m.sel = 5 // Invalid selection

	rows := []store.WebhookSummary{
		{ID: "test-0"},
		{ID: "test-1"},
	}

	newM, cmd := m.Update(listLoadedMsg{rows: rows})
	updated := newM.(model)

	if len(updated.rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(updated.rows))
	}
	if updated.sel != 1 {
		t.Errorf("Expected selection reset to 1 (last item), got %d", updated.sel)
	}
	if cmd == nil {
		t.Error("Expected loadDetailCmd after list loaded")
	}
}

// TestUpdateDetailLoaded tests handling of detailLoadedMsg
func TestUpdateDetailLoaded(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")

	wh := store.Webhook{
		ID:      "test-detail",
		Method:  "POST",
		Path:    "/webhook",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{"test":"data"}`),
	}

	newM, cmd := m.Update(detailLoadedMsg{wh: wh})
	updated := newM.(model)

	if updated.detail == nil {
		t.Fatal("Expected detail to be set")
	}
	if updated.detail.ID != "test-detail" {
		t.Errorf("Expected ID 'test-detail', got %s", updated.detail.ID)
	}
	if cmd != nil {
		t.Error("detailLoadedMsg should not return a command")
	}
}

// TestUpdateError tests error handling
func TestUpdateError(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	testErr := fmt.Errorf("test error")

	newM, cmd := m.Update(errMsg{err: testErr})
	updated := newM.(model)

	if updated.err == nil {
		t.Error("Expected error to be set")
	}
	if updated.err.Error() != "test error" {
		t.Errorf("Expected 'test error', got %v", updated.err)
	}
	if cmd != nil {
		t.Error("errMsg should not return a command")
	}
}

// TestUpdateReplayDone tests replay completion handling
func TestUpdateReplayDone(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")

	// Test successful replay
	newM, cmd := m.Update(replayDoneMsg{err: nil})
	updated := newM.(model)
	if updated.err != nil {
		t.Error("Expected no error for successful replay")
	}
	if cmd != nil {
		t.Error("replayDoneMsg should not return a command")
	}

	// Test failed replay
	newM, cmd = m.Update(replayDoneMsg{err: fmt.Errorf("replay failed")})
	updated = newM.(model)
	if updated.err == nil {
		t.Error("Expected error to be set for failed replay")
	}
}

// TestViewEmptyState tests view rendering with no webhooks
func TestViewEmptyState(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "localhost:3000")
	m.width = 100
	m.height = 30

	view := m.View()

	// Should contain title
	if !strings.Contains(view, "HookTM") {
		t.Error("View should contain 'HookTM' title")
	}

	// Should show target
	if !strings.Contains(view, "localhost:3000") {
		t.Error("View should show target")
	}

	// Should show empty state message
	if !strings.Contains(view, "No webhooks captured yet") {
		t.Error("View should show empty state message")
	}
}

// TestViewWithWebhooks tests view rendering with webhooks
func TestViewWithWebhooks(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "localhost:3000")
	m.width = 100
	m.height = 30
	m.rows = []store.WebhookSummary{
		{ID: "abc123", Method: "POST", Path: "/webhook", Provider: "stripe", ResponseMS: 42},
		{ID: "def456", Method: "GET", Path: "/api", Provider: "github", ResponseMS: 123},
	}

	view := m.View()

	// Should contain webhook IDs
	if !strings.Contains(view, "abc123") {
		t.Error("View should contain webhook ID 'abc123'")
	}
	if !strings.Contains(view, "def456") {
		t.Error("View should contain webhook ID 'def456'")
	}

	// Should show provider info
	if !strings.Contains(view, "stripe") {
		t.Error("View should show 'stripe' provider")
	}
}

// TestViewWithSearch tests view rendering during search
func TestViewWithSearch(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	m.width = 100
	m.height = 30
	m.search = "payment"

	view := m.View()

	// Should show search prompt
	if !strings.Contains(view, "search:") {
		t.Error("View should show search prompt")
	}
	if !strings.Contains(view, "payment") {
		t.Error("View should show search term")
	}
}

// TestViewWithError tests view rendering with error
func TestViewWithError(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	m.width = 100
	m.height = 30
	m.err = fmt.Errorf("something went wrong")

	view := m.View()

	// Should show error
	if !strings.Contains(view, "error:") {
		t.Error("View should show error prefix")
	}
	if !strings.Contains(view, "something went wrong") {
		t.Error("View should contain error message")
	}
}

// TestRenderList tests the renderList function
func TestRenderList(t *testing.T) {
	rows := []store.WebhookSummary{
		{ID: "test-0", Method: "POST", Path: "/a", Provider: "stripe", ResponseMS: 42},
		{ID: "test-1", Method: "GET", Path: "/b", Provider: "github", ResponseMS: 123},
	}

	result := renderList(rows, 0, 60, 10)

	// Should contain first webhook with selection indicator
	if !strings.Contains(result, "> ") {
		t.Error("Selected item should have '> ' prefix")
	}
	if !strings.Contains(result, "test-0") {
		t.Error("Should contain first webhook ID")
	}

	// Should contain second webhook
	if !strings.Contains(result, "test-1") {
		t.Error("Should contain second webhook ID")
	}
}

// TestRenderDetail tests the renderDetail function
func TestRenderDetail(t *testing.T) {
	// Test empty detail
	result := renderDetail(nil, 60, 20)
	if !strings.Contains(result, "No webhooks captured yet") {
		t.Error("Empty detail should show message")
	}

	// Test with webhook
	wh := &store.Webhook{
		ID:        "test-webhook",
		Method:    "POST",
		Path:      "/webhook",
		Provider:  "stripe",
		EventType: "payment.succeeded",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: []byte(`{"amount": 1000}`),
	}

	result = renderDetail(wh, 60, 20)

	if !strings.Contains(result, "test-webhook") {
		t.Error("Should contain webhook ID")
	}
	if !strings.Contains(result, "stripe") {
		t.Error("Should contain provider")
	}
	if !strings.Contains(result, "payment.succeeded") {
		t.Error("Should contain event type")
	}
}

// TestTruncate tests the truncate function
func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		maxW  int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hell…"},
		{"", 5, ""},
		{"test", 0, ""},
		{"test", -1, ""},
		{"a", 1, "a"},
		{"ab", 1, "a"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxW)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxW, got, tt.want)
		}
	}
}

// TestEmptyTo tests the emptyTo function
func TestEmptyTo(t *testing.T) {
	tests := []struct {
		s     string
		v     string
		want  string
	}{
		{"", "default", "default"},
		{"   ", "default", "default"},
		{"value", "default", "value"},
		{" value ", "default", " value "},
	}

	for _, tt := range tests {
		got := emptyTo(tt.s, tt.v)
		if got != tt.want {
			t.Errorf("emptyTo(%q, %q) = %q, want %q", tt.s, tt.v, got, tt.want)
		}
	}
}

// TestLoadListCmd tests the loadListCmd function
func TestLoadListCmd(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	// Insert test data
	for i := 0; i < 3; i++ {
		s.InsertWebhook(ctx, store.InsertParams{
			ID:       fmt.Sprintf("cmd-test-%d", i),
			Method:   "POST",
			Path:     "/webhook",
			Headers:  map[string][]string{},
			Body:     []byte(`{}`),
			BodyText: fmt.Sprintf("body text %d", i),
		})
	}

	m := newModel(ctx, s, "")
	cmd := m.loadListCmd("")

	// Execute the command
	msg := cmd()

	// Should return listLoadedMsg
	loaded, ok := msg.(listLoadedMsg)
	if !ok {
		t.Fatalf("Expected listLoadedMsg, got %T", msg)
	}
	if len(loaded.rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(loaded.rows))
	}
}

// TestLoadDetailCmd tests the loadDetailCmd function
func TestLoadDetailCmd(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	// Insert test webhook
	s.InsertWebhook(ctx, store.InsertParams{
		ID:      "detail-cmd-test",
		Method:  "POST",
		Path:    "/webhook",
		Headers: map[string][]string{},
		Body:    []byte(`{"test":"data"}`),
	})

	m := newModel(ctx, s, "")
	m.rows = []store.WebhookSummary{{ID: "detail-cmd-test"}}
	m.sel = 0

	cmd := m.loadDetailCmd()
	msg := cmd()

	// Should return detailLoadedMsg
	loaded, ok := msg.(detailLoadedMsg)
	if !ok {
		t.Fatalf("Expected detailLoadedMsg, got %T", msg)
	}
	if loaded.wh.ID != "detail-cmd-test" {
		t.Errorf("Expected ID 'detail-cmd-test', got %s", loaded.wh.ID)
	}
}

// TestLoadDetailCmdEmptyRows tests loadDetailCmd with no rows
func TestLoadDetailCmdEmptyRows(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "")
	m.rows = []store.WebhookSummary{}

	cmd := m.loadDetailCmd()
	msg := cmd()

	// Should still return detailLoadedMsg with empty webhook
	loaded, ok := msg.(detailLoadedMsg)
	if !ok {
		t.Fatalf("Expected detailLoadedMsg, got %T", msg)
	}
	if loaded.wh.ID != "" {
		t.Errorf("Expected empty ID for empty rows, got %s", loaded.wh.ID)
	}
}

// TestReplaySelectedCmdNoTarget tests replay without target configured
func TestReplaySelectedCmdNoTarget(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "") // No target
	m.rows = []store.WebhookSummary{{ID: "test-0"}}
	m.sel = 0

	cmd := m.replaySelectedCmd()
	msg := cmd()

	// Should return replayDoneMsg with error
	done, ok := msg.(replayDoneMsg)
	if !ok {
		t.Fatalf("Expected replayDoneMsg, got %T", msg)
	}
	if done.err == nil {
		t.Error("Expected error for missing target")
	}
}

// TestReplaySelectedCmdEmptyRows tests replay with no rows
func TestReplaySelectedCmdEmptyRows(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(":memory:")
	defer s.Close()

	m := newModel(ctx, s, "localhost:3000")
	m.rows = []store.WebhookSummary{}

	cmd := m.replaySelectedCmd()
	msg := cmd()

	// Should return replayDoneMsg without error
	done, ok := msg.(replayDoneMsg)
	if !ok {
		t.Fatalf("Expected replayDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Errorf("Expected no error for empty rows, got %v", done.err)
	}
}

