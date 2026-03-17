package storage

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
)

func ConnectWithRetry(ctx context.Context, dsn string, retries int, delay time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	closeWithErr := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, err
	}

	var lastErr error
	for i := 1; i <= retries; i++ {
		if err := ctx.Err(); err != nil {
			return closeWithErr(err)
		}

		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}

		lastErr = err
		log.Printf("db not ready yet (%d/%d): %v", i, retries, err)

		if i < retries {
			select {
			case <-ctx.Done():
				return closeWithErr(ctx.Err())
			case <-time.After(delay):
			}
		}
	}

	return closeWithErr(lastErr)
}
