import { writable, type Writable } from 'svelte/store';

type ColumnType = 'price' | 'changePct' | 'marketCap' | 'prevClose' | 'chgExt' | 'change';
const stores = new Map<string, Writable<any>>();

// Cache for calculating changes
const priceCache = new Map<number, number>();
const prevCloseCache = new Map<number, number>();

export const getColumnStore = (securityid: number, m: ColumnType) => {
	const k = `${securityid}:${m}`;
	if (!stores.has(k)) stores.set(k, writable({}));
	return stores.get(k)!;
};

const refCounts = new Map<number, number>(); // id → nSub
import { subscribe, unsubscribe } from './socket';

// Helper to fetch initial close data
async function fetchCloseData(securityId: number) {
	try {
		// This would typically make a request to get the previous close
		// For now, we'll subscribe to the close channel
		subscribe(`${securityId}-close-regular`);
	} catch (error) {
		console.warn('Failed to fetch close data for security', securityId, error);
	}
}

export function register(securityid: number) {
	const n = (refCounts.get(securityid) ?? 0) + 1;
	refCounts.set(securityid, n);
	if (n === 1) {
		subscribe(`${securityid}-slow-regular`); // one wire sub only
		// Also fetch close data for change calculations
		fetchCloseData(securityid);
	}
	return () => {
		const m = (refCounts.get(securityid) ?? 1) - 1;
		if (m <= 0) {
			refCounts.delete(securityid);
			unsubscribe(`${securityid}-slow-regular`);
			unsubscribe(`${securityid}-close-regular`);
			// Clean up caches for this security
			priceCache.delete(securityid);
			prevCloseCache.delete(securityid);
			['price', 'change', 'changePct', 'marketCap', 'prevClose', 'chgExt'].forEach((k) =>
				stores.delete(`${securityid}:${k}`)
			);
		} else refCounts.set(securityid, m);
	};
}
let dirty = false;
const latest = new Map<number, any>();
export function enqueueTick(t: any) {
	latest.set(t.securityid, { ...latest.get(t.securityid), ...t });
	if (!dirty) {
		dirty = true;
		requestAnimationFrame(flush);
	}
}

function flush() {
	dirty = false;
	for (const t of latest.values()) {
		const securityid = t.securityid;
		// Update price cache
		if (t.price !== undefined) {
			priceCache.set(securityid, t.price);
			getColumnStore(securityid, 'price').set({
				price: t.price,
				formatted: t.price.toFixed(2)
			});
		}

		// Update prevClose cache
		if (t.prevClose !== undefined) {
			prevCloseCache.set(securityid, t.prevClose);
			getColumnStore(securityid, 'prevClose').set({ prevClose: t.prevClose });
		}

		// Calculate and update change
		const currentPrice = priceCache.get(securityid);
		const prevClose = prevCloseCache.get(securityid);
		if (currentPrice !== undefined && prevClose !== undefined) {
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

		// Handle other direct values
		if (t.marketCap !== undefined) {
			getColumnStore(securityid, 'marketCap').set({ marketCap: t.marketCap });
		}

		if (t.chgExt !== undefined) {
			getColumnStore(securityid, 'chgExt').set({
				chgExt: t.chgExt,
				formatted: t.chgExt.toFixed(2) + '%'
			});
		}
	}
	latest.clear();
}
