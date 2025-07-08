package data

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
)

// ExecWithRetry executes a SQL statement with an exponential-backoff retry strategy.
// It is meant for transient network/database errors such as unexpected EOF.
// The function retries up to maxAttempts before giving up and returning the last error.
// A cancelled context immediately aborts further retries.
func ExecWithRetry(ctx context.Context, db *pgxpool.Pool, query string, args ...interface{}) (pgconn.CommandTag, error) {
	const maxAttempts = 5
	var backoff = 500 * time.Millisecond

	var tag pgconn.CommandTag
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		tag, err = db.Exec(ctx, query, args...)
		if err == nil {
			return tag, nil
		}

		// Abort retries for non-transient errors such as undefined column (SQLSTATE 42703).
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "42703" {
				// Undefined column – retrying won't help.
				return tag, err
			}
		}

		// Abort early if the context has been cancelled.
		if ctx.Err() != nil {
			return tag, ctx.Err()
		}

		log.Printf("Exec failed (attempt %d/%d): %v", attempt, maxAttempts, err)
		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2 // exponential back-off
		}
	}
	return tag, err
}
