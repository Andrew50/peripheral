import { writable, type Writable } from 'svelte/store';

type ColumnType = 'price' | 'changePct' | 'marketCap' | 'prevClose' | 'chgExt' | 'change';
const stores = new Map<string, Writable<any>>();

// Cache for calculating changes - separate for regular and extended
const priceCache = new Map<number, number>();
const extendedPriceCache = new Map<number, number>();
const prevCloseCache = new Map<number, number>();
const extendedCloseCache = new Map<number, number>();

export const getColumnStore = (securityid: number, m: ColumnType) => {
	const k = `${securityid}:${m}`;
	if (!stores.has(k)) stores.set(k, writable({}));
	return stores.get(k)!;
};

const refCounts = new Map<number, number>();
import { subscribe, unsubscribe } from './socket';

// Helper to fetch initial close data
async function fetchCloseData(securityId: number) {
	try {
		subscribe(`${securityId}-close-regular`);
		subscribe(`${securityId}-close-extended`);
	} catch (error) {
		console.warn('Failed to fetch close data for security', securityId, error);
	}
}

export function register(securityid: number) {
	const n = (refCounts.get(securityid) ?? 0) + 1;
	refCounts.set(securityid, n);
	if (n === 1) {
		subscribe(`${securityid}-slow-regular`);
		subscribe(`${securityid}-slow-extended`);
		fetchCloseData(securityid);
	}
	return () => {
		const m = (refCounts.get(securityid) ?? 1) - 1;
		if (m <= 0) {
			refCounts.delete(securityid);
			unsubscribe(`${securityid}-slow-regular`);
			unsubscribe(`${securityid}-slow-extended`);
			unsubscribe(`${securityid}-close-regular`);
			unsubscribe(`${securityid}-close-extended`);
			// Clean up all caches for this security
			priceCache.delete(securityid);
			extendedPriceCache.delete(securityid);
			prevCloseCache.delete(securityid);
			extendedCloseCache.delete(securityid);
			// Clean up stores
			(
				['price', 'change', 'changePct', 'marketCap', 'prevClose', 'chgExt'] as ColumnType[]
			).forEach((k) => stores.delete(`${securityid}:${k}`));
		} else refCounts.set(securityid, m);
	};
}

let dirty = false;
// Separate data structures for regular and extended hours
const latestRegular = new Map<number, any>();
const latestExtended = new Map<number, any>();

export function enqueueTick(t: any) {
	const securityid = t.securityid;

	// Route to appropriate data structure based on data type
	if (t.isExtended || t.extendedClose !== undefined) {
		// Extended hours data
		latestExtended.set(securityid, { ...latestExtended.get(securityid), ...t });
	} else {
		// Regular hours data
		latestRegular.set(securityid, { ...latestRegular.get(securityid), ...t });
	}

	if (!dirty) {
		dirty = true;
		requestAnimationFrame(flush);
	}
}

function flush() {
	dirty = false;

	// Process regular hours data
	for (const t of latestRegular.values()) {
		processRegularHoursData(t);
	}

	// Process extended hours data
	for (const t of latestExtended.values()) {
		processExtendedHoursData(t);
	}

	latestRegular.clear();
	latestExtended.clear();
}

function processRegularHoursData(t: any) {
	const securityid = t.securityid;

	// Update regular price cache and store
	if (t.price !== undefined) {
		// Skip price updates if price is -1 (indicates skip OHLC condition)
		if (t.price >= 0) {
			priceCache.set(securityid, t.price);
			getColumnStore(securityid, 'price').set({
				price: t.price,
				formatted: t.price.toFixed(2)
			});
		}
	}

	// Update prevClose cache and store
	if (t.prevClose !== undefined) {
		prevCloseCache.set(securityid, t.prevClose);
		getColumnStore(securityid, 'prevClose').set({ prevClose: t.prevClose });
	}

	// Calculate and update regular change and change percentage
	const currentPrice = priceCache.get(securityid);
	const prevClose = prevCloseCache.get(securityid);
	if (currentPrice !== undefined && prevClose !== undefined && prevClose !== 0) {
		const change = currentPrice - prevClose;
		const changePct = (currentPrice / prevClose - 1) * 100;

		getColumnStore(securityid, 'change').set({
			change: change,
			formatted: change.toFixed(2)
		});

		getColumnStore(securityid, 'changePct').set({
			pct: changePct,
			formatted: changePct.toFixed(2) + '%'
		});
	}

	// Handle market cap
	if (t.marketCap !== undefined) {
		getColumnStore(securityid, 'marketCap').set({ marketCap: t.marketCap });
	}
}

function processExtendedHoursData(t: any) {
	const securityid = t.securityid;

	// Update extended price cache
	if (t.price !== undefined) {
		// Skip price updates if price is -1 (indicates skip OHLC condition)
		if (t.price >= 0) {
			extendedPriceCache.set(securityid, t.price);
		}
	}

	// Update extendedClose cache
	if (t.extendedClose !== undefined) {
		extendedCloseCache.set(securityid, t.extendedClose);
	}

	// Calculate and update extended hours change
	const currentExtendedPrice = extendedPriceCache.get(securityid);
	const extendedClose = extendedCloseCache.get(securityid);

	if (currentExtendedPrice !== undefined && extendedClose !== undefined && extendedClose !== 0) {
		const chgExt = (currentExtendedPrice / extendedClose - 1) * 100;

		getColumnStore(securityid, 'chgExt').set({
			chgExt: chgExt,
			formatted: chgExt.toFixed(2) + '%'
		});
	} else if (t.chgExt !== undefined) {
		// Handle direct chgExt values (fallback)
		getColumnStore(securityid, 'chgExt').set({
			chgExt: t.chgExt,
			formatted: t.chgExt.toFixed(2) + '%'
		});
	}
}
