package server

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"backend/internal/app/pricing"

	stripe "github.com/stripe/stripe-go/v82"
	subscription "github.com/stripe/stripe-go/v82/subscription"

	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
)

// CreateCheckoutSessionArgs represents arguments for creating a checkout session
type CreateCheckoutSessionArgs struct {
	PriceID string `json:"priceId"`
}

// CreateCheckoutSession creates a new Stripe checkout session for subscription
func CreateCheckoutSession(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args CreateCheckoutSessionArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	if args.PriceID == "" {
		return nil, fmt.Errorf("priceId is required")
	}

	// Get user email from database
	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	var email string
	err := conn.DB.QueryRow(ctx, "SELECT email FROM users WHERE userId = $1", userID).Scan(&email)
	if err != nil {
		log.Printf("Error getting user email for checkout: %v", err)
		return nil, fmt.Errorf("error retrieving user information")
	}

	// Create checkout session using the new function name
	session, err := StripeCreateCheckoutSession(userID, args.PriceID, email)
	if err != nil {
		log.Printf("Error creating Stripe checkout session: %v", err)
		return nil, fmt.Errorf("error creating checkout session: %v", err)
	}

	return map[string]string{
		"sessionId": session.ID,
		"url":       session.URL,
	}, nil
}

// CreateCreditCheckoutSessionArgs represents arguments for creating a credit checkout session
type CreateCreditCheckoutSessionArgs struct {
	PriceID      string `json:"priceId"`
	CreditAmount int    `json:"creditAmount"`
}

// CreateCreditCheckoutSession creates a new Stripe checkout session for credit purchases
func CreateCreditCheckoutSession(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args CreateCreditCheckoutSessionArgs
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	if args.PriceID == "" {
		return nil, fmt.Errorf("priceId is required")
	}

	if args.CreditAmount <= 0 {
		return nil, fmt.Errorf("creditAmount must be positive")
	}

	// Check if user has an active subscription (required for credit purchases)
	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	var email, subscriptionStatus string
	err := conn.DB.QueryRow(ctx, `
		SELECT email, COALESCE(subscription_status, 'inactive') 
		FROM users WHERE userId = $1`, userID).Scan(&email, &subscriptionStatus)
	if err != nil {
		log.Printf("Error getting user info for credit checkout: %v", err)
		return nil, fmt.Errorf("error retrieving user information")
	}

	// Require active subscription for credit purchases
	if subscriptionStatus != "active" {
		return nil, fmt.Errorf("active subscription required to purchase credits")
	}

	// Create credit checkout session
	session, err := StripeCreateCreditCheckoutSession(userID, args.PriceID, email, args.CreditAmount)
	if err != nil {
		log.Printf("Error creating Stripe credit checkout session: %v", err)
		return nil, fmt.Errorf("error creating credit checkout session: %v", err)
	}

	return map[string]string{
		"sessionId": session.ID,
		"url":       session.URL,
	}, nil
}

// CreateCustomerPortal creates a new Stripe customer portal session
func CreateCustomerPortal(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Get user's Stripe customer ID from database
	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	var stripeCustomerID string
	err := conn.DB.QueryRow(ctx, "SELECT stripe_customer_id FROM users WHERE userId = $1", userID).Scan(&stripeCustomerID)
	if err != nil {
		log.Printf("Error getting user Stripe customer ID: %v", err)
		return nil, fmt.Errorf("no active subscription found")
	}

	if stripeCustomerID == "" {
		return nil, fmt.Errorf("no active subscription found")
	}

	// Create portal session using the new function name
	session, err := StripeCreatePortalSession(stripeCustomerID)
	if err != nil {
		log.Printf("Error creating Stripe portal session: %v", err)
		return nil, fmt.Errorf("error creating portal session: %v", err)
	}

	return map[string]string{
		"url": session.URL,
	}, nil
}

// stripeWebhookHandler handles Stripe webhook events
func stripeWebhookHandler(conn *data.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Delegate to the new Stripe webhook handler in the same package
		HandleStripeWebhook(conn, w, r)
	}
}

// GetSubscriptionStatus retrieves the current subscription status for a user
func GetSubscriptionStatus(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("GetSubscriptionStatus called for userID: %d", userID)

	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	var stripeCustomerID, stripeSubscriptionID, subscriptionStatus, subscriptionPlan string
	var currentPeriodEnd time.Time
	var subscriptionCreditsRemaining, purchasedCreditsRemaining, totalCreditsRemaining, subscriptionCreditsAllocated int

	log.Printf("Executing query for userID: %d", userID)
	err := conn.DB.QueryRow(ctx, `
		SELECT 
			COALESCE(stripe_customer_id, '') as stripe_customer_id,
			COALESCE(stripe_subscription_id, '') as stripe_subscription_id,
			COALESCE(subscription_status, 'inactive') as subscription_status,
			COALESCE(subscription_plan, '') as subscription_plan,
			COALESCE(current_period_end, '1970-01-01') as current_period_end,
			COALESCE(subscription_credits_remaining, 0) as subscription_credits_remaining,
			COALESCE(purchased_credits_remaining, 0) as purchased_credits_remaining,
			COALESCE(total_credits_remaining, 0) as total_credits_remaining,
			COALESCE(subscription_credits_allocated, 0) as subscription_credits_allocated
		FROM users 
		WHERE userId = $1`, userID).Scan(&stripeCustomerID, &stripeSubscriptionID, &subscriptionStatus, &subscriptionPlan, &currentPeriodEnd, &subscriptionCreditsRemaining, &purchasedCreditsRemaining, &totalCreditsRemaining, &subscriptionCreditsAllocated)

	if err != nil {
		log.Printf("Error getting user subscription status for userID %d: %v", userID, err)
		return nil, fmt.Errorf("error retrieving subscription status")
	}

	log.Printf("Query successful for userID %d: stripeCustomerID=%s, stripeSubscriptionID=%s, subscriptionStatus=%s, subscriptionPlan=%s",
		userID, stripeCustomerID, stripeSubscriptionID, subscriptionStatus, subscriptionPlan)

	// Determine subscription plan based on status and stored plan
	var currentPlan string
	var isActive bool
	var isCanceling bool

	switch subscriptionStatus {
	case "active":
		isActive = true
		isCanceling = false
		// Require plan name for active subscriptions
		if subscriptionPlan == "" {
			log.Printf("Error: Active subscription found for user %d but no plan name stored", userID)
			return nil, fmt.Errorf("active subscription found but plan information is missing")
		}
		currentPlan = subscriptionPlan
	case "canceling":
		isActive = true // Still active until period end
		isCanceling = true
		// Require plan name for canceling subscriptions too
		if subscriptionPlan == "" {
			log.Printf("Error: Canceling subscription found for user %d but no plan name stored", userID)
			return nil, fmt.Errorf("subscription found but plan information is missing")
		}
		currentPlan = subscriptionPlan
	case "past_due", "unpaid":
		isActive = false
		isCanceling = false
		// Require plan name for past due subscriptions too
		if subscriptionPlan == "" {
			log.Printf("Error: Past due subscription found for user %d but no plan name stored", userID)
			return nil, fmt.Errorf("subscription found but plan information is missing")
		}
		currentPlan = subscriptionPlan
	default:
		isActive = false
		isCanceling = false
		currentPlan = ""
	}

	response := map[string]interface{}{
		"status":                       subscriptionStatus,
		"isActive":                     isActive,
		"isCanceling":                  isCanceling,
		"currentPlan":                  currentPlan,
		"hasCustomer":                  stripeCustomerID != "",
		"hasSubscription":              stripeSubscriptionID != "",
		"currentPeriodEnd":             nil,
		"subscriptionCreditsRemaining": subscriptionCreditsRemaining,
		"purchasedCreditsRemaining":    purchasedCreditsRemaining,
		"totalCreditsRemaining":        totalCreditsRemaining,
		"subscriptionCreditsAllocated": subscriptionCreditsAllocated,
	}

	// Only include period end if we have a valid subscription
	if stripeSubscriptionID != "" && !currentPeriodEnd.Before(time.Unix(1, 0)) {
		response["currentPeriodEnd"] = currentPeriodEnd.Unix()
	}

	log.Printf("Returning subscription status for userID %d: %+v", userID, response)
	return response, nil
}

// VerifyCheckoutSession verifies a checkout session and returns subscription status
func VerifyCheckoutSession(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	var args struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(rawArgs, &args); err != nil {
		return nil, fmt.Errorf("invalid args: %v", err)
	}

	if args.SessionID == "" {
		return nil, fmt.Errorf("sessionId is required")
	}

	log.Printf("Verifying checkout session %s for user %d", args.SessionID, userID)

	// Get the checkout session from Stripe
	session, err := checkoutsession.Get(args.SessionID, nil)
	if err != nil {
		log.Printf("Error fetching checkout session: %v", err)
		return nil, fmt.Errorf("error fetching checkout session: %v", err)
	}

	// Verify the session belongs to this user
	if sessionUserID, exists := session.Metadata["user_id"]; !exists || sessionUserID != fmt.Sprintf("%d", userID) {
		return nil, fmt.Errorf("session does not belong to user")
	}

	// Return the current subscription status
	return GetSubscriptionStatus(conn, userID, json.RawMessage("{}"))
}

// GetUserUsage returns usage statistics for a user
func GetUserUsage(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("GetUserUsage called for userID: %d", userID)

	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	var activeAlerts, alertsLimit, activeStrategyAlerts, strategyAlertsLimit int
	var currentPeriodStart, lastLimitReset time.Time

	log.Printf("Executing usage query for userID: %d", userID)
	err := conn.DB.QueryRow(ctx, `
		SELECT 
			COALESCE(active_alerts, 0) as active_alerts,
			COALESCE(alerts_limit, 0) as alerts_limit,
			COALESCE(active_strategy_alerts, 0) as active_strategy_alerts,
			COALESCE(strategy_alerts_limit, 0) as strategy_alerts_limit,
			COALESCE(current_period_start, CURRENT_TIMESTAMP) as current_period_start,
			COALESCE(last_limit_reset, CURRENT_TIMESTAMP) as last_limit_reset
		FROM users 
		WHERE userId = $1`, userID).Scan(&activeAlerts, &alertsLimit, &activeStrategyAlerts, &strategyAlertsLimit, &currentPeriodStart, &lastLimitReset)

	if err != nil {
		log.Printf("Error getting user usage for userID %d: %v", userID, err)
		return nil, fmt.Errorf("error retrieving usage data")
	}

	response := map[string]interface{}{
		"activeAlerts":         activeAlerts,
		"alertsLimit":          alertsLimit,
		"activeStrategyAlerts": activeStrategyAlerts,
		"strategyAlertsLimit":  strategyAlertsLimit,
	}

	log.Printf("Returning usage data for userID %d", userID)
	return response, nil
}

// GetPublicPricingConfiguration returns the current subscription plans and credit products (public endpoint)
func GetPublicPricingConfiguration(conn *data.Conn, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("GetPublicPricingConfiguration called")

	// Get subscription plans
	plans, err := pricing.GetSubscriptionPlans(conn)
	if err != nil {
		log.Printf("Error getting subscription plans: %v", err)
		return nil, fmt.Errorf("error retrieving subscription plans")
	}

	// Get credit products
	creditProducts, err := pricing.GetCreditProducts(conn)
	if err != nil {
		log.Printf("Error getting credit products: %v", err)
		return nil, fmt.Errorf("error retrieving credit products")
	}

	// Use the standardized environment function
	environment := pricing.GetStripeEnvironment()

	return map[string]interface{}{
		"plans":          plans,
		"creditProducts": creditProducts,
		"environment":    environment,
	}, nil
}

// GetCombinedSubscriptionAndUsage returns both subscription status and usage stats in a single call
func GetCombinedSubscriptionAndUsage(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	log.Printf("GetCombinedSubscriptionAndUsage called for userID: %d", userID)

	// Get subscription status using existing function
	subscriptionResult, err := GetSubscriptionStatus(conn, userID, rawArgs)
	if err != nil {
		log.Printf("Error getting subscription status for userID %d: %v", userID, err)
		return nil, err
	}

	// Get usage data using new function
	usageResult, err := GetUserUsage(conn, userID, rawArgs)
	if err != nil {
		log.Printf("Error getting usage data for userID %d: %v", userID, err)
		return nil, err
	}

	// Combine the results
	subscriptionData := subscriptionResult.(map[string]interface{})
	usageData := usageResult.(map[string]interface{})

	// Merge the data
	response := make(map[string]interface{})
	for k, v := range subscriptionData {
		response[k] = v
	}
	for k, v := range usageData {
		response[k] = v
	}

	log.Printf("Returning combined subscription and usage for userID %d", userID)
	return response, nil
}

// CancelSubscription cancels the user's subscription at the end of the current billing period
func CancelSubscription(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	// Start a database transaction
	tx, err := conn.DB.Begin(ctx)
	if err != nil {
		log.Printf("Error starting transaction for user %d: %v", userID, err)
		return nil, fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(ctx) // Will be ignored if tx.Commit() is called

	// Retrieve the user's Stripe subscription ID
	var stripeSubscriptionID string
	if err := tx.QueryRow(ctx, "SELECT stripe_subscription_id FROM users WHERE userId = $1", userID).Scan(&stripeSubscriptionID); err != nil {
		log.Printf("Error fetching Stripe subscription ID for user %d: %v", userID, err)
		return nil, fmt.Errorf("no active subscription found")
	}

	if stripeSubscriptionID == "" {
		return nil, fmt.Errorf("no active subscription found")
	}

	// Update the database to reflect the cancellation state (without committing)
	_, err = tx.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'canceling',
		    updated_at = CURRENT_TIMESTAMP
		WHERE userId = $1`, userID)
	if err != nil {
		log.Printf("Error updating user subscription status to canceling: %v", err)
		return nil, fmt.Errorf("failed to update subscription status: %v", err)
	}

	// Schedule cancellation at period end with Stripe
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(true),
	}
	if _, err := subscription.Update(stripeSubscriptionID, params); err != nil {
		log.Printf("Error scheduling cancellation for subscription %s: %v", stripeSubscriptionID, err)
		return nil, fmt.Errorf("failed to cancel subscription: %v", err)
	}

	// If we reach here, both database update and Stripe update succeeded
	// Commit the database transaction
	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction for user %d: %v", userID, err)
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Subscription %s for user %d set to cancel at period end", stripeSubscriptionID, userID)
	return map[string]bool{"cancelAtPeriodEnd": true}, nil
}

// ReactivateSubscription reactivates a subscription that was set to cancel at period end
func ReactivateSubscription(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DBContextTimeout)
	defer cancel()

	// Retrieve the user's Stripe subscription ID
	var stripeSubscriptionID string
	if err := conn.DB.QueryRow(ctx, "SELECT stripe_subscription_id FROM users WHERE userId = $1", userID).Scan(&stripeSubscriptionID); err != nil {
		log.Printf("Error fetching Stripe subscription ID for user %d: %v", userID, err)
		return nil, fmt.Errorf("no subscription found")
	}

	if stripeSubscriptionID == "" {
		return nil, fmt.Errorf("no subscription found")
	}

	// Reactivate the subscription with Stripe
	params := &stripe.SubscriptionParams{
		CancelAtPeriodEnd: stripe.Bool(false),
	}
	if _, err := subscription.Update(stripeSubscriptionID, params); err != nil {
		log.Printf("Error reactivating subscription %s: %v", stripeSubscriptionID, err)
		return nil, fmt.Errorf("failed to reactivate subscription: %v", err)
	}

	// Update the database to reflect the reactivation
	_, err := conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'active',
		    updated_at = CURRENT_TIMESTAMP
		WHERE userId = $1`, userID)
	if err != nil {
		log.Printf("Error updating user subscription status to active: %v", err)
		// Don't fail the request since Stripe reactivation was successful
	}

	log.Printf("Subscription %s for user %d reactivated", stripeSubscriptionID, userID)
	return map[string]bool{"reactivated": true}, nil
}
