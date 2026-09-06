package store

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentBackupsNeverOverwriteDestination(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	destination := filepath.Join(t.TempDir(), "backup.db")
	start := make(chan struct{})
	results := make(chan error, 8)
	var workers sync.WaitGroup
	for range cap(results) {
		workers.Go(func() {
			<-start
			results <- BackupSQLite(context.Background(), db, destination)
		})
	}
	close(start)
	workers.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected exactly one published backup, got %d", succeeded)
	}
}

func TestSQLiteMaintenanceAndBackup(t *testing.T) {
	db := testDB(t)
	defer db.Close()
	ctx := context.Background()
	if err := CheckIntegrity(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckpointWAL(ctx, db); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(t.TempDir(), "backup.db")
	if err := BackupSQLite(ctx, db, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("backup is empty")
	}
	if err := BackupSQLite(ctx, db, destination); err == nil {
		t.Fatal("expected existing backup destination to be rejected")
	}

	backupDB, err := OpenSQLite(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer backupDB.Close()
	if err := CheckIntegrity(ctx, backupDB); err != nil {
		t.Fatal(err)
	}
}
