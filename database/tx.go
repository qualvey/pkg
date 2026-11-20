package database

import (
	"context"
	"database/sql"
	"fmt"
)

// WithTx 在标准库事务闭包中执行业务逻辑。
// 特性：自动 Rollback、成功自动 Commit、并在遇到 panic 时优先 Rollback 释放数据库连接与事务锁，而后再继续抛出 panic。
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin tx failed: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			if commitErr := tx.Commit(); commitErr != nil {
				err = fmt.Errorf("database: commit tx failed: %w", commitErr)
			}
		}
	}()

	return fn(tx)
}
