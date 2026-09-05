package database

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func openUnmigratedTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func applyMigrationsThrough(t *testing.T, db *DB, version int) {
	t.Helper()
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create migrations table: %v", err)
	}
	for _, migration := range migrations {
		if migration.Version > version {
			break
		}
		if err := db.applyMigration(migration); err != nil {
			t.Fatalf("apply migration %d: %v", migration.Version, err)
		}
	}
}

func querySignature(t *testing.T, db *DB, query string) []string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query signature: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("signature columns: %v", err)
	}
	var result []string
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err := rows.Scan(targets...); err != nil {
			t.Fatalf("scan signature: %v", err)
		}
		result = append(result, fmt.Sprint(values...))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("signature rows: %v", err)
	}
	return result
}

func schemaSignature(t *testing.T, db *DB) []string {
	t.Helper()
	result := querySignature(t, db, `
		SELECT type, name, tbl_name, COALESCE(sql, '')
		FROM sqlite_master
		WHERE type IN ('table', 'index', 'trigger', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY type, name
	`)
	tables := querySignature(t, db, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	for _, table := range tables {
		result = append(result, "table_info:"+table)
		result = append(result, querySignature(t, db, fmt.Sprintf("PRAGMA table_info(%q)", table))...)
		result = append(result, "index_list:"+table)
		result = append(result, querySignature(t, db, fmt.Sprintf("PRAGMA index_list(%q)", table))...)
		result = append(result, "foreign_key_list:"+table)
		result = append(result, querySignature(t, db, fmt.Sprintf("PRAGMA foreign_key_list(%q)", table))...)
	}
	return result
}

func assertDatabaseIntegrity(t *testing.T, db *DB) {
	t.Helper()
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check returned a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("foreign_key_check rows: %v", err)
	}

	var integrity string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil {
		t.Fatalf("integrity_check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity_check = %q, want ok", integrity)
	}
}

func TestMigrationUpgradePathsMatchFreshSchema(t *testing.T) {
	fresh := openUnmigratedTestDB(t)
	if err := fresh.Migrate(); err != nil {
		t.Fatalf("fresh Migrate: %v", err)
	}
	freshSchema := schemaSignature(t, fresh)
	assertDatabaseIntegrity(t, fresh)

	for _, startVersion := range []int{31, 42} {
		t.Run(fmt.Sprintf("from_v%d", startVersion), func(t *testing.T) {
			db := openUnmigratedTestDB(t)
			applyMigrationsThrough(t, db, startVersion)
			if err := db.Migrate(); err != nil {
				t.Fatalf("upgrade from v%d: %v", startVersion, err)
			}
			if got := schemaSignature(t, db); !reflect.DeepEqual(got, freshSchema) {
				t.Fatalf("schema after upgrade from v%d differs from fresh install: %s", startVersion, firstSchemaDifference(freshSchema, got))
			}
			assertDatabaseIntegrity(t, db)

			if err := db.Migrate(); err != nil {
				t.Fatalf("repeat Migrate after v%d upgrade: %v", startVersion, err)
			}
			if got := schemaSignature(t, db); !reflect.DeepEqual(got, freshSchema) {
				t.Fatalf("schema changed after repeat Migrate from v%d upgrade: %s", startVersion, firstSchemaDifference(freshSchema, got))
			}
		})
	}
}

func TestFailedMigrationDoesNotRecordVersion(t *testing.T) {
	db := openUnmigratedTestDB(t)
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		t.Fatalf("create migrations table: %v", err)
	}

	failed := Migration{Version: 999, SQL: `THIS IS NOT VALID SQL`}
	if err := db.applyMigration(failed); err == nil {
		t.Fatal("applyMigration succeeded, want SQL error")
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = ?`, failed.Version).Scan(&count); err != nil {
		t.Fatalf("count failed migration marker: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed migration marker count = %d, want 0", count)
	}
}

func firstSchemaDifference(want, got []string) string {
	limit := len(want)
	if len(got) < limit {
		limit = len(got)
	}
	for index := 0; index < limit; index++ {
		if want[index] != got[index] {
			return fmt.Sprintf("index %d: fresh=%q upgrade=%q", index, want[index], got[index])
		}
	}
	return fmt.Sprintf("entry count: fresh=%d upgrade=%d", len(want), len(got))
}
