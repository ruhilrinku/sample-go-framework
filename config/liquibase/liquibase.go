package liquibase

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

// Run validates the changelogs and then applies incremental changes using pgx.
func Run(pool *pgxpool.Pool, changelogPath string, logger *slog.Logger) error {
	log := logger.With("component", "migrations")
	ctx := context.Background()

	changeSets, err := loadChangeSets(changelogPath)
	if err != nil {
		return fmt.Errorf("loading changelog: %w", err)
	}

	if err := ensureChangelogTable(ctx, pool); err != nil {
		return fmt.Errorf("creating changelog tracking table: %w", err)
	}

	// Validate: check that already-applied changesets have not been modified
	log.Info("validating changelog...")
	if err := validate(ctx, pool, changeSets); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	log.Info("changelog is valid")

	// Update: apply pending changesets
	log.Info("applying incremental changes...")
	applied, err := update(ctx, pool, changeSets, log)
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	if applied == 0 {
		log.Info("already up to date")
	} else {
		log.Info("all changes applied", "count", applied)
	}

	return nil
}

// --- changelog YAML parsing ---

type changelogFile struct {
	DatabaseChangeLog []changelogEntry `yaml:"databaseChangeLog"`
}

type changelogEntry struct {
	ChangeSet *changeSetDef `yaml:"changeSet,omitempty"`
	Include   *includeDef   `yaml:"include,omitempty"`
}

type includeDef struct {
	File                    string `yaml:"file"`
	RelativeToChangelogFile bool   `yaml:"relativeToChangelogFile"`
}

type changeSetDef struct {
	ID      string      `yaml:"id"`
	Author  string      `yaml:"author"`
	Changes []changeMap `yaml:"changes"`
}

type changeMap = map[string]interface{}

// loadChangeSets reads the master changelog and resolves includes.
func loadChangeSets(masterPath string) ([]changeSetDef, error) {
	data, err := os.ReadFile(masterPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", masterPath, err)
	}

	var cl changelogFile
	if err := yaml.Unmarshal(data, &cl); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", masterPath, err)
	}

	var sets []changeSetDef
	baseDir := filepath.Dir(masterPath)

	for _, entry := range cl.DatabaseChangeLog {
		if entry.ChangeSet != nil {
			sets = append(sets, *entry.ChangeSet)
		}
		if entry.Include != nil {
			incPath := entry.Include.File
			if entry.Include.RelativeToChangelogFile {
				incPath = filepath.Join(baseDir, incPath)
			}
			included, err := loadChangeSets(incPath)
			if err != nil {
				return nil, err
			}
			sets = append(sets, included...)
		}
	}

	return sets, nil
}

// --- tracking table ---

func ensureChangelogTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS databasechangelog (
			id          VARCHAR(255) NOT NULL,
			author      VARCHAR(255) NOT NULL,
			checksum    VARCHAR(64)  NOT NULL,
			executed_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			PRIMARY KEY (id, author)
		)
	`)
	return err
}

// --- validate ---

func validate(ctx context.Context, pool *pgxpool.Pool, sets []changeSetDef) error {
	for _, cs := range sets {
		var storedChecksum string
		err := pool.QueryRow(ctx,
			"SELECT checksum FROM databasechangelog WHERE id = $1 AND author = $2",
			cs.ID, cs.Author,
		).Scan(&storedChecksum)

		if err != nil {
			continue // not yet applied — nothing to validate
		}

		currentChecksum := checksum(cs)
		if storedChecksum != currentChecksum {
			return fmt.Errorf("changeset %s/%s was modified after being applied (expected checksum %s, got %s)",
				cs.Author, cs.ID, storedChecksum, currentChecksum)
		}
	}
	return nil
}

// --- update ---

func update(ctx context.Context, pool *pgxpool.Pool, sets []changeSetDef, log *slog.Logger) (int, error) {
	applied := 0
	for _, cs := range sets {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM databasechangelog WHERE id = $1 AND author = $2)",
			cs.ID, cs.Author,
		).Scan(&exists)
		if err != nil {
			return applied, fmt.Errorf("checking changeset %s: %w", cs.ID, err)
		}
		if exists {
			continue
		}

		sql := changesToSQL(cs)
		if sql == "" {
			return applied, fmt.Errorf("changeset %s: unable to generate SQL", cs.ID)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("beginning tx for %s: %w", cs.ID, err)
		}

		if _, err := tx.Exec(ctx, sql); err != nil {
			tx.Rollback(ctx)
			return applied, fmt.Errorf("executing changeset %s: %w", cs.ID, err)
		}

		if _, err := tx.Exec(ctx,
			"INSERT INTO databasechangelog (id, author, checksum) VALUES ($1, $2, $3)",
			cs.ID, cs.Author, checksum(cs),
		); err != nil {
			tx.Rollback(ctx)
			return applied, fmt.Errorf("recording changeset %s: %w", cs.ID, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("committing changeset %s: %w", cs.ID, err)
		}

		log.Info("applied changeset", "author", cs.Author, "id", cs.ID)
		applied++
	}
	return applied, nil
}

// --- SQL generation from YAML change types ---

func changesToSQL(cs changeSetDef) string {
	var parts []string
	for _, change := range cs.Changes {
		if ct, ok := change["createTable"]; ok {
			parts = append(parts, generateCreateTable(ct))
		}
		if ci, ok := change["createIndex"]; ok {
			parts = append(parts, generateCreateIndex(ci))
		}
		if uc, ok := change["addUniqueConstraint"]; ok {
			parts = append(parts, generateAddUniqueConstraint(uc))
		}
	}
	return strings.Join(parts, "\n")
}

func generateCreateTable(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	tableName := str(m["tableName"])
	cols, _ := m["columns"].([]interface{})

	var colDefs []string
	var pkCols []string

	for _, c := range cols {
		cm, _ := c.(map[string]interface{})
		colMap, _ := cm["column"].(map[string]interface{})

		name := str(colMap["name"])
		colType := str(colMap["type"])
		def := name + " " + colType

		if dv, ok := colMap["defaultValueComputed"]; ok {
			def += " DEFAULT " + str(dv)
		} else if dv, ok := colMap["defaultValue"]; ok {
			def += " DEFAULT '" + str(dv) + "'"
		}

		if cons, ok := colMap["constraints"].(map[string]interface{}); ok {
			if cons["nullable"] == false {
				def += " NOT NULL"
			}
			if cons["primaryKey"] == true {
				pkCols = append(pkCols, name)
			}
		}

		colDefs = append(colDefs, def)
	}

	if len(pkCols) > 0 {
		colDefs = append(colDefs, "PRIMARY KEY ("+strings.Join(pkCols, ", ")+")")
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n);",
		tableName, strings.Join(colDefs, ",\n  "))
}

func generateCreateIndex(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	indexName := str(m["indexName"])
	tableName := str(m["tableName"])
	cols, _ := m["columns"].([]interface{})

	var colExprs []string
	for _, c := range cols {
		cm, _ := c.(map[string]interface{})
		colMap, _ := cm["column"].(map[string]interface{})
		expr := str(colMap["name"])
		if colMap["descending"] == true {
			expr += " DESC"
		}
		colExprs = append(colExprs, expr)
	}

	return fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s);",
		indexName, tableName, strings.Join(colExprs, ", "))
}

func generateAddUniqueConstraint(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	constraintName := str(m["constraintName"])
	tableName := str(m["tableName"])
	columnNames := str(m["columnNames"])

	return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);",
		tableName, constraintName, columnNames)
}

func checksum(cs changeSetDef) string {
	data, _ := yaml.Marshal(cs)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
