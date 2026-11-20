package database

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);")
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	return db
}

func TestDatabase_TuneSQLiteWAL(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	err := TuneSQLiteWAL(db, SQLiteWALConfig{BusyTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("TuneSQLiteWAL failed: %v", err)
	}

	// 验证 WAL 模式已开启
	var journalMode string
	err = db.QueryRow("PRAGMA journal_mode;").Scan(&journalMode)
	if err != nil {
		t.Fatalf("query journal_mode failed: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("expected journal_mode=wal, got %s", journalMode)
	}
}

func TestDatabase_WithTx_Commit(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	err := WithTx(ctx, db, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO users (id, name) VALUES (1, 'Alice');")
		return err
	})
	if err != nil {
		t.Fatalf("WithTx failed: %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM users;").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user committed, got %d", count)
	}
}

func TestDatabase_WithTx_RollbackOnError(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	errExpected := errors.New("something failed")

	err := WithTx(ctx, db, func(tx *sql.Tx) error {
		_, _ = tx.Exec("INSERT INTO users (id, name) VALUES (2, 'Bob');")
		return errExpected
	})

	if !errors.Is(err, errExpected) {
		t.Fatalf("expected errExpected, got %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE id = 2;").Scan(&count)
	if count != 0 {
		t.Errorf("expected rollback with 0 users, got %d", count)
	}
}

func TestDatabase_WithTx_RollbackOnPanic(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()

	defer func() {
		p := recover()
		if p == nil {
			t.Fatalf("expected panic to be propagated")
		}

		// 检查 panic 后数据是否确实被 rollback 了
		var count int
		_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE id = 3;").Scan(&count)
		if count != 0 {
			t.Errorf("expected rollback on panic, got count=%d", count)
		}
	}()

	_ = WithTx(ctx, db, func(tx *sql.Tx) error {
		_, _ = tx.Exec("INSERT INTO users (id, name) VALUES (3, 'Charlie');")
		panic("database panic trigger!")
	})
}
