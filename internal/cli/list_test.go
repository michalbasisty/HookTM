package cli

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	// Use a fixed reference time for testing relative durations
	now := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    string
		isFrom   bool // true for --from (start of day), false for --to (end of day)
		wantTime time.Time
		wantErr  bool
	}{
		// ISO 8601 formats
		{
			name:     "ISO with Z",
			input:    "2024-01-15T10:30:00Z",
			isFrom:   true,
			wantTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "ISO with offset",
			input:    "2024-01-15T10:30:00+01:00",
			isFrom:   true,
			wantTime: time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC), // Converted to UTC
			wantErr:  false,
		},
		// Date only - from (start of day)
		{
			name:     "date only - from",
			input:    "2024-01-15",
			isFrom:   true,
			wantTime: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		// Date only - to (end of day)
		{
			name:     "date only - to",
			input:    "2024-01-15",
			isFrom:   false,
			wantTime: time.Date(2024, 1, 15, 23, 59, 59, 999999999, time.UTC),
			wantErr:  false,
		},
		// Relative durations (from perspective)
		{
			name:     "relative 1d",
			input:    "1d",
			isFrom:   true,
			wantTime: now.Add(-24 * time.Hour),
			wantErr:  false,
		},
		{
			name:     "relative 7d",
			input:    "7d",
			isFrom:   true,
			wantTime: now.Add(-7 * 24 * time.Hour),
			wantErr:  false,
		},
		{
			name:     "relative 1h",
			input:    "1h",
			isFrom:   true,
			wantTime: now.Add(-time.Hour),
			wantErr:  false,
		},
		// Invalid formats
		{
			name:    "invalid format",
			input:   "not-a-date",
			isFrom:  true,
			wantErr: true,
		},
		{
			name:    "empty",
			input:   "",
			isFrom:  true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeWithReference(tt.input, tt.isFrom, now)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimeWithReference(%q, %v) error = %v, wantErr %v", tt.input, tt.isFrom, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.wantTime) {
				t.Errorf("parseTimeWithReference(%q, %v) = %v, want %v", tt.input, tt.isFrom, got, tt.wantTime)
			}
		})
	}
}
