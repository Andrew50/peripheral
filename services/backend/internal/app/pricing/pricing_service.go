package pricing

import (
	"backend/internal/data"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// SubscriptionPlan represents a subscription plan configuration
type SubscriptionPlan struct {
	ID                      int      `json:"id"`
	PlanName                string   `json:"plan_name"`
	StripePriceIDTest       *string  `json:"stripe_price_id_test"`
	StripePriceIDLive       *string  `json:"stripe_price_id_live"`
	DisplayName             string   `json:"display_name"`
	Description             *string  `json:"description"`
	PriceCents              int      `json:"price_cents"`
	BillingPeriod           string   `json:"billing_period"`
	CreditsPerBillingPeriod int      `json:"credits_per_billing_period"`
	AlertsLimit             int      `json:"alerts_limit"`
	StrategyAlertsLimit     int      `json:"strategy_alerts_limit"`
	Features                []string `json:"features"`
	IsActive                bool     `json:"is_active"`
	IsPopular               bool     `json:"is_popular"`
	SortOrder               int      `json:"sort_order"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// CreditProduct represents a credit product configuration
type CreditProduct struct {
	ID                int      `json:"id"`
	ProductKey        string   `json:"product_key"`
	StripePriceIDTest *string  `json:"stripe_price_id_test"`
	StripePriceIDLive *string  `json:"stripe_price_id_live"`
	DisplayName       string   `json:"display_name"`
	Description       *string  `json:"description"`
	CreditAmount      int      `json:"credit_amount"`
	PriceCents        int      `json:"price_cents"`
	IsActive          bool     `json:"is_active"`
	IsPopular         bool     `json:"is_popular"`
	SortOrder         int      `json:"sort_order"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// GetSubscriptionPlans retrieves all active subscription plans
func GetSubscriptionPlans(conn *data.Conn) ([]SubscriptionPlan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT 
			id, plan_name, stripe_price_id_test, stripe_price_id_live, 
			display_name, description, price_cents, billing_period, 
			credits_per_billing_period, alerts_limit, strategy_alerts_limit, 
			features, is_active, is_popular, sort_order, created_at, updated_at
		FROM subscription_plans 
		WHERE is_active = TRUE 
		ORDER BY sort_order ASC`

	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying subscription plans: %w", err)
	}
	defer rows.Close()

	var plans []SubscriptionPlan
	for rows.Next() {
		var plan SubscriptionPlan
		var featuresJSON sql.NullString
		var description sql.NullString
		var stripePriceIDTest, stripePriceIDLive sql.NullString

		err := rows.Scan(
			&plan.ID, &plan.PlanName, &stripePriceIDTest, &stripePriceIDLive,
			&plan.DisplayName, &description, &plan.PriceCents, &plan.BillingPeriod,
			&plan.CreditsPerBillingPeriod, &plan.AlertsLimit, &plan.StrategyAlertsLimit,
			&featuresJSON, &plan.IsActive, &plan.IsPopular, &plan.SortOrder,
			&plan.CreatedAt, &plan.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning subscription plan: %w", err)
		}

		// Handle nullable fields
		if description.Valid {
			plan.Description = &description.String
		}
		if stripePriceIDTest.Valid {
			plan.StripePriceIDTest = &stripePriceIDTest.String
		}
		if stripePriceIDLive.Valid {
			plan.StripePriceIDLive = &stripePriceIDLive.String
		}

		// Parse features JSON
		if featuresJSON.Valid {
			var features []string
			if err := json.Unmarshal([]byte(featuresJSON.String), &features); err != nil {
				log.Printf("Warning: Failed to parse features JSON for plan %s: %v", plan.PlanName, err)
				plan.Features = []string{} // Default to empty array on error
			} else {
				plan.Features = features
			}
		} else {
			plan.Features = []string{} // Default to empty array
		}

		plans = append(plans, plan)
	}

	return plans, nil
}

// GetCreditProducts retrieves all active credit products
func GetCreditProducts(conn *data.Conn) ([]CreditProduct, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT 
			id, product_key, stripe_price_id_test, stripe_price_id_live, 
			display_name, description, credit_amount, price_cents, 
			is_active, is_popular, sort_order, created_at, updated_at
		FROM credit_products 
		WHERE is_active = TRUE 
		ORDER BY sort_order ASC`

	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying credit products: %w", err)
	}
	defer rows.Close()

	var products []CreditProduct
	for rows.Next() {
		var product CreditProduct
		var description sql.NullString
		var stripePriceIDTest, stripePriceIDLive sql.NullString

		err := rows.Scan(
			&product.ID, &product.ProductKey, &stripePriceIDTest, &stripePriceIDLive,
			&product.DisplayName, &description, &product.CreditAmount, &product.PriceCents,
			&product.IsActive, &product.IsPopular, &product.SortOrder,
			&product.CreatedAt, &product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning credit product: %w", err)
		}

		// Handle nullable fields
		if description.Valid {
			product.Description = &description.String
		}
		if stripePriceIDTest.Valid {
			product.StripePriceIDTest = &stripePriceIDTest.String
		}
		if stripePriceIDLive.Valid {
			product.StripePriceIDLive = &stripePriceIDLive.String
		}

		products = append(products, product)
	}

	return products, nil
}

// GetPlanNameFromPriceID retrieves the plan name for a given Stripe price ID
func GetPlanNameFromPriceID(conn *data.Conn, priceID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var planName string
	query := `
		SELECT plan_name 
		FROM subscription_plans 
		WHERE stripe_price_id_test = $1 OR stripe_price_id_live = $1`

	err := conn.DB.QueryRow(ctx, query, priceID).Scan(&planName)
	if err != nil {
		return "", fmt.Errorf("plan not found for price ID %s: %w", priceID, err)
	}

	return planName, nil
}

// GetCreditAmountFromPriceID retrieves the credit amount for a given Stripe price ID
func GetCreditAmountFromPriceID(conn *data.Conn, priceID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var creditAmount int
	query := `
		SELECT credit_amount 
		FROM credit_products 
		WHERE stripe_price_id_test = $1 OR stripe_price_id_live = $1`

	err := conn.DB.QueryRow(ctx, query, priceID).Scan(&creditAmount)
	if err != nil {
		return 0, fmt.Errorf("credit product not found for price ID %s: %w", priceID, err)
	}

	return creditAmount, nil
}

// IsCreditPriceID checks if a given price ID belongs to a credit product
func IsCreditPriceID(conn *data.Conn, priceID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM credit_products 
			WHERE stripe_price_id_test = $1 OR stripe_price_id_live = $1
		)`

	err := conn.DB.QueryRow(ctx, query, priceID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if price ID is credit product: %w", err)
	}

	return exists, nil
}

// GetStripeEnvironment determines if we're in test or live mode
func GetStripeEnvironment() string {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		log.Println("Warning: STRIPE_SECRET_KEY not set, defaulting to test mode")
		return "test"
	}
	
	// Stripe test keys start with "sk_test_", live keys start with "sk_live_"
	if len(stripeKey) >= 8 && stripeKey[:8] == "sk_test_" {
		return "test"
	} else if len(stripeKey) >= 8 && stripeKey[:8] == "sk_live_" {
		return "live"
	}
	
	// Default to test mode if we can't determine
	log.Println("Warning: Could not determine Stripe environment, defaulting to test mode")
	return "test"
}

// GetStripePriceIDForPlan gets the appropriate Stripe price ID for a plan based on environment
func GetStripePriceIDForPlan(conn *data.Conn, planName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	environment := GetStripeEnvironment()
	
	var priceID sql.NullString
	var query string
	
	if environment == "test" {
		query = `SELECT stripe_price_id_test FROM subscription_plans WHERE plan_name = $1`
	} else {
		query = `SELECT stripe_price_id_live FROM subscription_plans WHERE plan_name = $1`
	}

	err := conn.DB.QueryRow(ctx, query, planName).Scan(&priceID)
	if err != nil {
		return "", fmt.Errorf("plan not found: %s", planName)
	}

	if !priceID.Valid {
		return "", fmt.Errorf("no %s price ID configured for plan: %s", environment, planName)
	}

	return priceID.String, nil
}

// GetStripePriceIDForCreditProduct gets the appropriate Stripe price ID for a credit product based on environment
func GetStripePriceIDForCreditProduct(conn *data.Conn, productKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	environment := GetStripeEnvironment()
	
	var priceID sql.NullString
	var query string
	
	if environment == "test" {
		query = `SELECT stripe_price_id_test FROM credit_products WHERE product_key = $1`
	} else {
		query = `SELECT stripe_price_id_live FROM credit_products WHERE product_key = $1`
	}

	err := conn.DB.QueryRow(ctx, query, productKey).Scan(&priceID)
	if err != nil {
		return "", fmt.Errorf("credit product not found: %s", productKey)
	}

	if !priceID.Valid {
		return "", fmt.Errorf("no %s price ID configured for credit product: %s", environment, productKey)
	}

	return priceID.String, nil
} 