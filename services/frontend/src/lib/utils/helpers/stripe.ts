import { loadStripe } from '@stripe/stripe-js';
import { browser } from '$app/environment';

// ===== TYPE DEFINITIONS =====
// These interfaces match the backend Go structs exactly

export interface Price {
	id: number;
	price_cents: number;
	stripe_price_id_live: string | null;
	stripe_price_id_test: string | null;
	product_id: number;
	billing_period: string;
	created_at: string;
	updated_at: string;
}

export interface SubscriptionProduct {
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
	credits_per_month: number;
	created_at: string;
	updated_at: string;
}

export interface SubscriptionPlanWithPricing extends SubscriptionProduct {
	prices: Price[];
}

export interface CreditProduct {
	id: number;
	product_key: string;
	stripe_price_id_test: string | null;
	stripe_price_id_live: string | null;
	credit_amount: number;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

export interface PublicPricingConfiguration {
	plans: SubscriptionPlanWithPricing[];
	creditProducts: CreditProduct[];
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

// Get the appropriate price ID based on environment and billing period
export function getPriceId(prices: Price[], billingPeriod: string, environment: string): string | null {
	const price = prices.find(p => p.billing_period === billingPeriod);
	if (!price) return null;

	return environment === 'test' ? price.stripe_price_id_test : price.stripe_price_id_live;
}

// Get the appropriate credit product price ID based on environment
export function getCreditPriceId(creditProduct: CreditProduct, environment: string): string | null {
	return environment === 'test' ? creditProduct.stripe_price_id_test : creditProduct.stripe_price_id_live;
}

// Format price in cents to display price
export function formatPrice(priceCents: number): string {
	return (priceCents / 100).toFixed(2);
}

// Format price with currency symbol
export function formatPriceWithCurrency(priceCents: number, currency: string = 'USD'): string {
	const formatter = new Intl.NumberFormat('en-US', {
		style: 'currency',
		currency: currency,
	});
	return formatter.format(priceCents / 100);
}

// Get plan by product key
export function getPlanByKey(plans: SubscriptionPlanWithPricing[], productKey: string): SubscriptionPlanWithPricing | null {
	return plans.find(plan => plan.product_key === productKey) || null;
}

// Get credit product by product key
export function getCreditProductByKey(creditProducts: CreditProduct[], productKey: string): CreditProduct | null {
	return creditProducts.find(product => product.product_key === productKey) || null;
}

// Check if subscription is active
export function isSubscriptionActive(status: SubscriptionStatus): boolean {
	return status.isActive && !status.isCanceling;
}

// Check if subscription is canceling
export function isSubscriptionCanceling(status: SubscriptionStatus): boolean {
	return status.isCanceling;
}

// Get days until subscription ends (if canceling)
export function getDaysUntilSubscriptionEnds(status: SubscriptionStatus): number | null {
	if (!status.currentPeriodEnd) return null;

	const endDate = new Date(status.currentPeriodEnd * 1000);
	const now = new Date();
	const diffTime = endDate.getTime() - now.getTime();
	const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

	return diffDays > 0 ? diffDays : 0;
}

// Format subscription period end date
export function formatSubscriptionEndDate(status: SubscriptionStatus): string | null {
	if (!status.currentPeriodEnd) return null;

	const endDate = new Date(status.currentPeriodEnd * 1000);
	return endDate.toLocaleDateString();
}

// Calculate usage percentage
export function calculateUsagePercentage(used: number, limit: number): number {
	if (limit === 0) return 0;
	return Math.min((used / limit) * 100, 100);
}

// Check if user is approaching limits
export function isApproachingLimit(used: number, limit: number, threshold: number = 80): boolean {
	if (limit === 0) return false;
	return calculateUsagePercentage(used, limit) >= threshold;
}

// Get billing period display name
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

// Get plan display name (fallback to product key if needed)
export function getPlanDisplayName(plan: SubscriptionPlanWithPricing): string {
	// You can customize this based on your product keys
	switch (plan.product_key) {
		case 'free':
			return 'Free';
		case 'basic':
			return 'Basic';
		case 'pro':
			return 'Pro';
		case 'enterprise':
			return 'Enterprise';
		default:
			return plan.product_key.charAt(0).toUpperCase() + plan.product_key.slice(1);
	}
}

// Calculate annual savings percentage
export function calculateAnnualSavings(monthlyPrice: number, yearlyPrice: number): number {
	if (monthlyPrice === 0) return 0;
	const annualMonthlyPrice = monthlyPrice * 12;
	const savings = annualMonthlyPrice - yearlyPrice;
	return Math.round((savings / annualMonthlyPrice) * 100);
}

// Get the best price for a plan (usually the yearly price if available)
export function getBestPrice(plan: SubscriptionPlanWithPricing, environment: string): { price: Price; savings?: number } | null {
	const yearlyPrice = plan.prices.find(p => p.billing_period === 'year');
	const monthlyPrice = plan.prices.find(p => p.billing_period === 'month');

	if (yearlyPrice && monthlyPrice) {
		const savings = calculateAnnualSavings(monthlyPrice.price_cents, yearlyPrice.price_cents);
		return { price: yearlyPrice, savings };
	}

	if (yearlyPrice) {
		return { price: yearlyPrice };
	}

	if (monthlyPrice) {
		return { price: monthlyPrice };
	}

	return plan.prices.length > 0 ? { price: plan.prices[0] } : null;
}
