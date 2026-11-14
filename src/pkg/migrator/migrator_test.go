// Package migrator tests.
//
// Prerequisites
//
//	All tests run against an in-memory SQLite database, so no external
//	service is required. However, the upgrade tests read the local
//	"test_scripts" directory via //go:embed and also via os.ReadDir, so
//	that directory MUST exist alongside this file with at least:
//
//	    test_scripts/v1.0.0_create_table_up.sql
//	    test_scripts/v1.0.0_create_table_down.sql
//	    test_scripts/v1.0.1_create_table_up.sql
//	    test_scripts/v1.0.1_create_table_down.sql
//
//	Missing files will fail TestUpMigrationA / TestUpMigrationWithEmbedMigrationsDir
//	/ TestNewDefault early with a clear error.
//
// Running
//
//	# Run every test in this package.
//	go test ./src/pkg/migrator/...
//
//	# Verbose output (shows each sub-test name and PASS/FAIL).
//	go test ./src/pkg/migrator/... -v
//
//	# Run a single test (or a sub-test) by name pattern.
//	go test ./src/pkg/migrator/... -run 'TestNoScriptProvider'
//	go test ./src/pkg/migrator/... -run 'TestSplitSQLPreservesExecutableComments/optimizer_hint_is_discarded' -v
//
//	# Run with the race detector (recommended before tagging a release).
//	go test ./src/pkg/migrator/... -race
//
//	# Show coverage.
//	go test ./src/pkg/migrator/... -cover
//
// Notes
//
//   - newTestDB() silences the GORM logger, so successful runs produce
//     almost no output; a clean `ok   ledger/src/pkg/migrator` line means
//     everything passed.
//   - Tests are independent: each opens its own :memory: SQLite instance
//     and its own migration history table, so they can be run in any order
//     or in parallel by the test framework.
//   - TestSplitSQLPreservesExecutableComments asserts on the final SQL
//     form (splitSQL output passed through collapseToSingleLine), which is
//     what executeScript actually sends to the database.
package migrator

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB creates a fresh in-memory SQLite database for each test.
// GORM's logger is silenced to keep test output free of SQL noise.
func newTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
}

// TestUpMigrationA tests the upgrade flow using a local file provider.
func TestUpMigrationA(t *testing.T) {
	db := newTestDB(t)
	tableName := "auto_migrations"
	require.NoError(t, InitTable(db, tableName))
	rootDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	scriptsDir := filepath.Join(rootDir, "test_scripts")
	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		t.Fatalf("script directory does not exist: %s", scriptsDir)
	}

	m := New(db, WithTableName(tableName))
	m.SetScriptProvider(NewLocalFileProvider(scriptsDir))

	hasPending, err := m.HasPending()
	require.NoError(t, err)
	assert.True(t, hasPending)

	latest, err := m.LatestVersion()
	require.NoError(t, err)
	assert.Empty(t, latest)

	require.NoError(t, m.Up())

	hasPending, err = m.HasPending()
	require.NoError(t, err)
	assert.False(t, hasPending)

	latest, err = m.LatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", latest)

	var count int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test'").Scan(&count)
	assert.Equal(t, int64(1), count)
}

//go:embed test_scripts/*.sql
var testScripts embed.FS

// TestUpMigrationWithEmbedMigrationsDir tests the upgrade flow using an embedded provider.
func TestUpMigrationWithEmbedMigrationsDir(t *testing.T) {
	db := newTestDB(t)
	tableName := "auto_migrations"
	require.NoError(t, InitTable(db, tableName))

	if _, err := os.ReadDir("test_scripts"); err != nil {
		t.Fatalf("embedded script directory does not exist: %v", err)
	}

	m := New(db, WithTableName(tableName), WithEmbedMigrationsDir("test_scripts", testScripts))

	hasPending, err := m.HasPending()
	require.NoError(t, err)
	assert.True(t, hasPending)

	latest, err := m.LatestVersion()
	require.NoError(t, err)
	assert.Empty(t, latest)

	require.NoError(t, m.Up())

	hasPending, err = m.HasPending()
	require.NoError(t, err)
	assert.False(t, hasPending)

	latest, err = m.LatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", latest)

	var count int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test'").Scan(&count)
	assert.Equal(t, int64(1), count)
}

// TestNewDefault tests migrator construction via NewDefault with a Config.
func TestNewDefault(t *testing.T) {
	db := newTestDB(t)
	tableName := "auto_migrations"
	m := NewDefault(db, &Config{
		TableName:      tableName,
		ScriptProvider: NewEmbeddedScriptProvider("test_scripts", testScripts),
	})

	hasPending, err := m.HasPending()
	require.NoError(t, err)
	assert.True(t, hasPending)

	latest, err := m.LatestVersion()
	require.NoError(t, err)
	assert.Empty(t, latest)

	require.NoError(t, m.Up())

	hasPending, err = m.HasPending()
	require.NoError(t, err)
	assert.False(t, hasPending)

	latest, err = m.LatestVersion()
	require.NoError(t, err)
	assert.Equal(t, "v1.0.1", latest)

	var count int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test'").Scan(&count)
	assert.Equal(t, int64(1), count)
}

// TestNoScriptProvider verifies that operations requiring a script provider
// return ErrNoScriptProvider instead of panicking when none is configured.
func TestNoScriptProvider(t *testing.T) {
	db := newTestDB(t)
	tableName := "auto_migrations"
	require.NoError(t, InitTable(db, tableName))

	m := New(db, WithTableName(tableName))

	t.Run("Up", func(t *testing.T) {
		err := m.Up()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoScriptProvider)
	})

	t.Run("Down", func(t *testing.T) {
		err := m.Down()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoScriptProvider)
	})

	t.Run("HasPending", func(t *testing.T) {
		_, err := m.HasPending()
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNoScriptProvider)
	})

	t.Run("LatestVersion still works without a provider", func(t *testing.T) {
		v, err := m.LatestVersion()
		require.NoError(t, err)
		assert.Empty(t, v)
	})
}

// TestWithLogger verifies that WithLogger overrides the default logger and
// that a nil argument is ignored.
func TestWithLogger(t *testing.T) {
	db := newTestDB(t)

	t.Run("custom logger is applied", func(t *testing.T) {
		custom := NewDefaultLogger(Debug)
		m := New(db, WithLogger(custom))
		assert.Same(t, custom, m.Logger())
	})

	t.Run("nil logger is ignored", func(t *testing.T) {
		m := New(db, WithLogger(nil))
		assert.NotNil(t, m.Logger())
	})
}

// TestSplitSQLPreservesExecutableComments locks in the behavior that MySQL
// executable comments (/*! ... */) are preserved verbatim so that the
// database executes the embedded SQL (SET NAMES, CREATE TRIGGER, ...),
// while ordinary block comments (including optimizer hints /*+ ... */) are
// discarded.
//
// Assertions are made on the *final* statement form, i.e. splitSQL output
// passed through collapseToSingleLine. That is exactly what executeScript
// sends to the database, and it is where redundant whitespace left by
// discarded comments gets normalized.
func TestSplitSQLPreservesExecutableComments(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "standalone executable comment is preserved",
			input: "/*!40101 SET NAMES utf8mb4 */;",
			want:  []string{"/*!40101 SET NAMES utf8mb4 */"},
		},
		{
			name:  "inline executable comment is preserved",
			input: "CREATE TABLE t (a int) /*!50100 ENGINE=InnoDB */;",
			want:  []string{"CREATE TABLE t (a int) /*!50100 ENGINE=InnoDB */"},
		},
		{
			name:  "executable comment appended to DML is preserved",
			input: "INSERT INTO t VALUES (1) /*! ON DUPLICATE KEY UPDATE a=1 */;",
			want:  []string{"INSERT INTO t VALUES (1) /*! ON DUPLICATE KEY UPDATE a=1 */"},
		},
		{
			name:  "ordinary block comment is discarded",
			input: "SELECT 1 /* just a comment */;",
			want:  []string{"SELECT 1"},
		},
		{
			name:  "optimizer hint is discarded",
			input: "SELECT /*+ MAX_EXECUTION_TIME(1000) */ * FROM t;",
			want:  []string{"SELECT * FROM t"},
		},
		{
			name:  "mixed executable and ordinary comments",
			input: "/* ordinary */ SELECT 1 /*! ; */ /* another */;",
			want:  []string{"SELECT 1 /*! ; */"},
		},
		{
			name:  "unterminated executable comment consumes the rest",
			input: "SELECT 1 /*!40101 SET @x=1",
			want:  []string{"SELECT 1 /*!40101 SET @x=1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmts := splitSQL(tc.input)
			// Normalize through collapseToSingleLine, matching the real
			// executeScript pipeline (splitSQL -> collapseToSingleLine -> Exec).
			final := make([]string, len(stmts))
			for i, s := range stmts {
				final[i] = collapseToSingleLine(s)
			}
			assert.Equal(t, tc.want, final)
		})
	}
}

// TestTruncateSQLPreservesUTF8 verifies that truncateSQL cuts on rune
// boundaries so the result remains valid UTF-8.
func TestTruncateSQLPreservesUTF8(t *testing.T) {
	// "abc中文def": 3 ASCII + 2 CJK (each 3 bytes) + 3 ASCII = 8 runes.
	const sql = "abc中文def"

	assert.Equal(t, sql, truncateSQL(sql, 0))
	assert.Equal(t, sql, truncateSQL(sql, 100))
	assert.Equal(t, "abc中...(truncated)", truncateSQL(sql, 4))

	// All results must be valid UTF-8.
	for _, limit := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		out := truncateSQL(sql, limit)
		assert.True(t, utf8.ValidString(out), "limit=%d produced invalid UTF-8: %q", limit, out)
	}
}
