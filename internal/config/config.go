package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Forward string `yaml:"forward"`
	Port    int    `yaml:"port"`
	DBPath  string `yaml:"db"`
	Lang    string `yaml:"lang"`
}

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
		return nil, err
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

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
