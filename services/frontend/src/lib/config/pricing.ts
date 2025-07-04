// Centralized pricing configuration with environment-aware Stripe price IDs

// Environment detection - uses build-time environment variable injection
function getEnvironment(): 'production' | 'development' {
	// Use Vite's environment variable injection at build time
	// This is much more reliable than runtime URL detection
	const environment = (import.meta as any).env?.VITE_ENVIRONMENT || (import.meta as any).env?.MODE || 'development';
	return environment === 'prod' ? 'production' : 'development';
}

// Environment-specific Stripe Price IDs
// Note: Only paid tiers (plus, pro) have Stripe price IDs since free tier doesn't use Stripe
const STRIPE_PRICE_IDS = {
	development: {
		// Test mode price IDs for local, staging, demo environments
		plus: 'price_1RhAuSGCLCMUwjFlh3wPqWyo',  // Test Plus price ID
		pro: 'price_1RhAucGCLCMUwjFli0rmWtIe'   // Test Pro price ID
	},
	production: {
		// Live mode price IDs for production
		plus: 'price_1Rgsj1GCLCMUwjFlf4U6jMRt',   // Your current live Plus price ID
		pro: 'price_1RgsiRGCLCMUwjFljLLknvSu'     // Your current live Pro price ID
	}
};

export const PRICING_CONFIG = {
	// Plan configurations with pricing and features
	PLANS: {
		free: {
			name: 'Free',
			price: 0,
			period: '/month',
			description: 'Basic access to get started',
			features: ['Delayed charting', '5 queries', 'Watchlists'],
			cta: 'Current Plan',
			disabled: true
		},
		plus: {
			name: 'Plus',
			price: 99,
			period: '/month',
			description: 'Perfect for active traders',
			features: [
				'Realtime charting',
				'250 queries',
				'5 strategy alerts',
				'Single strategy screening',
				'100 news or price alerts'
			],
			cta: 'Choose Plus',
			priceId: 'plus' // References environment-specific price ID
		},
		pro: {
			name: 'Pro',
			price: 199,
			period: '/month',
			description: 'Advanced features for professional traders',
			features: [
				'Sub 1 minute charting',
				'Multi chart',
				'1000 queries',
				'20 strategy alerts',
				'Multi strategy screening',
				'400 alerts',
				'Watchlist alerts'
			],
			cta: 'Choose Pro',
			priceId: 'pro', // References environment-specific price ID
			popular: true
		}
	}
} as const;

// Helper function to get plan by key
export function getPlan(planKey: keyof typeof PRICING_CONFIG.PLANS) {
	return PRICING_CONFIG.PLANS[planKey];
}

// Helper function to get environment-appropriate Stripe price ID
// Note: Only for paid tiers (plus, pro) - free tier doesn't use Stripe
export function getStripePrice(planKey: 'plus' | 'pro'): string {
	const environment = getEnvironment();
	const priceId = STRIPE_PRICE_IDS[environment][planKey];

	// Debug logging for development
	if (environment === 'development') {
		console.log(`[Stripe] Using ${environment} price ID for ${planKey}: ${priceId}`);
	}

	return priceId;
}

// Helper function to format price
export function formatPrice(price: number): string {
	return `$${price}`;
}

// Helper function to get current environment (for debugging)
export function getCurrentEnvironment(): string {
	return getEnvironment();
}
