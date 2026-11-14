package migrator

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ErrNoScriptProvider is returned when a migration operation is attempted
// before a ScriptProvider has been configured.
//
// It can be checked by callers via errors.Is:
//
//	if err := m.Up(); errors.Is(err, migrator.ErrNoScriptProvider) {
//	    // provider was not configured
//	}
var ErrNoScriptProvider = errors.New(
	"script provider is not configured: call SetScriptProvider, " +
		"or pass WithMigrationsDir / WithEmbedMigrationsDir")

type gormMigrator struct {
	db             *gorm.DB
	tableName      string
	scriptProvider ScriptProvider
	logger         Logger
}

// newGormMigrator creates a gormMigrator with safe defaults shared by all constructors.
func newGormMigrator(db *gorm.DB) *gormMigrator {
	return &gormMigrator{
		db:        db,
		tableName: "auto_migration",
		logger:    NewDefaultLogger(Error),
	}
}

// New creates a configurable migrator instance using functional options.
func New(db *gorm.DB, opts ...Option) Migrator {
	m := newGormMigrator(db)
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// NewDefault creates a migrator instance from a Config struct.
func NewDefault(db *gorm.DB, config *Config) Migrator {
	m := newGormMigrator(db)
	if config != nil {
		if config.Logger != nil {
			m.logger = config.Logger
		}
		if config.TableName != "" {
			m.tableName = config.TableName
		}
		if config.ScriptProvider != nil {
			m.scriptProvider = config.ScriptProvider
		}
	}
	if err := InitTable(db, m.tableName); err != nil {
		m.logger.Errorf("Failed to initialize migration table: %v", err)
	}
	return m
}

func (m *gormMigrator) SetScriptProvider(provider ScriptProvider) {
	m.scriptProvider = provider
}

func (m *gormMigrator) Logger() Logger {
	return m.logger
}

// ensureProvider returns ErrNoScriptProvider when no provider is configured.
// It is called at the entry of every operation that reads scripts, so a
// misconfigured migrator fails fast with a clear error instead of panicking.
func (m *gormMigrator) ensureProvider() error {
	if m.scriptProvider == nil {
		return ErrNoScriptProvider
	}
	return nil
}

func (m *gormMigrator) Up() error {
	if err := m.ensureProvider(); err != nil {
		return err
	}

	scripts, err := m.listScripts("up")
	if err != nil {
		return fmt.Errorf("failed to list upgrade scripts: %w", err)
	}

	pendingScripts := m.filterPending(scripts, "up")
	if len(pendingScripts) == 0 {
		m.logger.Infof("No pending migrations found")
		return nil
	}

	m.logger.Infof("Pending migrations detected (%d). Applying upgrade scripts...", len(pendingScripts))

	sort.Slice(pendingScripts, func(i, j int) bool {
		return compareVersions(pendingScripts[i].version, pendingScripts[j].version) < 0
	})

	for _, script := range pendingScripts {
		if err := m.executeScript(script.path, script.version, script.name, "up"); err != nil {
			return fmt.Errorf("failed to execute script %s: %w", script.name, err)
		}
	}
	return nil
}

func (m *gormMigrator) Down() error {
	if err := m.ensureProvider(); err != nil {
		return err
	}

	lastUp, err := m.getLatestUpRecord()
	if err != nil {
		return err
	}

	// Derive the down script name from the up script name by swapping the
	// suffix. HasSuffix/TrimSuffix is used instead of strings.Replace so
	// that an occurrence of "_up.sql" in the middle of the filename cannot
	// be accidentally rewritten.
	const upSuffix, downSuffix = "_up.sql", "_down.sql"
	if !strings.HasSuffix(lastUp.ScriptName, upSuffix) {
		return fmt.Errorf("unexpected up script name (missing %q suffix): %s",
			upSuffix, lastUp.ScriptName)
	}
	downScriptName := strings.TrimSuffix(lastUp.ScriptName, upSuffix) + downSuffix

	scripts, err := m.listScripts("down")
	if err != nil {
		return err
	}
	var target *script
	for i := range scripts {
		if scripts[i].name == downScriptName {
			target = &scripts[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("rollback script does not exist: %s", downScriptName)
	}

	if err := m.executeScript(target.path, lastUp.Version, downScriptName, "down"); err != nil {
		return fmt.Errorf("failed to execute rollback script: %w", err)
	}

	isoTime := time.Now().Format(ISO8601TZ)
	return m.db.Table(m.tableName).Where("id = ?", lastUp.ID).Update("deleted_time", isoTime).Error
}

func (m *gormMigrator) HasPending() (bool, error) {
	if err := m.ensureProvider(); err != nil {
		return false, err
	}

	scripts, err := m.listScripts("up")
	if err != nil {
		return false, err
	}
	pending := m.filterPending(scripts, "up")
	return len(pending) > 0, nil
}

func (m *gormMigrator) LatestVersion() (string, error) {
	var record MigrationRecord
	result := m.db.Table(m.tableName).
		Where("direction = ? AND status = ? AND (deleted_time IS NULL OR deleted_time = '')",
			"up", "success").
		Order("id DESC").
		First(&record)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return record.Version, result.Error
}

// executeScript executes a migration script within a database transaction.
//
// The named return value `err` is intentional: the recover() branch must convert a
// panic into a non-nil error, otherwise the caller would treat the rollback as success.
func (m *gormMigrator) executeScript(scriptPath, version, scriptName, direction string) (err error) {
	if err := m.ensureProvider(); err != nil {
		return err
	}

	content, err := m.scriptProvider.ReadScript(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Empty file or whitespace-only: treat as success and record it.
	if strings.TrimSpace(content) == "" {
		if err := m.recordResult(version, scriptName, direction, "success", ""); err != nil {
			return fmt.Errorf("empty script succeeded but failed to record result: %w", err)
		}
		return nil
	}

	sqlStatements := splitSQL(content)

	// Comment-only script (no executable statements after splitSQL): treat as success and record it.
	if len(sqlStatements) == 0 {
		if err := m.recordResult(version, scriptName, direction, "success", ""); err != nil {
			return fmt.Errorf("comment-only script succeeded but failed to record result: %w", err)
		}
		return nil
	}

	tx := m.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Convert a panic into a non-nil error and a "fail" history record.
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = fmt.Errorf("panic during migration script %s: %v", scriptName, r)
			if recordErr := m.recordResult(version, scriptName, direction, "fail", err.Error()); recordErr != nil {
				m.logger.Errorf("Failed to record migration panic: %v (original panic: %v)", recordErr, r)
			}
		}
	}()

	for i, stmt := range sqlStatements {
		singleLine := collapseToSingleLine(stmt)
		if !strings.HasSuffix(singleLine, ";") {
			singleLine += ";"
		}

		if err := tx.Exec(singleLine).Error; err != nil {
			tx.Rollback()
			execErr := fmt.Errorf(
				"failed to execute SQL statement %d (SQL: %s): %w",
				i+1, truncateSQL(singleLine, 4000), err,
			)
			if recordErr := m.recordResult(version, scriptName, direction, "fail", execErr.Error()); recordErr != nil {
				m.logger.Errorf("Failed to record migration failure: %v (original error: %v)", recordErr, execErr)
			}
			return execErr
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		commitErr := fmt.Errorf("failed to commit transaction: %w", err)
		if recordErr := m.recordResult(version, scriptName, direction, "fail", commitErr.Error()); recordErr != nil {
			m.logger.Errorf("Failed to record migration failure: %v (original error: %v)", recordErr, commitErr)
		}
		return commitErr
	}

	if err := m.recordResult(version, scriptName, direction, "success", ""); err != nil {
		return fmt.Errorf("migration succeeded but failed to record result: %w", err)
	}

	return nil
}

func (m *gormMigrator) recordResult(version, scriptName, direction, status, errMsg string) error {
	isoTime := time.Now().Format(ISO8601TZ)
	return m.db.Table(m.tableName).Create(
		&MigrationRecord{
			Version:      version,
			ScriptName:   scriptName,
			Direction:    direction,
			ExecutedTime: isoTime,
			Status:       status,
			ErrorMsg:     errMsg,
		}).Error
}

func (m *gormMigrator) getLatestUpRecord() (*MigrationRecord, error) {
	var record MigrationRecord
	result := m.db.Table(m.tableName).
		Where("direction = ? AND status = ? AND (deleted_time IS NULL OR deleted_time = '')",
			"up", "success").
		Order("id DESC").
		First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, errors.New("no rollbackable upgrade record found")
	}
	return &record, result.Error
}

// listScripts delegates to the configured provider. It returns
// ErrNoScriptProvider when no provider is set, so even unexpected direct
// callers cannot trigger a nil-pointer panic.
func (m *gormMigrator) listScripts(direction string) ([]script, error) {
	if err := m.ensureProvider(); err != nil {
		return nil, err
	}
	return m.scriptProvider.ListScripts(direction)
}

// filterPending identifies scripts that haven't been successfully executed.
//
// Uses a single SELECT version, script_name to fetch all executed records,
// then filters in memory via a map, reducing O(N) count queries to 1.
//
// If the query itself fails (e.g. transient network error), we conservatively
// treat every script as pending (same as the previous behavior) but emit a
// warning so the operator can see that the decision is based on a failed query.
func (m *gormMigrator) filterPending(scripts []script, direction string) []script {
	if len(scripts) == 0 {
		return nil
	}

	type row struct {
		Version    string
		ScriptName string
	}
	var rows []row
	if err := m.db.Table(m.tableName).
		Select("version, script_name").
		Where("direction = ? AND status = ? AND (deleted_time IS NULL OR deleted_time = '')",
			direction, "success").
		Find(&rows).Error; err != nil {
		m.logger.Warnf("failed to query executed migrations; treating all %d scripts as pending: %v",
			len(scripts), err)
		return scripts
	}

	// key = version + \x00 + script_name; \x00 cannot appear in either field.
	executed := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		executed[r.Version+"\x00"+r.ScriptName] = struct{}{}
	}

	pending := make([]script, 0, len(scripts))
	for _, s := range scripts {
		if _, ok := executed[s.version+"\x00"+s.name]; !ok {
			pending = append(pending, s)
		}
	}
	return pending
}

// collapseToSingleLine collapses a SQL statement into a single line
// (string literals are preserved verbatim).
func collapseToSingleLine(sql string) string {
	sql = normalizeLineEndings(sql)
	var buf strings.Builder
	buf.Grow(len(sql))
	i, n := 0, len(sql)
	pendingSpace := false

	writePendingSpace := func() {
		if pendingSpace && buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		pendingSpace = false
	}

	for i < n {
		c := sql[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			writePendingSpace()
			quote := c
			buf.WriteByte(c)
			i++
			for i < n {
				buf.WriteByte(sql[i])
				if sql[i] == '\\' && i+1 < n {
					i++
					buf.WriteByte(sql[i])
					i++
					continue
				}
				if sql[i] == quote {
					if quote != '`' && i+1 < n && sql[i+1] == quote {
						buf.WriteByte(sql[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == ' ' || c == '\t' || c == '\n':
			pendingSpace = true
			i++
		default:
			writePendingSpace()
			buf.WriteByte(c)
			i++
		}
	}

	return strings.TrimSpace(buf.String())
}

// truncateSQL shortens sql to at most limit runes, appending a marker when
// truncation occurs. Rune-based truncation preserves valid UTF-8 output
// when SQL contains multi-byte characters (e.g. Chinese table comments).
func truncateSQL(sql string, limit int) string {
	if limit <= 0 {
		return sql
	}
	runes := []rune(sql)
	if len(runes) <= limit {
		return sql
	}
	return string(runes[:limit]) + "...(truncated)"
}
