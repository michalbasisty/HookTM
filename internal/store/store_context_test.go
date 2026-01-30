package store

import (
	"context"
	"testing"
	"time"
)

func TestOpenContext(t *testing.T) {
	// Test successful open with context
	ctx := context.Background()
	s, err := OpenContext(ctx, ":memory:")
	if err != nil {
		t.Fatalf("OpenContext failed: %v", err)
	}
	defer s.Close()

	// Verify the store works
	if err := s.InsertWebhook(ctx, InsertParams{
		ID:      "test",
		Method:  "POST",
		Path:    "/",
		Headers: map[string][]string{},
		Body:    []byte(`{}`),
	}); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
}

func TestOpenContext_Cancellation(t *testing.T) {
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Open should still succeed because migration is fast
	// This mainly tests that the context is passed through
	s, err := OpenContext(ctx, ":memory:")
	if err != nil {
		t.Fatalf("OpenContext with cancelled context failed: %v", err)
	}
	defer s.Close()
}

func TestInsertWebhook_ContextCancellation(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Insert should fail with cancelled context
	err := s.InsertWebhook(ctx, InsertParams{
		ID:      "test",
		Method:  "POST",
		Path:    "/",
		Headers: map[string][]string{},
		Body:    []byte(`{}`),
	})

	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestGetWebhook_ContextCancellation(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	// First insert with normal context
	ctx := context.Background()
	s.InsertWebhook(ctx, InsertParams{
		ID:      "test",
		Method:  "POST",
		Path:    "/",
		Headers: map[string][]string{},
		Body:    []byte(`{}`),
	})

	// Then try to get with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.GetWebhook(cancelledCtx, "test")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestListSummaries_ContextCancellation(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.ListSummaries(cancelledCtx, ListFilter{Limit: 10})
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestSearchSummaries_ContextCancellation(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := s.SearchSummaries(cancelledCtx, "test", 10)
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestDeleteWebhook_ContextCancellation(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.DeleteWebhook(cancelledCtx, "test")
	if err == nil {
		t.Error("Expected error for cancelled context")
	}
}

func TestStoreOperations_WithTimeout(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	// Test with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Insert
	err := s.InsertWebhook(ctx, InsertParams{
		ID:      "timeout-test",
		Method:  "POST",
		Path:    "/",
		Headers: map[string][]string{},
		Body:    []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("Insert with timeout failed: %v", err)
	}

	// Get
	wh, err := s.GetWebhook(ctx, "timeout-test")
	if err != nil {
		t.Fatalf("Get with timeout failed: %v", err)
	}
	if wh.ID != "timeout-test" {
		t.Error("Wrong webhook retrieved")
	}

	// List
	rows, err := s.ListSummaries(ctx, ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List with timeout failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}
