package migrator

import (
	"embed"
)

// Option configures the migrator
type Option func(*gormMigrator)

// WithTableName customizes the migration history table name.
func WithTableName(name string) Option {
	return func(m *gormMigrator) {
		m.tableName = name
	}
}

// WithLogger sets a custom logger. A nil logger is ignored (defaults kept).
func WithLogger(l Logger) Option {
	return func(m *gormMigrator) {
		if l != nil {
			m.logger = l
		}
	}
}

// WithMigrationsDir configures a LocalFileProvider rooted at dir.
//
// Note: this REPLACES any previously configured script provider with a
// LocalFileProvider. The previous implementation tried to mutate an existing
// LocalFileProvider via type assertion, which never worked because options
// are applied before any provider is set (scriptProvider is nil at that
// point).
func WithMigrationsDir(dir string) Option {
	return func(m *gormMigrator) {
		m.scriptProvider = NewLocalFileProvider(dir)
	}
}

// WithEmbedMigrationsDir configures an EmbeddedScriptProvider rooted at dir.
func WithEmbedMigrationsDir(dir string, fs embed.FS) Option {
	return func(m *gormMigrator) {
		m.scriptProvider = NewEmbeddedScriptProvider(dir, fs)
	}
}
