package data

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
)

// isConnectionError checks if the error is related to database connectivity issues
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for PostgreSQL connection-related error codes
	if pgErr, ok := err.(*pgconn.PgError); ok {
		// Connection-related SQLSTATE classes:
		// 08xxx - Connection Exception
		// 57P01 - Admin Shutdown
		// 57P02 - Crash Shutdown
		// 57P03 - Cannot Connect Now
		sqlState := pgErr.Code
		return strings.HasPrefix(sqlState, "08") ||
			sqlState == "57P01" ||
			sqlState == "57P02" ||
			sqlState == "57P03"
	}

	// Check for common connection error strings
	errStr := strings.ToLower(err.Error())
	connectionKeywords := []string{
		"connection refused",
		"connection reset",
		"connection closed",
		"unexpected eof",
		"broken pipe",
		"no such host",
		"network is unreachable",
		"timeout",
		"connection lost",
		"server closed the connection",
	}

	for _, keyword := range connectionKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// ExecWithRetry executes a SQL statement with an exponential-backoff retry strategy.
// It is meant for transient network/database errors such as unexpected EOF.
// The function retries up to maxAttempts before giving up and returning the last error.
// A cancelled context immediately aborts further retries.
// Connection errors get extended retry attempts with longer backoff periods.
func ExecWithRetry(ctx context.Context, db *pgxpool.Pool, query string, args ...interface{}) (pgconn.CommandTag, error) {
	const maxAttempts = 5
	const maxConnectionAttempts = 10 // Extended attempts for connection errors
	var backoff = 500 * time.Millisecond

	var tag pgconn.CommandTag
	var err error

	for attempt := 1; attempt <= maxConnectionAttempts; attempt++ {
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

		// Determine if this is a connection error and set retry limits accordingly
		isConnErr := isConnectionError(err)
		maxAttemptsForThisError := maxAttempts
		if isConnErr {
			maxAttemptsForThisError = maxConnectionAttempts
		}

		// Stop retrying if we've exceeded the limit for this error type
		if attempt >= maxAttemptsForThisError {
			break
		}

		log.Printf("Exec failed (attempt %d/%d): %v", attempt, maxAttemptsForThisError, err)

		// Use longer backoff for connection errors
		currentBackoff := backoff
		if isConnErr && attempt > maxAttempts {
			// For connection errors beyond normal attempts, use longer backoff
			currentBackoff = backoff * 3
		}

		time.Sleep(currentBackoff)
		backoff *= 2 // exponential back-off

		// Cap backoff at reasonable maximum
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
	return tag, err
}
