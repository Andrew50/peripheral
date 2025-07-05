package server

import (
	"backend/internal/app/limits"
	"backend/internal/app/pricing"
	"backend/internal/data"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v78"
	billingportalsession "github.com/stripe/stripe-go/v78/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/subscription"
	"github.com/stripe/stripe-go/v78/webhook"
)

func init() {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	if stripe.Key == "" {
		log.Println("Warning: STRIPE_SECRET_KEY not set")
	}
}

// StripeCreateCheckoutSession creates a new Stripe Checkout session for subscription
func StripeCreateCheckoutSession(userID int, priceID, userEmail string) (*stripe.CheckoutSession, error) {
	frontendURL := getStripeEnvOrDefault("FRONTEND_URL", "https://peripheral.io")

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:         stripe.String(fmt.Sprintf("%s/app?session_id={CHECKOUT_SESSION_ID}", frontendURL)),
		CancelURL:          stripe.String(fmt.Sprintf("%s/pricing", frontendURL)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"user_id": fmt.Sprintf("%d", userID),
		},
	}

	// If user email is provided, pre-fill it
	if userEmail != "" {
		params.CustomerEmail = stripe.String(userEmail)
	}

	return checkoutsession.New(params)
}

// StripeCreateCreditCheckoutSession creates a new Stripe Checkout session for credit purchases
func StripeCreateCreditCheckoutSession(userID int, priceID, userEmail string, creditAmount int) (*stripe.CheckoutSession, error) {
	frontendURL := getStripeEnvOrDefault("FRONTEND_URL", "https://peripheral.io")

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)), // One-time payment for credits
		SuccessURL:         stripe.String(fmt.Sprintf("%s/pricing?credits_purchased={CHECKOUT_SESSION_ID}", frontendURL)),
		CancelURL:          stripe.String(fmt.Sprintf("%s/pricing", frontendURL)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"user_id":       fmt.Sprintf("%d", userID),
			"credit_amount": fmt.Sprintf("%d", creditAmount),
			"purchase_type": "credits",
		},
	}

	// If user email is provided, pre-fill it
	if userEmail != "" {
		params.CustomerEmail = stripe.String(userEmail)
	}

	return checkoutsession.New(params)
}

// StripeCreatePortalSession creates a new Stripe billing portal session
func StripeCreatePortalSession(stripeCustomerID string) (*stripe.BillingPortalSession, error) {
	frontendURL := getStripeEnvOrDefault("FRONTEND_URL", "https://peripheral.io")

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(fmt.Sprintf("%s/pricing", frontendURL)),
	}

	return billingportalsession.New(params)
}

// StripeCreateCustomer creates a new Stripe customer
func StripeCreateCustomer(email, name string, userID int) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"user_id": fmt.Sprintf("%d", userID),
		},
	}

	return customer.New(params)
}

// HandleStripeWebhook processes Stripe webhook events
func HandleStripeWebhook(conn *data.Conn, w http.ResponseWriter, r *http.Request) {
	const MaxBodyBytes = int64(65536)
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading Stripe webhook body: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Verify webhook signature - try multiple possible secrets
	var event stripe.Event
	var err error
	
	// Try primary webhook secret
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if webhookSecret != "" {
		event, err = webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), webhookSecret)
		if err == nil {
			// Successfully verified with primary secret
		} else {
			log.Printf("Failed to verify with primary webhook secret: %v", err)
		}
	}
	
	if err != nil {
		log.Printf("Error verifying Stripe webhook signature: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Handle the event
	switch event.Type {
	case "checkout.session.completed":
		err = handleStripeCheckoutSessionCompleted(conn, event)
	case "customer.subscription.deleted":
		err = handleStripeSubscriptionDeleted(conn, event)
	case "customer.subscription.updated":
		err = handleStripeSubscriptionUpdated(conn, event)
	case "invoice.payment_failed":
		err = handleStripePaymentFailed(conn, event)
	case "invoice.payment_succeeded":
		err = handleStripePaymentSucceeded(conn, event)
	default:
		log.Printf("Unhandled Stripe event type: %s", event.Type)
	}

	if err != nil {
		log.Printf("Error handling Stripe webhook event %s: %v", event.Type, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Helper function to map Stripe price IDs to plan names using database
func getPlanNameFromPriceID(conn *data.Conn, priceID string) (string, error) {
	return pricing.GetPlanNameFromPriceID(conn, priceID)
}

// Helper function to get credit amount from price ID using database
func getCreditAmountFromPriceID(conn *data.Conn, priceID string) (int, error) {
	return pricing.GetCreditAmountFromPriceID(conn, priceID)
}

// Helper function to check if price ID is for credits using database
func isCreditPriceID(conn *data.Conn, priceID string) (bool, error) {
	return pricing.IsCreditPriceID(conn, priceID)
}

func handleStripeCheckoutSessionCompleted(conn *data.Conn, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("error parsing checkout session: %v", err)
	}

	userID, err := strconv.Atoi(session.Metadata["user_id"])
	if err != nil {
		return fmt.Errorf("invalid user_id in session metadata: %v", err)
	}

	// Check if this is a credit purchase
	if purchaseType, exists := session.Metadata["purchase_type"]; exists && purchaseType == "credits" {
		return handleCreditPurchase(conn, session, userID)
	}

	// Handle subscription purchase
	return handleSubscriptionPurchase(conn, session, userID)
}

// handleCreditPurchase processes credit purchases from Stripe checkout
func handleCreditPurchase(conn *data.Conn, session stripe.CheckoutSession, userID int) error {
	// Get credit amount from metadata or price ID
	var creditAmount int
	var err error

	if creditAmountStr, exists := session.Metadata["credit_amount"]; exists {
		creditAmount, err = strconv.Atoi(creditAmountStr)
		if err != nil {
			return fmt.Errorf("invalid credit_amount in session metadata: %v", err)
		}
	} else {
		// Fallback: try to get credit amount from price ID
		var priceID string
		if len(session.LineItems.Data) > 0 {
			priceID = session.LineItems.Data[0].Price.ID
		} else {
			// If line items aren't expanded, fetch the session with expanded line items
			sessionParams := &stripe.CheckoutSessionParams{}
			sessionParams.AddExpand("line_items")
			expandedSession, err := checkoutsession.Get(session.ID, sessionParams)
			if err != nil {
				return fmt.Errorf("could not fetch expanded checkout session: %v", err)
			}
			if len(expandedSession.LineItems.Data) > 0 {
				priceID = expandedSession.LineItems.Data[0].Price.ID
			}
		}

		if priceID == "" {
			return fmt.Errorf("could not determine price ID for credit purchase")
		}

		creditAmount, err = getCreditAmountFromPriceID(conn, priceID)
		if err != nil {
			return fmt.Errorf("error getting credit amount from price ID: %v", err)
		}
	}

	// Add purchased credits to user's account
	if err := limits.AddPurchasedCredits(conn, userID, creditAmount); err != nil {
		return fmt.Errorf("error adding purchased credits: %v", err)
	}

	log.Printf("Successfully added %d purchased credits to user %d", creditAmount, userID)
	return nil
}

// handleSubscriptionPurchase processes subscription purchases from Stripe checkout
func handleSubscriptionPurchase(conn *data.Conn, session stripe.CheckoutSession, userID int) error {
	// Get the price ID from the session line items to determine the plan
	var priceID string
	if len(session.LineItems.Data) > 0 {
		priceID = session.LineItems.Data[0].Price.ID
	} else {
		// If line items aren't expanded, we need to fetch the checkout session with expanded line items
		sessionParams := &stripe.CheckoutSessionParams{}
		sessionParams.AddExpand("line_items")
		expandedSession, err := checkoutsession.Get(session.ID, sessionParams)
		if err != nil {
			log.Printf("Warning: Could not fetch expanded checkout session %s: %v", session.ID, err)
		} else if len(expandedSession.LineItems.Data) > 0 {
			priceID = expandedSession.LineItems.Data[0].Price.ID
		}
		
		// If still no price ID, try to get it from the subscription
		if priceID == "" && session.Subscription != nil {
			subscriptionID := session.Subscription.ID
			subscription, err := subscription.Get(subscriptionID, nil)
			if err != nil {
				log.Printf("Warning: Could not fetch subscription %s to get price ID: %v", subscriptionID, err)
				priceID = "" // Will use default plan name
			} else if len(subscription.Items.Data) > 0 {
				priceID = subscription.Items.Data[0].Price.ID
			}
		}
	}

	// Get plan name from price ID
	planName, err := getPlanNameFromPriceID(conn, priceID)
	if err != nil {
		return fmt.Errorf("error getting plan name from price ID: %v", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update user with Stripe customer ID, subscription info, and plan name
	_, err = conn.DB.Exec(ctx, `
		UPDATE users 
		SET stripe_customer_id = $1, 
		    stripe_subscription_id = $2, 
		    subscription_status = 'active',
		    subscription_plan = $4,
		    stripe_price_id = $5,
		    updated_at = CURRENT_TIMESTAMP
		WHERE userId = $3`,
		session.Customer.ID,
		session.Subscription.ID,
		userID,
		planName,
		priceID)

	if err != nil {
		return fmt.Errorf("error updating user subscription: %v", err)
	}

	// Update user credits based on the new plan
	if err := limits.UpdateUserCreditsForPlan(conn, userID, planName); err != nil {
		log.Printf("Warning: Failed to update user credits for user %d to plan %s: %v", userID, planName, err)
		// Don't fail the webhook since the subscription was successfully created
	}

	log.Printf("Successfully activated %s subscription for user %d (price ID: %s)", planName, userID, priceID)
	return nil
}

func handleStripeSubscriptionDeleted(conn *data.Conn, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("error parsing subscription: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user ID first
	var userID int
	err := conn.DB.QueryRow(ctx, `
		SELECT userId FROM users 
		WHERE stripe_subscription_id = $1`,
		subscription.ID).Scan(&userID)

	if err != nil {
		return fmt.Errorf("error finding user for canceled subscription: %v", err)
	}

	_, err = conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'canceled',
		    updated_at = CURRENT_TIMESTAMP
		WHERE stripe_subscription_id = $1`,
		subscription.ID)

	if err != nil {
		return fmt.Errorf("error updating user subscription status: %v", err)
	}

	// Reset user to Free plan credits when subscription is canceled
	if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
		log.Printf("Warning: Failed to reset user credits for user %d to Free plan: %v", userID, err)
	}

	log.Printf("Successfully canceled subscription %s for user %d", subscription.ID, userID)
	return nil
}

func handleStripeSubscriptionUpdated(conn *data.Conn, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("error parsing subscription: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status := string(subscription.Status)
	periodEnd := time.Unix(subscription.CurrentPeriodEnd, 0)

	// Get the price ID from the subscription to determine plan
	var priceID string
	var planName string
	if len(subscription.Items.Data) > 0 {
		priceID = subscription.Items.Data[0].Price.ID
		var err error
		planName, err = getPlanNameFromPriceID(conn, priceID)
		if err != nil {
			log.Printf("Warning: Error getting plan name from price ID %s: %v", priceID, err)
			planName = "" // Clear planName so we use fallback update
		}
	}

	// Get user ID for credit updates
	var userID int
	err := conn.DB.QueryRow(ctx, `
		SELECT userId FROM users 
		WHERE stripe_subscription_id = $1`,
		subscription.ID).Scan(&userID)

	if err != nil {
		return fmt.Errorf("error finding user for subscription update: %v", err)
	}

	// Update with plan name if we have it
	if planName != "" {
		_, err := conn.DB.Exec(ctx, `
			UPDATE users 
			SET subscription_status = $1,
			    current_period_end = $2,
			    subscription_plan = $4,
			    stripe_price_id = $5,
			    updated_at = CURRENT_TIMESTAMP
			WHERE stripe_subscription_id = $3`,
			status, periodEnd, subscription.ID, planName, priceID)

		if err != nil {
			return fmt.Errorf("error updating subscription: %v", err)
		}

		// Update user credits based on the plan and status
		var targetPlan string
		if status == "active" {
			targetPlan = planName
		} else {
			// For inactive statuses, reset to Free plan credits
			targetPlan = "Free"
		}

		if err := limits.UpdateUserCreditsForPlan(conn, userID, targetPlan); err != nil {
			log.Printf("Warning: Failed to update user credits for user %d to plan %s: %v", userID, targetPlan, err)
		}

		log.Printf("Successfully updated subscription %s to status %s with plan %s for user %d", subscription.ID, status, planName, userID)
	} else {
		// Fallback - update without plan info
		_, err := conn.DB.Exec(ctx, `
			UPDATE users 
			SET subscription_status = $1,
			    current_period_end = $2,
			    updated_at = CURRENT_TIMESTAMP
			WHERE stripe_subscription_id = $3`,
			status, periodEnd, subscription.ID)

		if err != nil {
			return fmt.Errorf("error updating subscription: %v", err)
		}

		// If subscription is not active, reset to Free plan
		if status != "active" {
			if err := limits.UpdateUserCreditsForPlan(conn, userID, "Free"); err != nil {
				log.Printf("Warning: Failed to reset user credits for user %d to Free plan: %v", userID, err)
			}
		}

		log.Printf("Successfully updated subscription %s to status %s for user %d", subscription.ID, status, userID)
	}

	return nil
}

func handleStripePaymentFailed(conn *data.Conn, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("error parsing invoice: %v", err)
	}

	if invoice.Subscription == nil {
		return nil // Not a subscription invoice
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update subscription status to past_due
	_, err := conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'past_due',
		    updated_at = CURRENT_TIMESTAMP
		WHERE stripe_subscription_id = $1`,
		invoice.Subscription.ID)

	if err != nil {
		return fmt.Errorf("error updating subscription status to past_due: %v", err)
	}

	log.Printf("Updated subscription %s to past_due due to payment failure", invoice.Subscription.ID)
	return nil
}

func handleStripePaymentSucceeded(conn *data.Conn, event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("error parsing invoice: %v", err)
	}

	if invoice.Subscription == nil {
		return nil // Not a subscription invoice
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch the current subscription from Stripe to get up-to-date plan information
	stripeSubscription, err := subscription.Get(invoice.Subscription.ID, nil)
	if err != nil {
		return fmt.Errorf("error fetching subscription from Stripe: %v", err)
	}

	// Get the price ID from the subscription to determine current plan
	var priceID string
	var planName string
	if len(stripeSubscription.Items.Data) > 0 {
		priceID = stripeSubscription.Items.Data[0].Price.ID
		planName, err = getPlanNameFromPriceID(conn, priceID)
		if err != nil {
			log.Printf("Warning: Error getting plan name from price ID %s: %v", priceID, err)
			planName = "" // Will use fallback update
		}
	}

	// Get user ID for credit updates
	var userID int
	err = conn.DB.QueryRow(ctx, `
		SELECT userId FROM users 
		WHERE stripe_subscription_id = $1`,
		invoice.Subscription.ID).Scan(&userID)

	if err != nil {
		return fmt.Errorf("error finding user for subscription: %v", err)
	}

	// Ensure we have plan information - return error if not found
	if planName == "" {
		return fmt.Errorf("could not determine plan name for subscription %s", invoice.Subscription.ID)
	}

	// Update subscription with current plan information
	periodEnd := time.Unix(invoice.PeriodEnd, 0)
	_, err = conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'active',
		    current_period_end = $2,
		    subscription_plan = $4,
		    stripe_price_id = $5,
		    updated_at = CURRENT_TIMESTAMP
		WHERE stripe_subscription_id = $1`,
		invoice.Subscription.ID, periodEnd, userID, planName, priceID)

	if err != nil {
		return fmt.Errorf("error updating subscription with plan info: %v", err)
	}

	// Reset subscription credits for the user's billing cycle with current plan
	if err := limits.ResetUserSubscriptionCredits(conn, userID, planName); err != nil {
		log.Printf("Warning: Failed to reset subscription credits for user %d: %v", userID, err)
		// Don't fail the webhook since the subscription was successfully renewed
	}

	log.Printf("Updated subscription %s to active with plan %s, period end %s and reset credits for user %d", invoice.Subscription.ID, planName, periodEnd, userID)

	return nil
}

// Helper function to get environment variable with default
func getStripeEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
