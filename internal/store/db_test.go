package store

import (
	"context"
	"path/filepath"
	"testing"
)

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
