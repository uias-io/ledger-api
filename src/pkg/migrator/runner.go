package migrator

import "fmt"

// RunWithMigrator orchestrates the database migration process.
//
// Up() is called only once: it performs a single batched query inside filterPending,
// which both determines pending scripts and executes them, avoiding duplicate full scans.
func RunWithMigrator(m Migrator) error {
	m.Logger().Infof("Checking for pending database migrations...")

	if err := m.Up(); err != nil {
		return fmt.Errorf("migration upgrade failed: %w", err)
	}

	latest, err := m.LatestVersion()
	if err != nil {
		return fmt.Errorf("failed to determine current database version: %w", err)
	}
	m.Logger().Infof("Database current version: %s", latest)
	return nil
}
