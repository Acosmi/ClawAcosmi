package memory

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LoadSqliteVecExtension attempts to load the sqlite-vec extension from the
// given path or common default locations. Returns ok=true if the extension
// was loaded successfully.
//
// TS source: sqlite-vec.ts (24 lines) — loadSqliteVecExtension().
// The TS version uses better-sqlite3's loadExtension(). We use SQL
// load_extension() which requires the database to be opened with extension
// loading enabled.
func LoadSqliteVecExtension(db *sql.DB, extensionPath string, logger *slog.Logger) (ok bool, resolvedPath string, err error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Allow extension loading.
	if _, err := db.Exec("SELECT load_extension('noop')"); err != nil {
		// Attempt failed — extension loading may be disabled.
		// This is expected; we'll try the actual extension below.
	}

	candidates := buildVecExtensionCandidates(extensionPath)
	if len(candidates) == 0 {
		return false, "", fmt.Errorf("sqlite-vec: no extension path provided and no defaults found")
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		// Strip file extension for load_extension().
		loadPath := strings.TrimSuffix(path, filepath.Ext(path))
		_, execErr := db.Exec("SELECT load_extension(?)", loadPath)
		if execErr != nil {
			logger.Debug("sqlite-vec: load attempt failed", "path", path, "err", execErr)
			continue
		}
		// Verify the extension is functional.
		var version string
		row := db.QueryRow("SELECT vec_version()")
		if scanErr := row.Scan(&version); scanErr != nil {
			logger.Debug("sqlite-vec: vec_version() failed after load", "path", path, "err", scanErr)
			continue
		}
		logger.Info("sqlite-vec loaded", "path", path, "version", version)
		return true, path, nil
	}

	return false, "", fmt.Errorf("sqlite-vec: unable to load from any candidate path")
}

// buildVecExtensionCandidates returns a list of extension file paths to try.
func buildVecExtensionCandidates(userPath string) []string {
	var candidates []string
	if userPath != "" {
		candidates = append(candidates, userPath)
	}

	ext := ".so"
	if runtime.GOOS == "darwin" {
		ext = ".dylib"
	} else if runtime.GOOS == "windows" {
		ext = ".dll"
	}

	// Common locations
	commonDirs := []string{
		"/usr/local/lib",
		"/usr/lib",
		filepath.Join(os.Getenv("HOME"), ".local", "lib"),
	}
	for _, dir := range commonDirs {
		candidates = append(candidates, filepath.Join(dir, "vec0"+ext))
	}

	return candidates
}

// EnsureVecTable creates the virtual vector table if it doesn't already exist.
// Returns true if the table is usable, false otherwise.
func EnsureVecTable(db *sql.DB, tableName string, dims int, logger *slog.Logger) (bool, error) {
	if db == nil || dims <= 0 {
		return false, nil
	}

	// Check if sqlite-vec is loaded by probing vec_version().
	var version string
	if err := db.QueryRow("SELECT vec_version()").Scan(&version); err != nil {
		// Extension not loaded — attempt to load.
		ok, _, loadErr := LoadSqliteVecExtension(db, "", logger)
		if !ok || loadErr != nil {
			return false, nil
		}
	}

	// Create the virtual table if not existing.
	createSQL := fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING vec0(
			id TEXT PRIMARY KEY,
			embedding float[%d]
		)`, tableName, dims,
	)
	if _, err := db.Exec(createSQL); err != nil {
		if logger != nil {
			logger.Warn("memory: failed to create vec table", "table", tableName, "err", err)
		}
		return false, nil
	}
	return true, nil
}
