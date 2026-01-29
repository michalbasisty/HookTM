package store

import (
	"context"
	"testing"
)

func TestInsertAndGet(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	err = s.InsertWebhook(ctx, InsertParams{
		ID:         "abc123",
		CreatedAt:  1700000000000,
		Method:     "POST",
		Path:       "/hooks/stripe",
		Query:      "a=1",
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{"hello":"world"}`),
		Provider:   "stripe",
		EventType:  "evt",
		StatusCode: ptr(200),
		ResponseMS: 12,
		BodyText:   `{"hello":"world"}`,
	})
	if err != nil {
		t.Fatalf("InsertWebhook: %v", err)
	}

	wh, err := s.GetWebhook(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if wh.Provider != "stripe" || wh.Method != "POST" || wh.Path != "/hooks/stripe" {
		t.Fatalf("unexpected webhook: %+v", wh)
	}
}

func TestListAndSearch(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	_ = s.InsertWebhook(ctx, InsertParams{
		ID:        "1",
		CreatedAt: 1,
		Method:    "POST",
		Path:      "/a",
		Headers:   map[string][]string{"Content-Type": {"application/json"}},
		Body:      []byte(`{"k":"alpha beta"}`),
		BodyText:  `alpha beta`,
	})
	_ = s.InsertWebhook(ctx, InsertParams{
		ID:        "2",
		CreatedAt: 2,
		Method:    "POST",
		Path:      "/b",
		Headers:   map[string][]string{"Content-Type": {"application/json"}},
		Body:      []byte(`{"k":"gamma"}`),
		BodyText:  `gamma`,
	})

	rows, err := s.ListSummaries(ctx, ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListSummaries: %v", err)
	}
	if len(rows) != 2 || rows[0].ID != "2" {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	searchRows, err := s.SearchSummaries(ctx, "alpha", 10)
	if err != nil {
		t.Fatalf("SearchSummaries: %v", err)
	}
	if len(searchRows) != 1 || searchRows[0].ID != "1" {
		t.Fatalf("unexpected search rows: %+v", searchRows)
	}
}

func ptr(v int) *int { return &v }

func TestDeleteWebhook(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	_ = s.InsertWebhook(ctx, InsertParams{
		ID:        "del1",
		CreatedAt: 1,
		Method:    "POST",
		Path:      "/a",
		Headers:   map[string][]string{"Content-Type": {"application/json"}},
		Body:      []byte(`{}`),
		BodyText:  `{}`,
	})
	_ = s.InsertWebhook(ctx, InsertParams{
		ID:        "del2",
		CreatedAt: 2,
		Method:    "POST",
		Path:      "/b",
		Headers:   map[string][]string{"Content-Type": {"application/json"}},
		Body:      []byte(`{}`),
		BodyText:  `{}`,
	})

	// Delete by ID
	if err := s.DeleteWebhook(ctx, "del1"); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}

	// Verify deleted
	_, err = s.GetWebhook(ctx, "del1")
	if err == nil {
		t.Fatal("expected error for deleted webhook")
	}

	// Verify other still exists
	wh, err := s.GetWebhook(ctx, "del2")
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if wh.ID != "del2" {
		t.Fatalf("unexpected webhook: %+v", wh)
	}

	// Delete non-existent should error
	if err := s.DeleteWebhook(ctx, "notfound"); err == nil {
		t.Fatal("expected error for non-existent webhook")
	}
}

func TestDeleteByFilter(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	now := 1000000000000
	_ = s.InsertWebhook(ctx, InsertParams{
		ID:         "old1",
		CreatedAt:  int64(now - 86400000*10), // 10 days old
		Method:     "POST",
		Path:       "/a",
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{}`),
		BodyText:   `{}`,
		Provider:   "stripe",
		StatusCode: ptr(500),
	})
	_ = s.InsertWebhook(ctx, InsertParams{
		ID:         "old2",
		CreatedAt:  int64(now - 86400000*5), // 5 days old
		Method:     "POST",
		Path:       "/b",
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{}`),
		BodyText:   `{}`,
		Provider:   "github",
		StatusCode: ptr(200),
	})
	_ = s.InsertWebhook(ctx, InsertParams{
		ID:         "new1",
		CreatedAt:  int64(now),
		Method:     "POST",
		Path:       "/c",
		Headers:    map[string][]string{"Content-Type": {"application/json"}},
		Body:       []byte(`{}`),
		BodyText:   `{}`,
		Provider:   "stripe",
		StatusCode: ptr(200),
	})

	// Delete by provider
	n, err := s.DeleteByFilter(ctx, DeleteFilter{Provider: "stripe"})
	if err != nil {
		t.Fatalf("DeleteByFilter: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}

	// Delete by status
	status := 200
	n, err = s.DeleteByFilter(ctx, DeleteFilter{StatusCode: &status})
	if err != nil {
		t.Fatalf("DeleteByFilter: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}

	// Empty filter should error
	_, err = s.DeleteByFilter(ctx, DeleteFilter{})
	if err == nil {
		t.Fatal("expected error for empty filter")
	}
}
