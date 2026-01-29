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
