package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("Level.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		want     Level
		wantErr  bool
	}{
		{"DEBUG", DebugLevel, false},
		{"debug", DebugLevel, false},
		{"INFO", InfoLevel, false},
		{"info", InfoLevel, false},
		{"WARN", WarnLevel, false},
		{"WARNING", WarnLevel, false},
		{"warn", WarnLevel, false},
		{"ERROR", ErrorLevel, false},
		{"error", ErrorLevel, false},
		{"invalid", InfoLevel, true},
		{"", InfoLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewDefault(t *testing.T) {
	l := NewDefault()
	if l == nil {
		t.Error("NewDefault() returned nil")
	}
}

func TestNew_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})

	l.Info("test message")
	
	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected 'INFO' in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
}

func TestNew_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "json",
		Output: &buf,
	})

	l.Info("test message")
	
	output := buf.String()
	var entry jsonLogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v\nOutput: %s", err, output)
	}
	
	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
	if entry.Message != "test message" {
		t.Errorf("Expected message 'test message', got %s", entry.Message)
	}
	if entry.Timestamp == "" {
		t.Error("Expected timestamp to be set")
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		logFunc  func(Logger)
		expected string
	}{
		{"Debug", DebugLevel, func(l Logger) { l.Debug("debug msg") }, "debug msg"},
		{"Info", InfoLevel, func(l Logger) { l.Info("info msg") }, "info msg"},
		{"Warn", WarnLevel, func(l Logger) { l.Warn("warn msg") }, "warn msg"},
		{"Error", ErrorLevel, func(l Logger) { l.Error("error msg") }, "error msg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(Config{
				Level:  DebugLevel,
				Format: "text",
				Output: &buf,
			})

			tt.logFunc(l)
			
			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("Expected %q in output, got: %s", tt.expected, output)
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  WarnLevel, // Only WARN and above
		Format: "text",
		Output: &buf,
	})

	l.Debug("debug")
	l.Info("info")
	l.Warn("warn")
	l.Error("error")
	
	output := buf.String()
	
	if strings.Contains(output, "debug") {
		t.Error("DEBUG message should be filtered out")
	}
	if strings.Contains(output, "info") {
		t.Error("INFO message should be filtered out")
	}
	if !strings.Contains(output, "warn") {
		t.Error("WARN message should be present")
	}
	if !strings.Contains(output, "error") {
		t.Error("ERROR message should be present")
	}
}

func TestWithField(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})

	l.WithField("key1", "value1").Info("message")
	
	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Errorf("Expected field in output, got: %s", output)
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "json",
		Output: &buf,
	})

	l.WithFields(Fields{
		"key1": "value1",
		"key2": 42,
	}).Info("message")
	
	output := buf.String()
	var entry jsonLogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	
	if entry.Fields["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %v", entry.Fields["key1"])
	}
	if entry.Fields["key2"] != float64(42) {
		t.Errorf("Expected key2=42, got %v", entry.Fields["key2"])
	}
}

func TestWithFieldChaining(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})

	l.WithField("key1", "value1").
		WithField("key2", "value2").
		Info("message")
	
	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Error("Expected key1 in output")
	}
	if !strings.Contains(output, "key2=value2") {
		t.Error("Expected key2 in output")
	}
}

func TestCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})

	ctx := WithCorrelationID(context.Background(), "abc-123")
	l.WithContext(ctx).Info("message")
	
	output := buf.String()
	if !strings.Contains(output, "correlation_id=abc-123") {
		t.Errorf("Expected correlation_id in output, got: %s", output)
	}
}

func TestCorrelationID_Empty(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})

	// No correlation ID in context
	l.WithContext(context.Background()).Info("message")
	
	output := buf.String()
	if strings.Contains(output, "correlation_id") {
		t.Error("Should not have correlation_id when not set")
	}
}

func TestCorrelationID_Extract(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "test-id")
	if id := CorrelationID(ctx); id != "test-id" {
		t.Errorf("Expected 'test-id', got %q", id)
	}
	
	// Empty context
	if id := CorrelationID(context.Background()); id != "" {
		t.Errorf("Expected empty string, got %q", id)
	}
}

func TestFormattedMethods(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})

	l.Infof("formatted %s %d", "string", 42)
	
	output := buf.String()
	if !strings.Contains(output, "formatted string 42") {
		t.Errorf("Expected formatted message, got: %s", output)
	}
}

func TestNopLogger(t *testing.T) {
	l := NopLogger{}
	
	// All methods should work without panic
	l.Debug("test")
	l.Info("test")
	l.Warn("test")
	l.Error("test")
	l.Debugf("test %s", "arg")
	l.Infof("test %s", "arg")
	l.Warnf("test %s", "arg")
	l.Errorf("test %s", "arg")
	
	// Chaining should work
	l2 := l.WithField("key", "value")
	l3 := l2.WithFields(Fields{"key": "value"})
	l4 := l3.WithContext(context.Background())
	
	if _, ok := l4.(NopLogger); !ok {
		t.Error("NopLogger chaining should return NopLogger")
	}
}

func TestJSONMarshalError(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "json",
		Output: &buf,
	})
	
	// Try to log something that can't be marshaled
	// This is tricky - we need a type that json.Marshal can't handle
	// Using a channel which can't be marshaled
	l.WithField("bad", make(chan int)).Info("test")
	
	output := buf.String()
	// Should have error message about failed marshaling
	if !strings.Contains(output, "failed to marshal") {
		t.Errorf("Expected error message about failed marshaling, got: %s", output)
	}
}

func TestDefaultOutput(t *testing.T) {
	// Test that nil output defaults to os.Stderr
	l := New(Config{
		Level:  ErrorLevel, // Use error level to avoid output during test
		Format: "text",
		Output: nil,
	})
	
	if l == nil {
		t.Error("Logger with nil output should not be nil")
	}
}

func TestDefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Level:  DebugLevel,
		Format: "", // Empty format should default to text
		Output: &buf,
	})
	
	l.Info("test")
	
	output := buf.String()
	// Text format includes brackets around level
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected text format default, got: %s", output)
	}
}

func TestLoggerImmutability(t *testing.T) {
	var buf bytes.Buffer
	base := New(Config{
		Level:  DebugLevel,
		Format: "text",
		Output: &buf,
	})
	
	// Create child loggers with different fields
	child1 := base.WithField("child", "1")
	child2 := base.WithField("child", "2")
	
	// Log from each
	base.Info("base")
	child1.Info("child1")
	child2.Info("child2")
	
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	
	if len(lines) != 3 {
		t.Fatalf("Expected 3 log lines, got %d", len(lines))
	}
	
	// Base should not have child field
	if strings.Contains(lines[0], "child=") {
		t.Error("Base logger should not have child field")
	}
	
	// child1 should have child=1
	if !strings.Contains(lines[1], "child=1") {
		t.Error("child1 should have child=1")
	}
	
	// child2 should have child=2
	if !strings.Contains(lines[2], "child=2") {
		t.Error("child2 should have child=2")
	}
}
