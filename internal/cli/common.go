package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"hooktm/internal/config"
	"hooktm/internal/store"

	"github.com/urfave/cli/v2"
)

// openStoreFromContext opens the database from CLI context.
// It uses c.Context() for cancellation support during database operations.
func openStoreFromContext(c *cli.Context) (*store.Store, *config.Config, error) {
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	cfgPath := c.String("config")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}

	dbPath := c.String("db")
	if dbPath == "" {
		dbPath = cfg.DBPath
	}
	if dbPath == "" {
		dbPath = defaultDBPath()
	}

	s, err := store.OpenContext(ctx, dbPath)
	if err != nil {
		return nil, nil, err
	}
	return s, cfg, nil
}

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "hooks.db"
	}
	return filepath.Join(home, ".hooktm", "hooks.db")
}

func ensureDirForFile(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func requireArg(c *cli.Context, idx int, name string) (string, error) {
	if c.Args().Len() <= idx {
		return "", fmt.Errorf("missing required argument: %s", name)
	}
	return c.Args().Get(idx), nil
}
