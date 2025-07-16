import type { ServerLoad } from '@sveltejs/kit';
import { publicRequest } from '$lib/utils/helpers/backend';

// ===== TYPE DEFINITIONS =====
interface DatabasePlan {
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
	prices: Array<{
		id: number;
		price_cents: number;
		stripe_price_id_live: string | null;
		stripe_price_id_test: string | null;
		billing_period: string;
	}> | null;
	created_at: string;
	updated_at: string;
}

interface DatabaseCreditProduct {
	id: number;
	product_key: string;
	credit_amount: number;
	price_cents: number;
	stripe_price_id_test: string | null;
	stripe_price_id_live: string | null;
	created_at: string;
	updated_at: string;
}

interface PublicPricingConfiguration {
	plans: DatabasePlan[];
	creditProducts: DatabaseCreditProduct[];
	environment: string;
}

export const load: ServerLoad = async () => {
	try {
		console.log('🔄 [pricing server] Loading pricing configuration...');
		
		// Fetch pricing configuration from backend
		const config = await publicRequest<PublicPricingConfiguration>(
			'getPublicPricingConfiguration',
			{}
		);

		console.log('✅ [pricing server] Pricing configuration loaded:', {
			plansCount: config.plans.length,
			creditProductsCount: config.creditProducts.length,
			environment: config.environment
		});

		return {
			plans: config.plans,
			creditProducts: config.creditProducts,
			environment: config.environment
		};
	} catch (error) {
		console.error('❌ [pricing server] Failed to load pricing configuration:', error);
		
		// Return error state that the client can handle
		return {
			plans: [],
			creditProducts: [],
			environment: 'test',
			pricingError: 'Failed to load pricing information. Please refresh the page.'
		};
	}
}; 