package subscriptions

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// UpdateYearlySubscriptionCredits grants the monthly credit allotment to users on yearly
// subscription plans. It should be called once per day by the scheduler.
//
// Logic:
//  1. Identify all users with an *active* Stripe subscription whose plan's
//     billing period is `year` (looked-up via the prices table).
//  2. For each user, check if at least one calendar month has elapsed since
//     their last_limit_reset timestamp (or if last_limit_reset is NULL).
//  3. When the interval has elapsed, call ResetUserSubscriptionCredits which
//     resets both `subscription_credits_remaining` and
//     `subscription_credits_allocated` to the per-period amount and updates
//     last_limit_reset/current_period_start.
//
// The daily scheduler ensures we never miss a month boundary, while the date
// check guarantees we do not allocate credits more than once per period even
// if the job runs multiple times (e.g. on server restart).
func UpdateYearlySubscriptionCredits(conn *data.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Select active users on yearly billing plans along with their last reset.
	// We rely on prices.billing_period = 'yearly' to detect yearly plans.
	rows, err := conn.DB.Query(ctx, `
		SELECT u.userId,
		       u.subscription_plan,
		       COALESCE(u.last_limit_reset, u.current_period_start) AS last_reset,
		       sp.queries_limit
		FROM users u
		JOIN subscription_products sp ON sp.product_key = u.subscription_plan
		JOIN prices p ON sp.product_key = p.product_key
		WHERE u.subscription_status = 'active'
		  AND p.billing_period = 'yearly'`)
	if err != nil {
		return fmt.Errorf("querying yearly subscription users: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	processed := 0

	for rows.Next() {
		var userID int
		var productKey string
		var lastReset time.Time
		var creditsPerMonth int
		if err := rows.Scan(&userID, &productKey, &lastReset, &creditsPerMonth); err != nil {
			log.Printf("❌ Error scanning yearly subscription user row: %v", err)
			continue
		}

		// Compute if at least one full month has elapsed since last reset.
		if now.After(lastReset.AddDate(0, 1, 0)) {
			// Perform credit reset without touching current_period_start
			_, err := data.ExecWithRetry(ctx, conn.DB, `
				UPDATE users SET
					subscription_credits_remaining = $2,
					subscription_credits_allocated = $2,
					last_limit_reset              = CURRENT_TIMESTAMP
				WHERE userId = $1`,
				userID, creditsPerMonth)
			if err != nil {
				log.Printf("❌ Error updating credits for user %d (product %s): %v", userID, productKey, err)
				continue
			}

			// Record in usage_logs for audit
			metadata := map[string]interface{}{
				"reset_reason":      "yearly_plan_monthly_allocation",
				"credits_allocated": creditsPerMonth,
				"product_key":       productKey,
			}
			if metaJSON, err := json.Marshal(metadata); err == nil {
				_, _ = data.ExecWithRetry(ctx, conn.DB, `
					INSERT INTO usage_logs (userId, usage_type, resource_consumed, plan_name, metadata)
					VALUES ($1, 'credits_reset', 0, $2, $3)`,
					userID, productKey, metaJSON)
			}

			processed++
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating yearly subscription rows: %w", err)
	}

	log.Printf("✅ Yearly subscription credit update completed – processed %d users", processed)
	return nil
}
