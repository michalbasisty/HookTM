package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Forward string `yaml:"forward"`
	Port    int    `yaml:"port"`
	DBPath  string `yaml:"db"`
	Lang    string `yaml:"lang"`
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation error for field %q: %s", e.Field, e.Message)
}

// Load loads the configuration from the given path.
// If path is empty, it tries to load from the default location.
// It does not validate the configuration; use Validate() for that.
func Load(path string) (*Config, error) {
	// Defaults only; config file optional for MVP.
	cfg := &Config{}
	if strings.TrimSpace(path) == "" {
		// Try ~/.hooktm/config.yaml if exists.
		if p, ok := defaultConfigPathIfExists(); ok {
			path = p
		} else {
			return cfg, nil
		}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return cfg, nil
}

// LoadAndValidate loads and validates the configuration.
func LoadAndValidate(path string) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate validates the configuration values.
// Returns the first validation error found, or nil if valid.
func (c *Config) Validate() error {
	// Validate port (if set)
	if c.Port != 0 {
		if err := validatePort(c.Port); err != nil {
			return &ValidationError{
				Field:   "port",
				Value:   c.Port,
				Message: err.Error(),
			}
		}
	}

	// Validate forward URL (if set)
	if strings.TrimSpace(c.Forward) != "" {
		if err := validateForwardURL(c.Forward); err != nil {
			return &ValidationError{
				Field:   "forward",
				Value:   c.Forward,
				Message: err.Error(),
			}
		}
	}

	// Validate DB path (if set)
	if strings.TrimSpace(c.DBPath) != "" {
		if err := validateDBPath(c.DBPath); err != nil {
			return &ValidationError{
				Field:   "db",
				Value:   c.DBPath,
				Message: err.Error(),
			}
		}
	}

	// Validate lang (if set)
	if strings.TrimSpace(c.Lang) != "" {
		if err := validateLang(c.Lang); err != nil {
			return &ValidationError{
				Field:   "lang",
				Value:   c.Lang,
				Message: err.Error(),
			}
		}
	}

	return nil
}

// validatePort validates that the port is within the valid range (1-65535).
func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}
	return nil
}

// validateForwardURL validates the forward URL format.
func validateForwardURL(forward string) error {
	forward = strings.TrimSpace(forward)
	if forward == "" {
		return nil
	}

	// Allow host:port shorthand
	if !strings.Contains(forward, "://") && strings.Contains(forward, ":") {
		forward = "http://" + forward
	}

	u, err := url.Parse(forward)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL format: %q", forward)
	}
	return nil
}

// validateDBPath validates the database path.
func validateDBPath(dbPath string) error {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" {
		return nil
	}

	// Check for path traversal attempts
	if strings.Contains(dbPath, "..") {
		return fmt.Errorf("path contains invalid characters: %q", dbPath)
	}

	// Check if parent directory exists or can be created
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		info, err := os.Stat(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("cannot access directory %q: %w", dir, err)
			}
			// Directory doesn't exist, that's ok - it will be created
		} else if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", dir)
		}
	}

	return nil
}

// validateLang validates the language code.
func validateLang(lang string) error {
	validLangs := map[string]bool{
		"go":         true,
		"golang":     true,
		"ts":         true,
		"typescript": true,
		"js":         true,
		"javascript": true,
		"python":     true,
		"py":         true,
		"php":        true,
		"ruby":       true,
		"rb":         true,
	}

	lang = strings.ToLower(strings.TrimSpace(lang))
	if !validLangs[lang] {
		return fmt.Errorf("unsupported language: %q (valid: go, ts, python, php, ruby)", lang)
	}
	return nil
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// defaultConfigPathIfExists returns the default config path if it exists.
func defaultConfigPathIfExists() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	p := filepath.Join(home, ".hooktm", "config.yaml")
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}

// ExampleConfig returns an example configuration string.
func ExampleConfig() string {
	return `# HookTM Configuration File
# Place this file at ~/.hooktm/config.yaml

# Default forward target for replay
forward: localhost:3000

# Default port for the listen command
port: 8080

# Database file path
db: ~/.hooktm/hooks.db

# Default language for code generation
lang: go
`
}

// SetDefaults sets default values for empty fields.
func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = 8080
	}
	if strings.TrimSpace(c.Lang) == "" {
		c.Lang = "go"
	}
	if strings.TrimSpace(c.DBPath) == "" {
		home, _ := os.UserHomeDir()
		c.DBPath = filepath.Join(home, ".hooktm", "hooks.db")
	}
}

// String returns a string representation of the config (for debugging).
func (c *Config) String() string {
	var parts []string
	if c.Forward != "" {
		parts = append(parts, fmt.Sprintf("forward=%q", c.Forward))
	}
	if c.Port != 0 {
		parts = append(parts, fmt.Sprintf("port=%d", c.Port))
	}
	if c.DBPath != "" {
		parts = append(parts, fmt.Sprintf("db=%q", c.DBPath))
	}
	if c.Lang != "" {
		parts = append(parts, fmt.Sprintf("lang=%q", c.Lang))
	}
	return "Config{" + strings.Join(parts, " ") + "}"
}

// ParsePort parses a port string and validates it.
func ParsePort(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("port cannot be empty")
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("port must be a number: %w", err)
	}
	if err := validatePort(port); err != nil {
		return 0, err
	}
	return port, nil
}
