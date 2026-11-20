package database

import (
	"database/sql"
	"fmt"
	"time"
)

// PostgresPoolConfig PostgreSQL 连接池调优参数
type PostgresPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultPostgresPoolConfig 生产推荐默认连接池参数
func DefaultPostgresPoolConfig() PostgresPoolConfig {
	return PostgresPoolConfig{
		MaxOpenConns:    20,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// TunePostgres 为标准库 *sql.DB 应用 PostgreSQL 优化连接池设置
func TunePostgres(db *sql.DB, cfg ...PostgresPoolConfig) {
	c := DefaultPostgresPoolConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}

	if c.MaxOpenConns > 0 {
		db.SetMaxOpenConns(c.MaxOpenConns)
	}
	if c.MaxIdleConns > 0 {
		db.SetMaxIdleConns(c.MaxIdleConns)
	}
	if c.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(c.ConnMaxLifetime)
	}
	if c.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(c.ConnMaxIdleTime)
	}
}

// SQLiteWALConfig SQLite WAL 与并发模式调优参数
type SQLiteWALConfig struct {
	BusyTimeout time.Duration
}

// DefaultSQLiteWALConfig 默认 SQLite WAL 调优参数
func DefaultSQLiteWALConfig() SQLiteWALConfig {
	return SQLiteWALConfig{
		BusyTimeout: 5 * time.Second,
	}
}

// TuneSQLiteWAL 为标准库 *sql.DB 应用 SQLite WAL 模式、防死锁超时与单一写入连接池限制
func TuneSQLiteWAL(db *sql.DB, cfg ...SQLiteWALConfig) error {
	c := DefaultSQLiteWALConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}

	// 1. 开启 WAL 预写日志模式
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		return fmt.Errorf("database: enable sqlite WAL failed: %w", err)
	}

	// 2. 设置 busy_timeout 防止高频读写时立即抛出 database is locked 异常
	timeoutMs := int(c.BusyTimeout.Milliseconds())
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d;", timeoutMs)); err != nil {
		return fmt.Errorf("database: set sqlite busy_timeout failed: %w", err)
	}

	// 3. 严格限制连接池，SQLite 单文件写并发必须为 1，避免锁竞争冲突
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return nil
}
