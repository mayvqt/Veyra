package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteRestrictsDatabasePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "veyra.db")
	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("database permissions = %o, want 600", got)
	}
}

func TestOpenSQLiteEnforcesForeignKeys(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "veyra.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := InitSchema(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id_hash, user_id, expires_at, created_at, last_seen_at) VALUES ('orphan', 999, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err == nil {
		t.Fatal("expected foreign-key violation for orphaned session")
	}
}

func TestMigrateInitializesFreshSchemaAndIsRepeatable(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "veyra.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := InitSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(ctx, db); err != nil {
		t.Fatalf("second schema initialization failed: %v", err)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'users'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "users" {
		t.Fatalf("expected users table, got %q", name)
	}
}

func TestInitSchemaRecordsCurrentMigrationVersion(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != len(schemaMigrations) {
		t.Fatalf("schema version = %d, want %d", version, len(schemaMigrations))
	}
}

func TestInitSchemaRejectsNewerDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	if err := InitSchema(context.Background(), db); err == nil {
		t.Fatal("expected newer database schema to be rejected")
	}
}
