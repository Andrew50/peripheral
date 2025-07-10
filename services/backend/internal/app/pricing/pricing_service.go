package pricing

import (
	"backend/internal/data"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
)

// SubscriptionProduct represents a subscription product configuration (renamed from SubscriptionPlan)
type SubscriptionProduct struct {
	ID                     int       `json:"id"`
	ProductKey             string    `json:"product_key"`
	QueriesLimit           int       `json:"queries_limit"`
	AlertsLimit            int       `json:"alerts_limit"`
	StrategyAlertsLimit    int       `json:"strategy_alerts_limit"`
	RealtimeCharts         bool      `json:"realtime_charts"`
	SubMinuteCharts        bool      `json:"sub_minute_charts"`
	MultiChart             bool      `json:"multi_chart"`
	MultiStrategyScreening bool      `json:"multi_strategy_screening"`
	WatchlistAlerts        bool      `json:"watchlist_alerts"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// Price represents pricing information for products
type Price struct {
	ID                int       `json:"id"`
	PriceCents        int       `json:"price_cents"`
	StripePriceIDLive *string   `json:"stripe_price_id_live"`
	StripePriceIDTest *string   `json:"stripe_price_id_test"`
	ProductKey        string    `json:"product_key"`
	BillingPeriod     string    `json:"billing_period"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SubscriptionPlanWithPricing combines subscription product with pricing information
type SubscriptionPlanWithPricing struct {
	SubscriptionProduct
	Prices []Price `json:"prices"`
}

// CreditProduct represents a credit product configuration (simplified after migration)
type CreditProduct struct {
	ID           int       `json:"id"`
	ProductKey   string    `json:"product_key"`
	CreditAmount int       `json:"credit_amount"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// Pricing info now comes from prices table
	StripePriceIDTest *string `json:"stripe_price_id_test"`
	StripePriceIDLive *string `json:"stripe_price_id_live"`
	PriceCents        int     `json:"price_cents"`
}

// GetSubscriptionProductsWithPricing retrieves all subscription products with their pricing
func GetSubscriptionProductsWithPricing(conn *data.Conn) ([]SubscriptionPlanWithPricing, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get subscription products
	query := `
		SELECT 
			id, product_key, queries_limit, alerts_limit, strategy_alerts_limit,
			realtime_charts, sub_minute_charts, multi_chart, multi_strategy_screening,
			watchlist_alerts, created_at, updated_at
		FROM subscription_products 
		ORDER BY id ASC`

	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying subscription products: %w", err)
	}
	defer rows.Close()

	var products []SubscriptionPlanWithPricing
	for rows.Next() {
		var product SubscriptionProduct
		err := rows.Scan(
			&product.ID, &product.ProductKey, &product.QueriesLimit, &product.AlertsLimit,
			&product.StrategyAlertsLimit, &product.RealtimeCharts, &product.SubMinuteCharts,
			&product.MultiChart, &product.MultiStrategyScreening, &product.WatchlistAlerts,
			&product.CreatedAt, &product.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning subscription product: %w", err)
		}

		// Get prices for this product using product_key
		prices, err := getPricesForProductKey(conn, product.ProductKey, ctx)
		if err != nil {
			return nil, fmt.Errorf("error getting prices for product %s: %w", product.ProductKey, err)
		}

		products = append(products, SubscriptionPlanWithPricing{
			SubscriptionProduct: product,
			Prices:              prices,
		})
	}

	return products, nil
}

// getPricesForProductKey retrieves all prices for a given product key
func getPricesForProductKey(conn *data.Conn, productKey string, ctx context.Context) ([]Price, error) {
	query := `
		SELECT 
			id, price_cents, stripe_price_id_live, stripe_price_id_test,
			product_key, billing_period, created_at, updated_at
		FROM prices 
		WHERE product_key = $1
		ORDER BY billing_period ASC`

	rows, err := conn.DB.Query(ctx, query, productKey)
	if err != nil {
		return nil, fmt.Errorf("error querying prices: %w", err)
	}
	defer rows.Close()

	var prices []Price
	for rows.Next() {
		var price Price
		var stripePriceIDLive, stripePriceIDTest sql.NullString

		err := rows.Scan(
			&price.ID, &price.PriceCents, &stripePriceIDLive, &stripePriceIDTest,
			&price.ProductKey, &price.BillingPeriod, &price.CreatedAt, &price.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning price: %w", err)
		}

		if stripePriceIDLive.Valid {
			price.StripePriceIDLive = &stripePriceIDLive.String
		}
		if stripePriceIDTest.Valid {
			price.StripePriceIDTest = &stripePriceIDTest.String
		}

		prices = append(prices, price)
	}

	return prices, nil
}

// GetCreditProducts retrieves all credit products with their pricing information
func GetCreditProducts(conn *data.Conn) ([]CreditProduct, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT 
			cp.id, cp.product_key, cp.credit_amount, 
			cp.created_at, cp.updated_at,
			p.stripe_price_id_test, p.stripe_price_id_live, p.price_cents
		FROM credit_products cp
		LEFT JOIN prices p ON cp.product_key = p.product_key AND p.billing_period = 'single'
		ORDER BY cp.id ASC`

	rows, err := conn.DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying credit products: %w", err)
	}
	defer rows.Close()

	var products []CreditProduct
	for rows.Next() {
		var product CreditProduct
		var stripePriceIDTest, stripePriceIDLive sql.NullString
		var priceCents sql.NullInt32

		err := rows.Scan(
			&product.ID, &product.ProductKey, &product.CreditAmount,
			&product.CreatedAt, &product.UpdatedAt,
			&stripePriceIDTest, &stripePriceIDLive, &priceCents,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning credit product: %w", err)
		}

		if stripePriceIDTest.Valid {
			product.StripePriceIDTest = &stripePriceIDTest.String
		}
		if stripePriceIDLive.Valid {
			product.StripePriceIDLive = &stripePriceIDLive.String
		}
		if priceCents.Valid {
			product.PriceCents = int(priceCents.Int32)
		}

		products = append(products, product)
	}

	return products, nil
}

// GetProductKeyFromPriceID retrieves the product key for a given Stripe price ID
func GetProductKeyFromPriceID(conn *data.Conn, priceID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var productKey string
	query := `
		SELECT product_key 
		FROM prices
		WHERE stripe_price_id_test = $1 OR stripe_price_id_live = $1`

	err := conn.DB.QueryRow(ctx, query, priceID).Scan(&productKey)
	if err != nil {
		return "", fmt.Errorf("product not found for price ID %s: %w", priceID, err)
	}

	return productKey, nil
}

// GetCreditAmountFromPriceID retrieves the credit amount for a given Stripe price ID
func GetCreditAmountFromPriceID(conn *data.Conn, priceID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var creditAmount int
	query := `
		SELECT cp.credit_amount 
		FROM credit_products cp
		JOIN prices p ON cp.product_key = p.product_key
		WHERE (p.stripe_price_id_test = $1 OR p.stripe_price_id_live = $1)
		AND p.billing_period = 'single'`

	err := conn.DB.QueryRow(ctx, query, priceID).Scan(&creditAmount)
	if err != nil {
		return 0, fmt.Errorf("credit product not found for price ID %s: %w", priceID, err)
	}

	return creditAmount, nil
}

// IsCreditPriceID checks if a given price ID belongs to a credit product
// COMMENTED OUT: This function is not used anywhere in the codebase
/*
func IsCreditPriceID(conn *data.Conn, priceID string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM credit_products cp
			JOIN prices p ON cp.product_key = p.product_key
			WHERE (p.stripe_price_id_test = $1 OR p.stripe_price_id_live = $1)
			AND p.billing_period = 'single'
		)`

	err := conn.DB.QueryRow(ctx, query, priceID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if price ID is credit product: %w", err)
	}

	return exists, nil
}
*/

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

// GetStripePriceIDForProduct gets the appropriate Stripe price ID for a product based on environment
// COMMENTED OUT: This function is not used anywhere in the codebase
/*
func GetStripePriceIDForProduct(conn *data.Conn, productKey string, billingPeriod string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	environment := GetStripeEnvironment()

	var priceID sql.NullString
	var query string

	if environment == "test" {
		query = `
			SELECT stripe_price_id_test
			FROM prices
			WHERE product_key = $1 AND billing_period = $2`
	} else {
		query = `
			SELECT stripe_price_id_live
			FROM prices
			WHERE product_key = $1 AND billing_period = $2`
	}

	err := conn.DB.QueryRow(ctx, query, productKey, billingPeriod).Scan(&priceID)
	if err != nil {
		return "", fmt.Errorf("product not found: %s with billing period: %s", productKey, billingPeriod)
	}

	if !priceID.Valid {
		return "", fmt.Errorf("no %s price ID configured for product: %s with billing period: %s", environment, productKey, billingPeriod)
	}

	return priceID.String, nil
}
*/

// GetStripePriceIDForCreditProduct gets the appropriate Stripe price ID for a credit product based on environment
// COMMENTED OUT: This function is not used anywhere in the codebase
/*
func GetStripePriceIDForCreditProduct(conn *data.Conn, productKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	environment := GetStripeEnvironment()

	var priceID sql.NullString
	var query string

	if environment == "test" {
		query = `
			SELECT stripe_price_id_test
			FROM prices
			WHERE product_key = $1 AND billing_period = 'single'`
	} else {
		query = `
			SELECT stripe_price_id_live
			FROM prices
			WHERE product_key = $1 AND billing_period = 'single'`
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
*/
