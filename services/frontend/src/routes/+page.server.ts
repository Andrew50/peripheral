import type { ServerLoad } from '@sveltejs/kit';
import { publicRequest } from '$lib/utils/helpers/backend';

// ---- In-memory daily cache ----------------------------------------------
// this is for the splash screen timeline chart
let cached: any = null;
let lastFetched = 0;
const ONE_DAY_MS = 24 * 60 * 60 * 1000;

// Utility to refresh the cached SPY 1-day chart data
async function refreshCache() {
	// 1. Resolve SPY securityId (once per refresh)
	interface SecurityIdResponse {
		securityId?: number;
	}
	const { securityId = 0 } = await publicRequest<SecurityIdResponse>(
		'getSecurityIDFromTickerTimestamp',
		{ ticker: 'SPY', timestampMs: 0 }
	);

	// Safety fallback – if backend did not return id
	if (!securityId) {
		throw new Error('Failed to retrieve securityId for SPY');
	}

	// 2. Fetch 1-day chart data for SPY starting at timestamp 0 (latest)
	interface ChartDataResponse {
		bars: unknown[];
		isEarliestData?: boolean;
	}
	const chartData = await publicRequest<ChartDataResponse>('getChartData', {
		securityId,
		timeframe: '1d',
		timestamp: 0,
		direction: 'backward',
		bars: 300, // fetch a reasonable default window
		extendedhours: false,
		isreplay: false
	});
	cached = {
		ticker: 'SPY',
		securityId,
		timeframe: '1d',
		timestamp: 0,
		price: 0,
		bars: 300,
		chartData
	};
	lastFetched = Date.now();
}

export const load: ServerLoad = async ({ setHeaders }) => {
	const now = Date.now();
	if (!cached || now - lastFetched > ONE_DAY_MS) {
		await refreshCache();
	}

	// Tell upstream caches (CDN, proxies) they may store this JSON for 24h.
	setHeaders({ 'Cache-Control': 'public, max-age=0, s-maxage=86400' });

	return {
		defaultChartData: cached
	};
};
