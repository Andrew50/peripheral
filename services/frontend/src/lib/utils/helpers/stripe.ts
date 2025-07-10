import { loadStripe } from '@stripe/stripe-js';
import { browser } from '$app/environment';

// ===== TYPE DEFINITIONS =====
// These interfaces match the new backend format exactly

// Price information from the API
export interface Price {
	id: number;
	price_cents: number;
	stripe_price_id_live: string | null;
	stripe_price_id_test: string | null;
	product_key: string;
	billing_period: string; // "monthly" or "yearly"
	created_at: string;
	updated_at: string;
}

export interface DatabasePlan {
	id: number;
	product_key: string;
	queries_limit: number;
	alerts_limit: number;
	strategy_alerts_limit: number;
	realtime_charts: boolean;
	sub_minute_charts: boolean;
	multi_chart: boolean;
	multi_strategy_screening: boolean;
	watchlist_alerts: boolean;
	prices: Price[] | null; // Array of prices for different billing periods
	created_at: string;
	updated_at: string;
}

export interface DatabaseCreditProduct {
	id: number;
	product_key: string;
	credit_amount: number;
	price_cents: number;
	stripe_price_id_test: string | null;
	stripe_price_id_live: string | null;
	created_at: string;
	updated_at: string;
}

export interface PublicPricingConfiguration {
	plans: DatabasePlan[];
	creditProducts: DatabaseCreditProduct[];
	environment: string;
}

// Checkout session response from backend
export interface CheckoutSessionResponse {
	sessionId: string;
	url: string;
}

// Customer portal response from backend
export interface CustomerPortalResponse {
	url: string;
}

// Subscription status response from backend
export interface SubscriptionStatus {
	status: string;
	isActive: boolean;
	isCanceling: boolean;
	currentPlan: string;
	hasCustomer: boolean;
	hasSubscription: boolean;
	currentPeriodEnd: number | null;
	subscriptionCreditsRemaining: number;
	purchasedCreditsRemaining: number;
	totalCreditsRemaining: number;
	subscriptionCreditsAllocated: number;
}

// Usage stats response from backend
export interface UserUsageStats {
	activeAlerts: number;
	alertsLimit: number;
	activeStrategyAlerts: number;
	strategyAlertsLimit: number;
}

// Combined subscription and usage response from backend
export interface CombinedSubscriptionAndUsage extends SubscriptionStatus, UserUsageStats { }

// Request/response types for API calls
export interface CreateCheckoutSessionArgs {
	priceId: string;
}

export interface CreateCreditCheckoutSessionArgs {
	priceId: string;
	creditAmount: number;
}

export interface VerifyCheckoutSessionArgs {
	sessionId: string;
}

// ===== STRIPE CLIENT UTILITIES =====

// Load Stripe.js with publishable key from environment
export async function getStripe() {
	if (!browser) return null;

	const stripeKey = import.meta.env.VITE_PUBLIC_STRIPE_KEY;
	if (!stripeKey) {
		console.warn('Stripe publishable key not found in environment variables');
		return null;
	}

	return await loadStripe(stripeKey);
}

// Redirect to Stripe Checkout
export async function redirectToCheckout(sessionId: string): Promise<void> {
	const stripe = await getStripe();
	if (!stripe) {
		throw new Error('Stripe failed to load');
	}

	const result = await stripe.redirectToCheckout({ sessionId });
	if (result.error) {
		throw new Error(result.error.message);
	}
}

// Open Stripe Customer Portal
export function redirectToCustomerPortal(portalUrl: string): void {
	if (browser) {
		window.location.href = portalUrl;
	}
}

// ===== UTILITY FUNCTIONS =====

// Get the appropriate price for a plan based on billing period
export function getPlanPrice(plan: DatabasePlan, billingPeriod: string): number {
	// Free plan always costs $0
	if (plan.product_key.toLowerCase() === 'free') {
		return 0;
	}

	// If no prices available, return 0
	if (!plan.prices || plan.prices.length === 0) {
		return 0;
	}

	// Convert billing period to match API format
	const apiPeriod = billingPeriod === 'month' ? 'monthly' : 'yearly';

	// Find the price for the requested billing period
	const priceInfo = plan.prices.find(p => p.billing_period === apiPeriod);

	if (priceInfo) {
		return priceInfo.price_cents;
	}

	// Fallback to first available price
	return plan.prices[0].price_cents;
}

// Get the appropriate price ID based on environment and billing period
export function getPriceId(plan: DatabasePlan, environment: string, billingPeriod: string = 'month'): string | null {
	// For Free plan, there's no price ID
	if (plan.product_key.toLowerCase() === 'free') {
		return null;
	}

	// If no prices available, return null
	if (!plan.prices || plan.prices.length === 0) {
		return null;
	}

	// Convert billing period to match API format
	const apiPeriod = billingPeriod === 'month' ? 'monthly' : 'yearly';

	// Find the price for the requested billing period
	const priceInfo = plan.prices.find(p => p.billing_period === apiPeriod);

	if (priceInfo) {
		return environment === 'test' ? priceInfo.stripe_price_id_test : priceInfo.stripe_price_id_live;
	}

	return null;
}

// Get the appropriate credit product price ID based on environment
export function getCreditPriceId(creditProduct: DatabaseCreditProduct, environment: string): string | null {
	return environment === 'test' ? creditProduct.stripe_price_id_test : creditProduct.stripe_price_id_live;
}

// Format price in cents to display price
export function formatPrice(priceCents: number, billingPeriod: string): string {
	if (priceCents === 0) return '$0';
	if (billingPeriod === 'year') {
		return `$${(priceCents / 100 / 12).toFixed(2).replace(/\.00$/, '')}`;
	}
	return `$${(priceCents / 100).toFixed(2).replace(/\.00$/, '')}`;
}

// Format price with currency symbol
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function formatPriceWithCurrency(priceCents: number, currency: string = 'USD'): string {
	const formatter = new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency: currency,
	});
	return formatter.format(priceCents / 100);
}
*/

// Get plan by product key
export function getPlanByKey(plans: DatabasePlan[], productKey: string): DatabasePlan | null {
	return plans.find(plan => plan.product_key.toLowerCase() === productKey.toLowerCase()) || null;
}

// Get credit product by product key
export function getCreditProductByKey(creditProducts: DatabaseCreditProduct[], productKey: string): DatabaseCreditProduct | null {
	return creditProducts.find(product => product.product_key === productKey) || null;
}

// Check if subscription is active
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function isSubscriptionActive(status: SubscriptionStatus): boolean {
	return status.isActive && !status.isCanceling;
}
*/

// Check if subscription is canceling
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function isSubscriptionCanceling(status: SubscriptionStatus): boolean {
	return status.isCanceling;
}
*/

// Get days until subscription ends (if canceling)
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function getDaysUntilSubscriptionEnds(status: SubscriptionStatus): number | null {
	if (!status.currentPeriodEnd) return null;

	const endDate = new Date(status.currentPeriodEnd * 1000);
	const now = new Date();
	const diffTime = endDate.getTime() - now.getTime();
	const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

	return diffDays > 0 ? diffDays : 0;
}
*/

// Format subscription period end date
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function formatSubscriptionEndDate(status: SubscriptionStatus): string | null {
	if (!status.currentPeriodEnd) return null;

	const endDate = new Date(status.currentPeriodEnd * 1000);
	return endDate.toLocaleDateString();
}
*/

// Calculate usage percentage
export function calculateUsagePercentage(used: number, limit: number): number {
	if (limit === 0) return 0;
	return Math.min((used / limit) * 100, 100);
}

// Check if user is approaching limits
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function isApproachingLimit(used: number, limit: number, threshold: number = 80): boolean {
	if (limit === 0) return false;
	return calculateUsagePercentage(used, limit) >= threshold;
}
*/

// Get billing period display name
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function getBillingPeriodDisplayName(billingPeriod: string): string {
	switch (billingPeriod) {
		case 'month':
			return 'Monthly';
		case 'year':
			return 'Yearly';
		case 'single':
			return 'One-time';
		default:
			return billingPeriod;
	}
}
*/

// Get plan display name (fallback to product key if needed)
export function getPlanDisplayName(plan: DatabasePlan): string {
	const displayNames: Record<string, string> = {
		'free': 'Free',
		'plus': 'Plus',
		'pro': 'Pro',
		'enterprise': 'Enterprise'
	};

	return displayNames[plan.product_key.toLowerCase()] || plan.product_key;
}

// Calculate annual savings percentage
// COMMENTED OUT: This function is exported but never used anywhere in the frontend
/*
export function calculateAnnualSavings(monthlyPriceCents: number, yearlyPriceCents: number): number {
	if (monthlyPriceCents === 0) return 0;
	const annualMonthlyPrice = monthlyPriceCents * 12;
	const savings = annualMonthlyPrice - yearlyPriceCents;
	return Math.round((savings / annualMonthlyPrice) * 100);
}
*/

// Get plan features dynamically from plan data
export function getPlanFeatures(plan: DatabasePlan): string[] {
	const features: string[] = [];

	// Data type feature - based on realtime_charts
	if (plan.realtime_charts) {
		features.push('Realtime data');
	} else {
		features.push('Delayed data');
	}

	// Queries limit
	if (plan.queries_limit > 0) {
		features.push(`${plan.queries_limit} queries/mo`);
	}

	// Strategy alerts limit
	if (plan.strategy_alerts_limit > 0) {
		features.push(`${plan.strategy_alerts_limit} active strategies`);
	}

	// Strategy screening type
	if (plan.multi_strategy_screening) {
		features.push('Multi strategy screening');
	} else if (plan.strategy_alerts_limit > 0) {
		features.push('Single strategy screening');
	}

	// Alerts limit
	if (plan.alerts_limit > 0) {
		features.push(`${plan.alerts_limit} news or price alerts`);
	}

	// Watchlist alerts
	if (plan.watchlist_alerts) {
		features.push('Watchlist alerts');
	}

	// Multi chart layouts
	if (plan.multi_chart) {
		features.push('Multi chart layouts');
	}

	// Sub-minute charts
	if (plan.sub_minute_charts) {
		features.push('Sub-minute charts');
	}

	return features;
}

// Get credit product display name (hardcoded mapping)
export function getCreditProductDisplayName(creditProduct: DatabaseCreditProduct): string {
	const displayNames: Record<string, string> = {
		'credits100': '100 Credits',
		'credits250': '250 Credits',
		'credits1000': '1000 Credits'
	};

	return displayNames[creditProduct.product_key] || `${creditProduct.credit_amount} Credits`;
}
