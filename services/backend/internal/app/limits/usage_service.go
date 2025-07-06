package limits

import (
	"backend/internal/data"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// UsageType represents different types of resource usage
type UsageType string

const (
	UsageTypeCredits       UsageType = "credits"
	UsageTypeAlert         UsageType = "alert"
	UsageTypeStrategyAlert UsageType = "strategy_alert"
	UsageTypeBacktest      UsageType = "backtest"
)

// UserUsage represents the current credits and usage for a user
type UserUsage struct {
	UserID                       int       `json:"user_id"`
	SubscriptionCreditsRemaining int       `json:"subscription_credits_remaining"`
	PurchasedCreditsRemaining    int       `json:"purchased_credits_remaining"`
	TotalCreditsRemaining        int       `json:"total_credits_remaining"`
	SubscriptionCreditsAllocated int       `json:"subscription_credits_allocated"`
	ActiveAlerts                 int       `json:"active_alerts"`
	AlertsLimit                  int       `json:"alerts_limit"`
	ActiveStrategyAlerts         int       `json:"active_strategy_alerts"`
	StrategyAlertsLimit          int       `json:"strategy_alerts_limit"`
	CurrentPeriodStart           time.Time `json:"current_period_start"`
	LastLimitReset               time.Time `json:"last_limit_reset"`
	PlanName                     string    `json:"plan_name"`
	SubscriptionStatus           string    `json:"subscription_status"`
}

// CreditConsumptionResult represents the result of consuming credits
type CreditConsumptionResult struct {
	Success          bool   `json:"success"`
	RemainingCredits int    `json:"remaining_credits"`
	SourceUsed       string `json:"source_used"`
}

// CheckUsageAllowed checks if a user can perform a specific action based on their credits
func CheckUsageAllowed(conn *data.Conn, userID int, usageType UsageType, creditsRequired int) (bool, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var subscriptionCredits, purchasedCredits, totalCredits int
	var activeAlerts, alertsLimit, activeStrategyAlerts, strategyAlertsLimit int

	query := `
		SELECT 
			COALESCE(subscription_credits_remaining, 0),
			COALESCE(purchased_credits_remaining, 0),
			COALESCE(total_credits_remaining, 0),
			COALESCE(active_alerts, 0),
			COALESCE(alerts_limit, 0),
			COALESCE(active_strategy_alerts, 0),
			COALESCE(strategy_alerts_limit, 0)
		FROM users 
		WHERE userId = $1`

	err := conn.DB.QueryRow(ctx, query, userID).Scan(
		&subscriptionCredits, &purchasedCredits, &totalCredits,
		&activeAlerts, &alertsLimit,
		&activeStrategyAlerts, &strategyAlertsLimit,
	)

	if err != nil {
		return false, 0, fmt.Errorf("error checking user credits: %v", err)
	}

	// Check specific usage type
	switch usageType {
	case UsageTypeCredits:
		// For queries, check if user has enough credits for the required amount
		return totalCredits >= creditsRequired, totalCredits, nil
	case UsageTypeAlert:
		remaining := alertsLimit - activeAlerts
		return activeAlerts < alertsLimit, remaining, nil
	case UsageTypeStrategyAlert:
		remaining := strategyAlertsLimit - activeStrategyAlerts
		return activeStrategyAlerts < strategyAlertsLimit, remaining, nil
	default:
		return true, -1, nil
	}
}

// RecordUsage records usage of a resource and updates the user's usage counters
func RecordUsage(conn *data.Conn, userID int, usageType UsageType, resourceConsumed int, metadata map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start transaction
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error starting transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get current plan name for logging
	var planName sql.NullString
	err = tx.QueryRow(ctx, "SELECT subscription_plan FROM users WHERE userId = $1", userID).Scan(&planName)
	if err != nil {
		return fmt.Errorf("error getting user plan: %v", err)
	}

	currentPlan := "Free"
	if planName.Valid {
		currentPlan = planName.String
	}

	// Handle different usage types
	switch usageType {
	case UsageTypeCredits:
		// For queries, consume credits directly in Go code
		var currentSubscriptionCredits, currentPurchasedCredits int

		// Get current credit balances
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(subscription_credits_remaining, 0), COALESCE(purchased_credits_remaining, 0)
			FROM users WHERE userId = $1`, userID).Scan(&currentSubscriptionCredits, &currentPurchasedCredits)
		if err != nil {
			return fmt.Errorf("error getting current credit balances: %v", err)
		}

		// Check if user has enough total credits
		totalCredits := currentSubscriptionCredits + currentPurchasedCredits
		if totalCredits < resourceConsumed {
			return fmt.Errorf("insufficient credits")
		}

		// Calculate how many credits to consume from each source
		var creditsFromSubscription, creditsFromPurchased int
		if currentSubscriptionCredits >= resourceConsumed {
			creditsFromSubscription = resourceConsumed
			creditsFromPurchased = 0
		} else {
			creditsFromSubscription = currentSubscriptionCredits
			creditsFromPurchased = resourceConsumed - currentSubscriptionCredits
		}

		// Update user credits
		_, err = tx.Exec(ctx, `
			UPDATE users SET 
				subscription_credits_remaining = subscription_credits_remaining - $2,
				purchased_credits_remaining = purchased_credits_remaining - $3
			WHERE userId = $1`,
			userID, creditsFromSubscription, creditsFromPurchased)
		if err != nil {
			return fmt.Errorf("error updating user credits: %v", err)
		}

		// Determine source used for logging
		var sourceUsed string
		if creditsFromPurchased > 0 {
			sourceUsed = "both"
		} else {
			sourceUsed = "subscription"
		}

		// Log the usage with credit consumption details
		metadataJSON, _ := json.Marshal(metadata)
		_, err = tx.Exec(ctx, `
			INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata, credits_consumed, credits_source)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			userID, string(usageType), resourceConsumed, currentPlan, metadataJSON, resourceConsumed, sourceUsed)

	case UsageTypeAlert:
		// For alerts, update the counter
		_, err = tx.Exec(ctx, "UPDATE users SET active_alerts = active_alerts + $2 WHERE userId = $1", userID, resourceConsumed)
		if err != nil {
			return fmt.Errorf("error updating active alerts counter: %v", err)
		}

		// Log the usage
		metadataJSON, _ := json.Marshal(metadata)
		_, err = tx.Exec(ctx, `
			INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, string(usageType), resourceConsumed, currentPlan, metadataJSON)

	case UsageTypeStrategyAlert:
		// For strategy alerts, update the counter
		_, err = tx.Exec(ctx, "UPDATE users SET active_strategy_alerts = active_strategy_alerts + $2 WHERE userId = $1", userID, resourceConsumed)
		if err != nil {
			return fmt.Errorf("error updating active strategy alerts counter: %v", err)
		}

		// Log the usage
		metadataJSON, _ := json.Marshal(metadata)
		_, err = tx.Exec(ctx, `
			INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, string(usageType), resourceConsumed, currentPlan, metadataJSON)

	case UsageTypeBacktest:
		// For backtests, just log usage without consuming credits
		metadataJSON, _ := json.Marshal(metadata)
		_, err = tx.Exec(ctx, `
			INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, string(usageType), resourceConsumed, currentPlan, metadataJSON)

	default:
		// For other usage types, just log
		metadataJSON, _ := json.Marshal(metadata)
		_, err = tx.Exec(ctx, `
			INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
			VALUES ($1, $2, $3, $4, $5)`,
			userID, string(usageType), resourceConsumed, currentPlan, metadataJSON)
	}

	if err != nil {
		return fmt.Errorf("error logging usage: %v", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("error committing usage transaction: %v", err)
	}

	return nil
}

// ResetUserSubscriptionCredits resets subscription credits for a specific user when their billing period renews
// This function is designed to be called from Stripe webhooks
func ResetUserSubscriptionCredits(conn *data.Conn, userID int, planName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the credits for this plan
	var creditsPerPeriod int
	err := conn.DB.QueryRow(ctx, `
		SELECT credits_per_billing_period 
		FROM plan_limits 
		WHERE plan_name = $1`, planName).Scan(&creditsPerPeriod)

	if err != nil {
		return fmt.Errorf("plan '%s' not found in plan_limits table", planName)
	}

	// Reset subscription credits but keep purchased credits
	_, err = conn.DB.Exec(ctx, `
		UPDATE users SET 
			subscription_credits_remaining = $2,
			subscription_credits_allocated = $2,
			current_period_start = CURRENT_TIMESTAMP,
			last_limit_reset = CURRENT_TIMESTAMP
		WHERE userId = $1`,
		userID, creditsPerPeriod)

	if err != nil {
		return fmt.Errorf("error resetting user subscription credits: %v", err)
	}

	// Log the reset action
	metadata := map[string]interface{}{
		"reset_reason":      "billing_cycle_webhook",
		"credits_allocated": creditsPerPeriod,
		"plan_name":         planName,
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err = conn.DB.Exec(ctx, `
		INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
		VALUES ($1, 'credits_reset', 0, $2, $3)`,
		userID, planName, metadataJSON)

	if err != nil {
		log.Printf("Warning: Failed to log credit reset for user %d: %v", userID, err)
	}

	log.Printf("Reset subscription credits for user %d (plan: %s, credits: %d)", userID, planName, creditsPerPeriod)
	return nil
}

// GetUserUsageStats returns usage statistics for a user
func GetUserUsageStats(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var usage UserUsage
	var subscriptionPlan sql.NullString

	query := `
		SELECT 
			userId,
			COALESCE(subscription_credits_remaining, 0) as subscription_credits_remaining,
			COALESCE(purchased_credits_remaining, 0) as purchased_credits_remaining,
			COALESCE(total_credits_remaining, 0) as total_credits_remaining,
			COALESCE(subscription_credits_allocated, 0) as subscription_credits_allocated,
			COALESCE(active_alerts, 0) as active_alerts,
			COALESCE(alerts_limit, 0) as alerts_limit,
			COALESCE(active_strategy_alerts, 0) as active_strategy_alerts,
			COALESCE(strategy_alerts_limit, 0) as strategy_alerts_limit,
			COALESCE(current_period_start, CURRENT_TIMESTAMP) as current_period_start,
			COALESCE(last_limit_reset, CURRENT_TIMESTAMP) as last_limit_reset,
			subscription_plan,
			COALESCE(subscription_status, 'inactive') as subscription_status
		FROM users 
		WHERE userId = $1`

	err := conn.DB.QueryRow(ctx, query, userID).Scan(
		&usage.UserID,
		&usage.SubscriptionCreditsRemaining,
		&usage.PurchasedCreditsRemaining,
		&usage.TotalCreditsRemaining,
		&usage.SubscriptionCreditsAllocated,
		&usage.ActiveAlerts,
		&usage.AlertsLimit,
		&usage.ActiveStrategyAlerts,
		&usage.StrategyAlertsLimit,
		&usage.CurrentPeriodStart,
		&usage.LastLimitReset,
		&subscriptionPlan,
		&usage.SubscriptionStatus,
	)

	if err != nil {
		return nil, fmt.Errorf("error retrieving user usage: %v", err)
	}

	// Set plan name
	if subscriptionPlan.Valid {
		usage.PlanName = subscriptionPlan.String
	} else {
		usage.PlanName = "Free"
	}

	return usage, nil
}

// UpdateUserCreditsForPlan updates a user's credit allocation based on their subscription plan
func UpdateUserCreditsForPlan(conn *data.Conn, userID int, planName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the credits for the specified plan
	var creditsPerPeriod, alertsLimit, strategyAlertsLimit int

	query := `
		SELECT credits_per_billing_period, alerts_limit, strategy_alerts_limit
		FROM plan_limits 
		WHERE plan_name = $1`

	err := conn.DB.QueryRow(ctx, query, planName).Scan(
		&creditsPerPeriod, &alertsLimit, &strategyAlertsLimit,
	)

	if err != nil {
		// If plan not found, use default Free plan limits
		if planName == "" || planName == "Free" {
			creditsPerPeriod = 5
			alertsLimit = 0
			strategyAlertsLimit = 0
		} else {
			return fmt.Errorf("plan '%s' not found in plan_limits table", planName)
		}
	}

	// Update the user's credit allocation and limits
	updateQuery := `
		UPDATE users SET 
			subscription_credits_remaining = $2,
			subscription_credits_allocated = $2,
			alerts_limit = $3,
			strategy_alerts_limit = $4
		WHERE userId = $1`

	_, err = conn.DB.Exec(ctx, updateQuery,
		userID, creditsPerPeriod, alertsLimit, strategyAlertsLimit)

	if err != nil {
		return fmt.Errorf("error updating user credits: %v", err)
	}

	log.Printf("Updated credits for user %d to plan '%s': credits=%d, alerts=%d, strategy_alerts=%d",
		userID, planName, creditsPerPeriod, alertsLimit, strategyAlertsLimit)

	return nil
}

// AddPurchasedCredits adds purchased credits to a user's account
func AddPurchasedCredits(conn *data.Conn, userID int, creditsToAdd int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Add purchased credits to the user's account
	_, err := conn.DB.Exec(ctx, `
		UPDATE users SET 
			purchased_credits_remaining = purchased_credits_remaining + $2
		WHERE userId = $1`,
		userID, creditsToAdd)

	if err != nil {
		return fmt.Errorf("error adding purchased credits: %v", err)
	}

	// Log the credit purchase
	metadata := map[string]interface{}{
		"credits_added": creditsToAdd,
		"purchase_type": "manual_addition",
	}
	metadataJSON, _ := json.Marshal(metadata)

	_, err = conn.DB.Exec(ctx, `
		INSERT INTO usage_logs (user_id, usage_type, resource_consumed, plan_name, metadata)
		VALUES ($1, 'credits_purchase', $2, 'N/A', $3)`,
		userID, creditsToAdd, metadataJSON)

	if err != nil {
		log.Printf("Warning: Failed to log credit purchase for user %d: %v", userID, err)
	}

	log.Printf("Added %d purchased credits to user %d", creditsToAdd, userID)
	return nil
}

// DecrementActiveAlerts decrements the active alerts counter when an alert is removed
func DecrementActiveAlerts(conn *data.Conn, userID int, alertsToRemove int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := conn.DB.Exec(ctx, `
		UPDATE users SET 
			active_alerts = GREATEST(0, active_alerts - $2)
		WHERE userId = $1`,
		userID, alertsToRemove)

	if err != nil {
		return fmt.Errorf("error decrementing active alerts: %v", err)
	}

	return nil
}

// DecrementActiveStrategyAlerts decrements the active strategy alerts counter when a strategy alert is removed
func DecrementActiveStrategyAlerts(conn *data.Conn, userID int, alertsToRemove int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := conn.DB.Exec(ctx, `
		UPDATE users SET 
			active_strategy_alerts = GREATEST(0, active_strategy_alerts - $2)
		WHERE userId = $1`,
		userID, alertsToRemove)

	if err != nil {
		return fmt.Errorf("error decrementing active strategy alerts: %v", err)
	}

	return nil
}
