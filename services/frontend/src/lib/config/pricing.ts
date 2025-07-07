// Database-driven pricing configuration
import { browser } from '$app/environment';
import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';

// Database-driven pricing configuration
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
	features: string[];
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
			const config = await publicRequest<PricingConfiguration>('getPublicPricingConfiguration', {});

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


