package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_ValidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "empty config",
			cfg:  Config{},
		},
		{
			name: "valid port",
			cfg:  Config{Port: 8080},
		},
		{
			name: "valid forward URL",
			cfg:  Config{Forward: "http://localhost:3000"},
		},
		{
			name: "valid forward shorthand",
			cfg:  Config{Forward: "localhost:3000"},
		},
		{
			name: "valid db path",
			cfg:  Config{DBPath: "/tmp/hooks.db"},
		},
		{
			name: "in-memory db",
			cfg:  Config{DBPath: ":memory:"},
		},
		{
			name: "valid lang go",
			cfg:  Config{Lang: "go"},
		},
		{
			name: "valid lang typescript",
			cfg:  Config{Lang: "typescript"},
		},
		{
			name: "complete valid config",
			cfg: Config{
				Forward: "http://localhost:3000",
				Port:    8080,
				DBPath:  "~/.hooktm/hooks.db",
				Lang:    "go",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		// Port 0 is treated as "unset" and is valid (will use default)
		{"port -1", -1},
		{"port 65536", 65536},
		{"port 100000", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Port: tt.port}
			err := cfg.Validate()
			if err == nil {
				t.Error("Expected validation error for invalid port")
				return
			}
			
			valErr, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("Expected ValidationError, got %T", err)
				return
			}
			
			if valErr.Field != "port" {
				t.Errorf("Expected field 'port', got %q", valErr.Field)
			}
		})
	}
}

func TestValidate_PortZeroIsValid(t *testing.T) {
	// Port 0 is valid (treated as "unset", will use default)
	cfg := Config{Port: 0}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Port 0 should be valid (treated as unset): %v", err)
	}
}

func TestValidate_InvalidForwardURL(t *testing.T) {
	tests := []struct {
		name   string
		forward string
	}{
		{"invalid scheme", "://invalid"},
		{"missing host", "http://"},
		{"just path", "/path/to/something"},
		{"not a URL", "not-a-url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Forward: tt.forward}
			err := cfg.Validate()
			if err == nil {
				t.Error("Expected validation error for invalid forward URL")
				return
			}
			
			valErr, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("Expected ValidationError, got %T", err)
				return
			}
			
			if valErr.Field != "forward" {
				t.Errorf("Expected field 'forward', got %q", valErr.Field)
			}
		})
	}
}

func TestValidate_InvalidDBPath(t *testing.T) {
	tests := []struct {
		name   string
		dbPath string
	}{
		{"path traversal", "../../../etc/passwd"},
		{"path traversal with backslash", "..\\..\\windows\\system32"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{DBPath: tt.dbPath}
			err := cfg.Validate()
			if err == nil {
				t.Error("Expected validation error for invalid db path")
				return
			}
			
			valErr, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("Expected ValidationError, got %T", err)
				return
			}
			
			if valErr.Field != "db" {
				t.Errorf("Expected field 'db', got %q", valErr.Field)
			}
		})
	}
}

func TestValidate_InvalidLang(t *testing.T) {
	tests := []struct {
		name string
		lang string
	}{
		{"invalid lang", "rust"},
		{"empty lang", ""},
		{"random string", "xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Lang: tt.lang}
			// Empty lang is valid (will use default)
			if tt.lang == "" {
				if err := cfg.Validate(); err != nil {
					t.Errorf("Empty lang should be valid: %v", err)
				}
				return
			}
			
			err := cfg.Validate()
			if err == nil {
				t.Error("Expected validation error for invalid lang")
				return
			}
			
			valErr, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("Expected ValidationError, got %T", err)
				return
			}
			
			if valErr.Field != "lang" {
				t.Errorf("Expected field 'lang', got %q", valErr.Field)
			}
		})
	}
}

func TestValidate_ValidLangs(t *testing.T) {
	validLangs := []string{
		"go", "golang",
		"ts", "typescript", "js", "javascript",
		"python", "py",
		"php",
		"ruby", "rb",
	}

	for _, lang := range validLangs {
		t.Run(lang, func(t *testing.T) {
			cfg := Config{Lang: lang}
			if err := cfg.Validate(); err != nil {
				t.Errorf("Expected %q to be valid: %v", lang, err)
			}
		})
	}
}

func TestIsValidationError(t *testing.T) {
	// Test with ValidationError
	valErr := &ValidationError{Field: "test", Message: "test error"}
	if !IsValidationError(valErr) {
		t.Error("IsValidationError should return true for ValidationError")
	}
	
	// Test with nil
	if IsValidationError(nil) {
		t.Error("IsValidationError should return false for nil")
	}
}

func TestLoadAndValidate(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	
	// Valid config
	validConfig := `
forward: localhost:3000
port: 8080
`
	validPath := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write valid config: %v", err)
	}
	
	// Invalid config
	invalidConfig := `
forward: not-a-url
port: 99999
`
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}
	
	// Test valid config
	cfg, err := LoadAndValidate(validPath)
	if err != nil {
		t.Errorf("LoadAndValidate with valid config failed: %v", err)
	}
	if cfg == nil {
		t.Error("Expected config, got nil")
	}
	
	// Test invalid config
	_, err = LoadAndValidate(invalidPath)
	if err == nil {
		t.Error("LoadAndValidate with invalid config should fail")
	}
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	
	if cfg.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Port)
	}
	if cfg.Lang != "go" {
		t.Errorf("Expected default lang 'go', got %q", cfg.Lang)
	}
	if cfg.DBPath == "" {
		t.Error("Expected default DB path to be set")
	}
}

func TestSetDefaults_PreservesExisting(t *testing.T) {
	cfg := &Config{
		Port: 3000,
		Lang: "python",
	}
	cfg.SetDefaults()
	
	if cfg.Port != 3000 {
		t.Errorf("Expected port 3000 to be preserved, got %d", cfg.Port)
	}
	if cfg.Lang != "python" {
		t.Errorf("Expected lang 'python' to be preserved, got %q", cfg.Lang)
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"8080", 8080, false},
		{"1", 1, false},
		{"65535", 65535, false},
		{"", 0, true},
		{"0", 0, true},
		{"-1", 0, true},
		{"65536", 0, true},
		{"abc", 0, true},
		{"80.5", 0, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParsePort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePort(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePort(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestExampleConfig(t *testing.T) {
	example := ExampleConfig()
	if example == "" {
		t.Error("ExampleConfig should not return empty string")
	}
	
	// Should contain all config fields
	requiredFields := []string{"forward:", "port:", "db:", "lang:"}
	for _, field := range requiredFields {
		if !strings.Contains(example, field) {
			t.Errorf("Example config should contain %q", field)
		}
	}
}

func TestConfig_String(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "empty config",
			cfg:  Config{},
			want: "Config{}",
		},
		{
			name: "port only",
			cfg:  Config{Port: 8080},
			want: "Config{port=8080}",
		},
		{
			name: "all fields",
			cfg: Config{
				Forward: "localhost:3000",
				Port:    8080,
				DBPath:  "hooks.db",
				Lang:    "go",
			},
			want: `forward="localhost:3000" port=8080 db="hooks.db" lang="go"`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("Config.String() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}
