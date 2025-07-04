package server

import (
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// CreateCustomerPortal creates a new Stripe customer portal session
func CreateCustomerPortal(conn *data.Conn, userID int, rawArgs json.RawMessage) (interface{}, error) {
	// Get user's Stripe customer ID from database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var stripeCustomerID, stripeSubscriptionID, subscriptionStatus string
	var currentPeriodEnd time.Time

	log.Printf("Executing query for userID: %d", userID)
	err := conn.DB.QueryRow(ctx, `
		SELECT 
			COALESCE(stripe_customer_id, '') as stripe_customer_id,
			COALESCE(stripe_subscription_id, '') as stripe_subscription_id,
			COALESCE(subscription_status, 'inactive') as subscription_status,
			COALESCE(current_period_end, '1970-01-01') as current_period_end
		FROM users 
		WHERE userId = $1`, userID).Scan(&stripeCustomerID, &stripeSubscriptionID, &subscriptionStatus, &currentPeriodEnd)

	if err != nil {
		log.Printf("Error getting user subscription status for userID %d: %v", userID, err)
		return nil, fmt.Errorf("error retrieving subscription status")
	}

	log.Printf("Query successful for userID %d: stripeCustomerID=%s, stripeSubscriptionID=%s, subscriptionStatus=%s",
		userID, stripeCustomerID, stripeSubscriptionID, subscriptionStatus)

	// Determine subscription plan based on status and other factors
	var currentPlan string
	var isActive bool

	switch subscriptionStatus {
	case "active":
		isActive = true
		currentPlan = "pro" // Default to pro for now, you can enhance this later
	case "past_due", "unpaid":
		isActive = false
		currentPlan = "pro" // Keep plan name but mark as inactive
	default:
		isActive = false
		currentPlan = ""
	}

	response := map[string]interface{}{
		"status":           subscriptionStatus,
		"isActive":         isActive,
		"currentPlan":      currentPlan,
		"hasCustomer":      stripeCustomerID != "",
		"hasSubscription":  stripeSubscriptionID != "",
		"currentPeriodEnd": nil,
	}

	// Only include period end if we have a valid subscription
	if stripeSubscriptionID != "" && !currentPeriodEnd.Before(time.Unix(1, 0)) {
		response["currentPeriodEnd"] = currentPeriodEnd.Unix()
	}

	log.Printf("Returning subscription status for userID %d: %+v", userID, response)
	return response, nil
}
