// Database-driven pricing configuration
import { browser } from '$app/environment';
import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
import { getPlanFeatures } from './plan-features';

// Backend structure for subscription products (matches backend SubscriptionProduct)
export interface BackendSubscriptionProduct {
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

// Backend structure for prices (matches backend Price)
export interface BackendPrice {
	id: number;
	price_cents: number;
	stripe_price_id_live?: string;
	stripe_price_id_test?: string;
	product_id: number;
	billing_period: string;
	created_at: string;
	updated_at: string;
}

// Backend structure for subscription plans with pricing (matches backend SubscriptionPlanWithPricing)
export interface BackendSubscriptionPlanWithPricing {
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
	prices: BackendPrice[];
}

// Backend structure for credit products (matches backend CreditProduct)
export interface BackendCreditProduct {
	id: number;
	product_key: string;
	stripe_price_id_test?: string;
	stripe_price_id_live?: string;
	credit_amount: number;
	is_active: boolean;
	created_at: string;
	updated_at: string;
}

// Frontend-facing interfaces with hardcoded display information
export interface DatabasePlan {
	id: number;
	plan_name: string;
	stripe_price_id_test?: string;
	stripe_price_id_live?: string;
	display_name: string;
	description?: string;
	price_cents: number;
	billing_period: string;
	credits_per_billing_period: number;
	alerts_limit: number;
	strategy_alerts_limit: number;
	is_active: boolean;
	is_popular: boolean;
	sort_order: number;
	created_at: string;
	updated_at: string;
}

export interface DatabaseCreditProduct {
	id: number;
	product_key: string;
	stripe_price_id_test?: string;
	stripe_price_id_live?: string;
	display_name: string;
	description?: string;
	credit_amount: number;
	price_cents: number;
	is_active: boolean;
	is_popular: boolean;
	sort_order: number;
	created_at: string;
	updated_at: string;
}

export interface PricingConfiguration {
	plans: DatabasePlan[];
	creditProducts: DatabaseCreditProduct[];
	environment: string;
}

// Hardcoded display information for plans
const PLAN_DISPLAY_INFO: Record<string, {
	display_name: string;
	description?: string;
	is_popular: boolean;
	sort_order: number;
}> = {
	'Free': {
		display_name: 'Free',
		description: 'Perfect for getting started',
		is_popular: false,
		sort_order: 1
	},
	'Plus': {
		display_name: 'Plus',
		description: 'Great for active traders',
		is_popular: true,
		sort_order: 2
	},
	'Pro': {
		display_name: 'Pro',
		description: 'For professional traders',
		is_popular: false,
		sort_order: 3
	}
};

// Hardcoded display information for credit products
const CREDIT_PRODUCT_DISPLAY_INFO: Record<string, {
	display_name: string;
	description?: string;
	is_popular: boolean;
	sort_order: number;
}> = {
	'credits100': {
		display_name: '100 Credits',
		description: 'Perfect for occasional use',
		is_popular: false,
		sort_order: 1
	},
	'credits250': {
		display_name: '250 Credits',
		description: 'Great value for regular users',
		is_popular: true,
		sort_order: 2
	},
	'credits1000': {
		display_name: '1000 Credits',
		description: 'Best for heavy users',
		is_popular: false,
		sort_order: 3
	}
};

interface CachedPricingData {
	config: PricingConfiguration;
	timestamp: number;
}

// Session storage keys
const PRICING_CACHE_KEY = 'pricing_configuration';
const PRICING_CACHE_EXPIRY = 24 * 60 * 60 * 1000; // 1 day in milliseconds

// Global pricing configuration store
let pricingConfig: PricingConfiguration | null = null;
let configPromise: Promise<PricingConfiguration> | null = null;

// Check if cached pricing data is still valid
function isCachedPricingValid(): boolean {
	if (!browser) return false;

	const cached = sessionStorage.getItem(PRICING_CACHE_KEY);
	if (!cached) return false;

	try {
		const data: CachedPricingData = JSON.parse(cached);
		const now = Date.now();
		return (now - data.timestamp) < PRICING_CACHE_EXPIRY;
	} catch {
		return false;
	}
}

// Get cached pricing data
function getCachedPricing(): PricingConfiguration | null {
	if (!browser) return null;

	try {
		const cached = sessionStorage.getItem(PRICING_CACHE_KEY);
		if (!cached) return null;

		const data: CachedPricingData = JSON.parse(cached);
		return data.config;
	} catch {
		return null;
	}
}

// Cache pricing data in session storage
function cachePricingData(config: PricingConfiguration): void {
	if (!browser) return;

	try {
		const data: CachedPricingData = {
			config,
			timestamp: Date.now()
		};
		sessionStorage.setItem(PRICING_CACHE_KEY, JSON.stringify(data));
	} catch (error) {
		console.warn('Failed to cache pricing data:', error);
	}
}

// Clear cached pricing data
function clearCachedPricing(): void {
	if (!browser) return;
	sessionStorage.removeItem(PRICING_CACHE_KEY);
}

// Transform backend data to frontend format
function transformBackendDataToFrontend(
	backendPlans: BackendSubscriptionPlanWithPricing[],
	backendCreditProducts: BackendCreditProduct[],
	environment: string
): PricingConfiguration {
	const plans: DatabasePlan[] = [];

	// Add Free plan (hardcoded since it's not in the backend)
	const freePlan: DatabasePlan = {
		id: 0,
		plan_name: 'Free',
		stripe_price_id_test: undefined,
		stripe_price_id_live: undefined,
		display_name: 'Free',
		description: 'Perfect for getting started',
		price_cents: 0,
		billing_period: 'month',
		credits_per_billing_period: 5,
		alerts_limit: 0,
		strategy_alerts_limit: 0,
		is_active: true,
		is_popular: false,
		sort_order: 1,
		created_at: new Date().toISOString(),
		updated_at: new Date().toISOString()
	};
	plans.push(freePlan);

	// Filter out invalid backend plans and deduplicate
	const validBackendPlans = backendPlans.filter(plan =>
		plan.product_key &&
		plan.product_key !== 'Free' &&
		plan.prices &&
		plan.prices.length > 0
	);

	// Transform subscription plans
	for (const backendPlan of validBackendPlans) {
		const displayInfo = PLAN_DISPLAY_INFO[backendPlan.product_key] || {
			display_name: backendPlan.product_key,
			description: '',
			is_popular: false,
			sort_order: 999
		};

		// Create a plan for each billing period
		for (const price of backendPlan.prices) {
			// Skip invalid prices
			if (!price.price_cents || price.price_cents <= 0) {
				continue;
			}

			const plan: DatabasePlan = {
				id: backendPlan.id,
				plan_name: backendPlan.product_key,
				stripe_price_id_test: price.stripe_price_id_test,
				stripe_price_id_live: price.stripe_price_id_live,
				display_name: price.billing_period === 'yearly' ? `${displayInfo.display_name} Yearly` : displayInfo.display_name,
				description: displayInfo.description,
				price_cents: price.price_cents,
				billing_period: price.billing_period,
				credits_per_billing_period: backendPlan.credits_per_month,
				alerts_limit: backendPlan.alerts_limit,
				strategy_alerts_limit: backendPlan.strategy_alerts_limit,
				is_active: true, // Only active plans are returned from backend
				is_popular: displayInfo.is_popular && price.billing_period === 'yearly', // Only yearly plans are popular
				sort_order: displayInfo.sort_order + (price.billing_period === 'yearly' ? 0.5 : 0), // Yearly plans come after monthly
				created_at: backendPlan.created_at,
				updated_at: backendPlan.updated_at
			};
			plans.push(plan);
		}
	}

	// Filter out invalid credit products and deduplicate
	const validCreditProducts = backendCreditProducts.filter(product =>
		product.product_key &&
		product.credit_amount &&
		product.credit_amount > 0 &&
		product.is_active
	);

	// Transform credit products
	const creditProducts: DatabaseCreditProduct[] = validCreditProducts.map(backendProduct => {
		const displayInfo = CREDIT_PRODUCT_DISPLAY_INFO[backendProduct.product_key] || {
			display_name: backendProduct.product_key,
			description: '',
			is_popular: false,
			sort_order: 999
		};

		return {
			id: backendProduct.id,
			product_key: backendProduct.product_key,
			stripe_price_id_test: backendProduct.stripe_price_id_test,
			stripe_price_id_live: backendProduct.stripe_price_id_live,
			display_name: displayInfo.display_name,
			description: displayInfo.description,
			credit_amount: backendProduct.credit_amount,
			price_cents: 0, // Credit products don't need price_cents in the frontend interface
			is_active: backendProduct.is_active,
			is_popular: displayInfo.is_popular,
			sort_order: displayInfo.sort_order,
			created_at: backendProduct.created_at,
			updated_at: backendProduct.updated_at
		};
	});

	// Deduplicate plans by plan_name + billing_period combination
	const deduplicatedPlans = plans.filter((plan, index, self) =>
		index === self.findIndex(p =>
			p.plan_name === plan.plan_name &&
			p.billing_period === plan.billing_period
		)
	);

	// Deduplicate credit products by product_key
	const deduplicatedCreditProducts = creditProducts.filter((product, index, self) =>
		index === self.findIndex(p => p.product_key === product.product_key)
	);

	return {
		plans: deduplicatedPlans.sort((a, b) => a.sort_order - b.sort_order),
		creditProducts: deduplicatedCreditProducts.sort((a, b) => a.sort_order - b.sort_order),
		environment
	};
}

// Fetch pricing configuration from the database
export async function fetchPricingConfiguration(): Promise<PricingConfiguration> {
	// Return cached config if available
	if (pricingConfig) {
		return pricingConfig;
	}

	// Check session storage cache first
	if (isCachedPricingValid()) {
		const cached = getCachedPricing();
		if (cached) {
			pricingConfig = cached;
			return cached;
		}
	}

	// Return existing promise if already fetching
	if (configPromise) {
		return configPromise;
	}

	// Create new fetch promise
	configPromise = (async () => {
		try {
			if (!browser) {
				throw new Error('Pricing configuration not available during server-side rendering');
			}

			// Always use public endpoint for pricing configuration
			const backendResponse = await publicRequest<{
				plans: BackendSubscriptionPlanWithPricing[];
				creditProducts: BackendCreditProduct[];
				environment: string;
			}>('getPublicPricingConfiguration', {});

			// Transform backend data to frontend format
			const config = transformBackendDataToFrontend(
				backendResponse.plans,
				backendResponse.creditProducts,
				backendResponse.environment
			);

			// Cache the result in memory and session storage
			pricingConfig = config;
			cachePricingData(config);
			return config;
		} catch (error) {
			console.error('Failed to fetch pricing configuration from API:', error);
			throw error;
		} finally {
			configPromise = null;
		}
	})();

	return configPromise;
}

// Preload pricing configuration (call this early in the app lifecycle)
export async function preloadPricingConfiguration(): Promise<void> {
	if (!browser) return;

	try {
		// Check if we already have valid cached data
		if (isCachedPricingValid()) {
			const cached = getCachedPricing();
			if (cached) {
				pricingConfig = cached;
				return;
			}
		}

		// Fetch fresh data in the background
		await fetchPricingConfiguration();
	} catch (error) {
		console.warn('Failed to preload pricing configuration:', error);
		// Don't throw - this is a background operation
	}
}

// Clear cached configuration (useful for testing or when config changes)
export function clearPricingCache(): void {
	pricingConfig = null;
	configPromise = null;
	clearCachedPricing();
}

// Get plan by key from database configuration
export async function getPlan(planKey: string): Promise<DatabasePlan | null> {
	const config = await fetchPricingConfiguration();
	return config.plans.find(plan => plan.plan_name.toLowerCase() === planKey.toLowerCase()) || null;
}

// Get credit product by key from database configuration
export async function getCreditProduct(productKey: string): Promise<DatabaseCreditProduct | null> {
	const config = await fetchPricingConfiguration();
	return config.creditProducts.find(product => product.product_key === productKey) || null;
}

// Get Stripe price ID for a plan based on environment
export async function getStripePriceForPlan(planKey: string): Promise<string | null> {
	const plan = await getPlan(planKey);
	if (!plan) return null;

	const config = await fetchPricingConfiguration();
	const environment = config.environment;

	return environment === 'test' ? plan.stripe_price_id_test || null : plan.stripe_price_id_live || null;
}

// Get Stripe price ID for a credit product based on environment
export async function getStripePriceForCreditProduct(productKey: string): Promise<string | null> {
	const product = await getCreditProduct(productKey);
	if (!product) return null;

	const config = await fetchPricingConfiguration();
	const environment = config.environment;

	return environment === 'test' ? product.stripe_price_id_test || null : product.stripe_price_id_live || null;
}

// Format price from cents to display format
export function formatPrice(cents: number, billingPeriod: string): string {
	if (cents === 0) return '$0';
	if (billingPeriod === 'year') {
		return `$${(cents / 100 / 12).toFixed(2).replace(/\.00$/, '')}`;
	}
	return `$${(cents / 100).toFixed(2).replace(/\.00$/, '')}`;
}

export function getPlanFeaturesForPlan(plan: DatabasePlan): string[] {
	return getPlanFeatures(plan.plan_name);
}

