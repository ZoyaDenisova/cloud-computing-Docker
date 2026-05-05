package storage

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func ConnectWithRetry(ctx context.Context, dsn string, attempts int, delay time.Duration) (*sql.DB, error) {
	var lastErr error

	for i := 1; i <= attempts; i++ {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			lastErr = err
			log.Printf("db open attempt %d/%d failed: %v", i, attempts, err)
		} else {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			if err == nil {
				return db, nil
			}
			lastErr = err
			_ = db.Close()
			log.Printf("db ping attempt %d/%d failed: %v", i, attempts, err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	return nil, lastErr
}
