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

export const load: ServerLoad = async ({request, url}) => {
	const userAgent = request.headers.get('user-agent') || '';
    const referrer = request.headers.get('referer') || '';
    const path = url.pathname;
	try {
		const backendUrl = process.env.BACKEND_URL || 'http://backend:5058';
        const cfIP = request.headers.get('cf-connecting-ip') || '127.0.0.1';
        const forwarded = request.headers.get('x-forwarded-for') || '127.0.0.1';
        
        const response = await fetch(`${backendUrl}/frontend/server`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Peripheral-Frontend-Key': 'williamsIsTheBestLiberalArtsCollege!1@', // TODO: Move to environment variable
            },
            body: JSON.stringify({
                func: 'logSplashScreenView',
                args: {
                    path: path,
                    referrer: referrer,
                    user_agent: userAgent,
                    ip_address: forwarded,
                    cloudflare_ipv6: cfIP,
                    timestamp: new Date().toISOString(),
                }
            }),
            // Add timeout to prevent hanging
            signal: AbortSignal.timeout(2000) // 2 second timeout
        });
		if (!response.ok) {
			const errorBody = await response.text().catch(() => 'Unable to read error body');
            console.error(`Failed to log page view: ${response.status} ${response.statusText} - ${errorBody}`);
		}
		
		console.log('🔄 [pricing server] Loading pricing configuration...');
		
		// Fetch pricing configuration from backend
		const config = await publicRequest<PublicPricingConfiguration>(
			'getPublicPricingConfiguration',
			{}
		);


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