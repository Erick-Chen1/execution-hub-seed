package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations executes SQL migrations from the migrations directory.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	files := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(files)

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", path, err)
		}
		sql := strings.TrimSpace(string(data))
		if sql == "" {
			continue
		}
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", path, err)
		}
	}

	return nil
}

func IsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}

func EnsureDirExists(dir string) error {
	return os.MkdirAll(dir, fs.ModePerm)
}
