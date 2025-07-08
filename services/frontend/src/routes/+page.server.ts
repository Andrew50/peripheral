import type { ServerLoad } from '@sveltejs/kit';
import { publicRequest } from '$lib/utils/helpers/backend';

// ---- In-memory daily cache ----------------------------------------------
// this is for the splash screen timeline chart
let cached: any = null;
let lastFetched = 0;
const ONE_DAY_MS = 24 * 60 * 60 * 1000;
// Helper to build a unique key for each chart slice (ticker + timestamp + timeframe)
function ChartKey(ticker: string, timestampMs: number, timeframe: string): string {
	return `${ticker}_${timestampMs}_${timeframe}`;
}

// Describe every slice the hero timeline (or other splash logic) might need.
// Add new entries as required – they will all be fetched once per day and
// shipped to the client in a single payload.
const CHART_SPECS = [
	{ ticker: 'QQQ', timestampMs: 0, timeframe: '1d', extendedhours: false, direction: 'backward' },
	{ ticker: 'QQQ', timestampMs: Date.UTC(2019, 8, 1), timeframe: '1d', extendedhours: false, direction: 'forward' },
	{ ticker: 'WMT', timestampMs: Date.UTC(2019, 8, 1), timeframe: '1d', extendedhours: false, direction: 'forward' },
	{ ticker: 'COST', timestampMs: Date.UTC(2019, 8, 1), timeframe: '1d', extendedhours: false, direction: 'forward' },
  // { ticker: 'QQQ', timestampMs: Date.UTC(2019, 7, 23), timeframe: '1d', extendedhours: false },
  // { ticker: 'WMT', timestampMs: 0, timeframe: '1d', extendedhours: false },
  // { ticker: 'COST', timestampMs: 0, timeframe: '1d', extendedhours: false },
];

// Which chart the client should show first
const DEFAULT_KEY = ChartKey('QQQ', 0, '1d');
const DEFAULT_SPLASH_BARS = 400;

interface ChartDataResponse {
	bars: unknown[];
	isEarliestData?: boolean;
}

interface SecurityIdResponse {
	securityId?: number;
}
async function getChartDataForSplashScreen(
	/** Ticker symbol to fetch */ ticker: string,
	/** Anchor timestamp (ms) – 0 = now */ timestampMs: number,
	/** Timeframe, e.g. "1d" */ timeframe: string,
	/** Include pre/post-market? */ extendedhours: boolean,
	/** Direction of the chart */ direction: 'backward' | 'forward',
	/** Number of bars to request */ bars: number = DEFAULT_SPLASH_BARS
) {

	const { securityId = 0 } = await publicRequest<SecurityIdResponse>(
		'getSecurityIDFromTickerTimestamp',
		{ ticker, timestampMs }
	);

	if (!securityId) {
		throw new Error(`Failed to retrieve securityId for ${ticker}`);
	}

	const chartData = await publicRequest<ChartDataResponse>('getChartData', {
		securityId,
		timeframe,
		timestamp: timestampMs,
		direction: 'backward',
		bars, // fetch a larger window for hero timeline
		extendedhours,
		isreplay: false
	});

	return { securityId, chartData };
}
// Utility to refresh the cached SPY 1-day chart data
async function refreshCache() {
	// Fetch all requested slices in parallel
	const sliceEntries = await Promise.all(
		CHART_SPECS.map(async ({ ticker, timestampMs, timeframe, extendedhours, direction }) => {
			const { securityId, chartData } = await getChartDataForSplashScreen(
				ticker,
				timestampMs,
				timeframe,
				extendedhours,
				direction,
				DEFAULT_SPLASH_BARS
			);
			const key = ChartKey(ticker, timestampMs, timeframe);
			return [
				key,
				{
					ticker,
					securityId,
					timeframe,
					timestamp: timestampMs,
					bars: chartData?.bars?.length ?? DEFAULT_SPLASH_BARS,
					chartData
				}
			] as const;
		})
	);

	const data = Object.fromEntries(sliceEntries);

	// Preserve backwards-compatibility: expose one default slice separately
	const defaultSlice = data[DEFAULT_KEY];

	cached = {
		defaultKey: DEFAULT_KEY,
		chartsByKey: data,
		defaultChartData: defaultSlice
	};

	lastFetched = Date.now();
}

export const load: ServerLoad = async ({ setHeaders }) => {
	const now = Date.now();
	if (!cached || now - lastFetched > ONE_DAY_MS) {
		await refreshCache();
	}

	// Tell upstream caches (CDN, proxies) they may store this JSON for 24 h.
	setHeaders({ 'Cache-Control': 'public, max-age=0, s-maxage=86400' });

	// Return the full cache object; keep legacy `defaultChartData` for existing client code
	return cached;
};
