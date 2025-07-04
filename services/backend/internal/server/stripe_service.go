package server

import (
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
	frontendURL := getStripeEnvOrDefault("FRONTEND_URL", "https://atlantis.trading")

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

// StripeCreatePortalSession creates a new Stripe billing portal session
func StripeCreatePortalSession(stripeCustomerID string) (*stripe.BillingPortalSession, error) {
	frontendURL := getStripeEnvOrDefault("FRONTEND_URL", "https://atlantis.trading")

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

	// Verify webhook signature
	event, err := webhook.ConstructEvent(payload, r.Header.Get("Stripe-Signature"), os.Getenv("STRIPE_WEBHOOK_SECRET"))
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

func handleStripeCheckoutSessionCompleted(conn *data.Conn, event stripe.Event) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return fmt.Errorf("error parsing checkout session: %v", err)
	}

	userID, err := strconv.Atoi(session.Metadata["user_id"])
	if err != nil {
		return fmt.Errorf("invalid user_id in session metadata: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update user with Stripe customer ID and subscription info
	_, err = conn.DB.Exec(ctx, `
		UPDATE users 
		SET stripe_customer_id = $1, 
		    stripe_subscription_id = $2, 
		    subscription_status = 'active',
		    updated_at = CURRENT_TIMESTAMP
		WHERE userId = $3`,
		session.Customer.ID,
		session.Subscription.ID,
		userID)

	if err != nil {
		return fmt.Errorf("error updating user subscription: %v", err)
	}

	log.Printf("Successfully activated subscription for user %d", userID)
	return nil
}

func handleStripeSubscriptionDeleted(conn *data.Conn, event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("error parsing subscription: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'canceled',
		    updated_at = CURRENT_TIMESTAMP
		WHERE stripe_subscription_id = $1`,
		subscription.ID)

	if err != nil {
		return fmt.Errorf("error updating user subscription status: %v", err)
	}

	log.Printf("Successfully canceled subscription %s", subscription.ID)
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

	log.Printf("Successfully updated subscription %s to status %s", subscription.ID, status)
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

	_, err := conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'past_due',
		    updated_at = CURRENT_TIMESTAMP
		WHERE stripe_subscription_id = $1`,
		invoice.Subscription.ID)

	if err != nil {
		return fmt.Errorf("error updating user payment status: %v", err)
	}

	log.Printf("Payment failed for subscription %s", invoice.Subscription.ID)
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

	_, err := conn.DB.Exec(ctx, `
		UPDATE users 
		SET subscription_status = 'active',
		    updated_at = CURRENT_TIMESTAMP
		WHERE stripe_subscription_id = $1`,
		invoice.Subscription.ID)

	if err != nil {
		return fmt.Errorf("error updating user payment status: %v", err)
	}

	log.Printf("Payment succeeded for subscription %s", invoice.Subscription.ID)
	return nil
}

// Helper function to get environment variables with defaults for Stripe
func getStripeEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
