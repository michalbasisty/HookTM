package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// setupBenchStore creates a temporary store for benchmarking
func setupBenchStore(b *testing.B) *Store {
	s, err := Open(":memory:")
	if err != nil {
		b.Fatalf("Failed to open store: %v", err)
	}
	return s
}

// BenchmarkInsertWebhook measures webhook insertion performance
func BenchmarkInsertWebhook(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	params := InsertParams{
		ID:        "bench-test",
		Method:    "POST",
		Path:      "/webhook",
		Headers:   map[string][]string{"Content-Type": {"application/json"}},
		Body:      []byte(`{"event":"test"}`),
		BodyText:  `{"event":"test"}`,
		Provider:  "stripe",
		EventType: "test.event",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		params.ID = fmt.Sprintf("bench-%d", i)
		if err := s.InsertWebhook(ctx, params); err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}
}

// BenchmarkInsertWebhook_WithLargeBody measures insertion with large payloads
func BenchmarkInsertWebhook_WithLargeBody(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	
	// Create a 100KB body
	largeBody := make([]byte, 100*1024)
	for i := range largeBody {
		largeBody[i] = byte('a' + (i % 26))
	}
	
	params := InsertParams{
		ID:       "bench-large",
		Method:   "POST",
		Path:     "/webhook",
		Headers:  map[string][]string{"Content-Type": {"application/json"}},
		Body:     largeBody,
		BodyText: string(largeBody),
		Provider: "stripe",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		params.ID = fmt.Sprintf("bench-large-%d", i)
		if err := s.InsertWebhook(ctx, params); err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}
}

// BenchmarkGetWebhook measures single webhook retrieval performance
func BenchmarkGetWebhook(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	
	// Insert test webhook
	if err := s.InsertWebhook(ctx, InsertParams{
		ID:       "get-test",
		Method:   "POST",
		Path:     "/webhook",
		Headers:  map[string][]string{"Content-Type": {"application/json"}},
		Body:     []byte(`{"test":"data"}`),
		BodyText: `{"test":"data"}`,
	}); err != nil {
		b.Fatalf("Setup insert failed: %v", err)
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := s.GetWebhook(ctx, "get-test")
		if err != nil {
			b.Fatalf("Get failed: %v", err)
		}
	}
}

// BenchmarkListSummaries measures listing performance with different dataset sizes
func BenchmarkListSummaries(b *testing.B) {
	sizes := []int{10, 100, 1000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			s := setupBenchStore(b)
			defer s.Close()
			
			ctx := context.Background()
			
			// Populate database
			for i := 0; i < size; i++ {
				params := InsertParams{
					ID:       fmt.Sprintf("list-%d", i),
					Method:   "POST",
					Path:     "/webhook",
					Headers:  map[string][]string{"Content-Type": {"application/json"}},
					Body:     []byte(`{"test":"data"}`),
					BodyText: `{"test":"data"}`,
					Provider: "stripe",
				}
				if err := s.InsertWebhook(ctx, params); err != nil {
					b.Fatalf("Setup insert failed: %v", err)
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := s.ListSummaries(ctx, ListFilter{Limit: 20})
				if err != nil {
					b.Fatalf("List failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkListSummaries_WithFilters measures listing with various filters
func BenchmarkListSummaries_WithFilters(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	
	// Populate with mixed providers and statuses
	providers := []string{"stripe", "github", "unknown"}
	statuses := []int{200, 400, 500}
	
	for i := 0; i < 100; i++ {
		status := statuses[i%len(statuses)]
		params := InsertParams{
			ID:         fmt.Sprintf("filter-%d", i),
			Method:     "POST",
			Path:       "/webhook",
			Headers:    map[string][]string{"Content-Type": {"application/json"}},
			Body:       []byte(`{"test":"data"}`),
			BodyText:   `{"test":"data"}`,
			Provider:   providers[i%len(providers)],
			StatusCode: &status,
		}
		if err := s.InsertWebhook(ctx, params); err != nil {
			b.Fatalf("Setup insert failed: %v", err)
		}
	}
	
	filters := []struct {
		name   string
		filter ListFilter
	}{
		{"no_filter", ListFilter{Limit: 20}},
		{"by_provider", ListFilter{Limit: 20, Provider: "stripe"}},
		{"by_status", ListFilter{Limit: 20, StatusCode: intPtr(200)}},
		{"by_provider_and_status", ListFilter{Limit: 20, Provider: "stripe", StatusCode: intPtr(200)}},
	}
	
	for _, f := range filters {
		b.Run(f.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := s.ListSummaries(ctx, f.filter)
				if err != nil {
					b.Fatalf("List failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSearchSummaries measures full-text search performance
func BenchmarkSearchSummaries(b *testing.B) {
	sizes := []int{10, 100, 1000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			s := setupBenchStore(b)
			defer s.Close()
			
			ctx := context.Background()
			
			// Populate with varied body text
			for i := 0; i < size; i++ {
				bodyText := fmt.Sprintf("payment invoice customer order %d", i)
				params := InsertParams{
					ID:       fmt.Sprintf("search-%d", i),
					Method:   "POST",
					Path:     "/webhook",
					Headers:  map[string][]string{"Content-Type": {"application/json"}},
					Body:     []byte(`{"test":"data"}`),
					BodyText: bodyText,
				}
				if err := s.InsertWebhook(ctx, params); err != nil {
					b.Fatalf("Setup insert failed: %v", err)
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := s.SearchSummaries(ctx, "payment", 20)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkSearchSummaries_DifferentQueries measures search with different query complexities
func BenchmarkSearchSummaries_DifferentQueries(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	
	// Populate database
	for i := 0; i < 100; i++ {
		bodyText := fmt.Sprintf("payment invoice customer order product subscription charge refund %d", i)
		params := InsertParams{
			ID:       fmt.Sprintf("search-query-%d", i),
			Method:   "POST",
			Path:     "/webhook",
			Headers:  map[string][]string{"Content-Type": {"application/json"}},
			Body:     []byte(`{"test":"data"}`),
			BodyText: bodyText,
		}
		if err := s.InsertWebhook(ctx, params); err != nil {
			b.Fatalf("Setup insert failed: %v", err)
		}
	}
	
	queries := []string{"payment", "invoice customer", "subscription charge refund"}
	
	for _, query := range queries {
		b.Run(fmt.Sprintf("query_%s", query), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := s.SearchSummaries(ctx, query, 20)
				if err != nil {
					b.Fatalf("Search failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkDeleteWebhook measures deletion performance
func BenchmarkDeleteWebhook(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	
	// Pre-populate with webhooks to delete
	for i := 0; i < b.N; i++ {
		params := InsertParams{
			ID:       fmt.Sprintf("delete-%d", i),
			Method:   "POST",
			Path:     "/webhook",
			Headers:  map[string][]string{"Content-Type": {"application/json"}},
			Body:     []byte(`{"test":"data"}`),
			BodyText: `{"test":"data"}`,
		}
		if err := s.InsertWebhook(ctx, params); err != nil {
			b.Fatalf("Setup insert failed: %v", err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		if err := s.DeleteWebhook(ctx, fmt.Sprintf("delete-%d", i)); err != nil {
			b.Fatalf("Delete failed: %v", err)
		}
	}
}

// BenchmarkDeleteByFilter measures bulk deletion performance
func BenchmarkDeleteByFilter(b *testing.B) {
	sizes := []int{10, 100, 500}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			b.Skip("Skipping - requires fresh database for each iteration")
			
			s := setupBenchStore(b)
			defer s.Close()
			
			ctx := context.Background()
			
			// Populate with old webhooks
			oldTime := time.Now().Add(-30 * 24 * time.Hour)
			for i := 0; i < size; i++ {
				params := InsertParams{
					ID:        fmt.Sprintf("old-%d", i),
					CreatedAt: oldTime.Add(time.Duration(i) * time.Second).UnixMilli(),
					Method:    "POST",
					Path:      "/webhook",
					Headers:   map[string][]string{"Content-Type": {"application/json"}},
					Body:      []byte(`{"test":"data"}`),
					BodyText:  `{"test":"data"}`,
					Provider:  "stripe",
				}
				if err := s.InsertWebhook(ctx, params); err != nil {
					b.Fatalf("Setup insert failed: %v", err)
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				_, err := s.DeleteByFilter(ctx, DeleteFilter{
					OlderThan: 7 * 24 * time.Hour,
					Provider:  "stripe",
				})
				if err != nil {
					b.Fatalf("DeleteByFilter failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkConcurrentOperations measures concurrent access performance
func BenchmarkConcurrentOperations(b *testing.B) {
	s := setupBenchStore(b)
	defer s.Close()
	
	ctx := context.Background()
	
	// Pre-populate
	for i := 0; i < 100; i++ {
		params := InsertParams{
			ID:       fmt.Sprintf("concurrent-%d", i),
			Method:   "POST",
			Path:     "/webhook",
			Headers:  map[string][]string{"Content-Type": {"application/json"}},
			Body:     []byte(`{"test":"data"}`),
			BodyText: `{"test":"data"}`,
		}
		if err := s.InsertWebhook(ctx, params); err != nil {
			b.Fatalf("Setup insert failed: %v", err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 3 {
			case 0:
				// Read
				s.ListSummaries(ctx, ListFilter{Limit: 10})
			case 1:
				// Insert
				params := InsertParams{
					ID:       fmt.Sprintf("bench-concurrent-%d", i),
					Method:   "POST",
					Path:     "/webhook",
					Headers:  map[string][]string{"Content-Type": {"application/json"}},
					Body:     []byte(`{"test":"data"}`),
					BodyText: `{"test":"data"}`,
				}
				s.InsertWebhook(ctx, params)
			case 2:
				// Search
				s.SearchSummaries(ctx, "test", 10)
			}
			i++
		}
	})
}

// Helper for benchmarks
func intPtr(v int) *int {
	return &v
}
