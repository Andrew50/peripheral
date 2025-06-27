import { get } from 'svelte/store';
import type { Instance } from '$lib/utils/types/types';
import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
import { queryInstanceInput } from '$lib/components/input/input.svelte';
import {
	flagWatchlistId,
	isPublicViewing,
	currentWatchlistItems
} from '$lib/utils/stores/stores';
import { showAuthModal } from '$lib/stores/authModal';

// Extended Instance type to include watchlistItemId
interface WatchlistItem extends Instance {
	watchlistItemId?: number;
}

// Helper function to update both stores when adding items
function updateWatchlistStores(newItem: WatchlistItem, targetWatchlistId: number) {
	// Always update currentWatchlistItems (what the UI shows)
	currentWatchlistItems.update((v: WatchlistItem[]) => {
		const currentItems = Array.isArray(v) ? v : [];
		// Check if item already exists to avoid duplicates
		if (!currentItems.find(item => item.securityId === newItem.securityId || item.ticker === newItem.ticker)) {
			return [...currentItems, newItem];
		}
		return currentItems;
	});
	
	// Also update flagWatchlist if this is the flag watchlist
	if (targetWatchlistId === flagWatchlistId) {
		// Import flagWatchlist here to avoid circular dependency
		import('$lib/utils/stores/stores').then(({ flagWatchlist }) => {
			flagWatchlist.update((v: WatchlistItem[]) => {
				const currentItems = Array.isArray(v) ? v : [];
				// Check if item already exists to avoid duplicates
				if (!currentItems.find(item => item.securityId === newItem.securityId || item.ticker === newItem.ticker)) {
					return [...currentItems, newItem];
				}
				return currentItems;
			});
		});
	}
}

export function addInstanceToWatchlist(currentWatchlistId?: number, securityId?: number, ticker?: string) {
    console.log('addInstanceToWatchlist', securityId, currentWatchlistId);
	if (get(isPublicViewing)) {
		showAuthModal('watchlists', 'signup');
		return;
	}

	// If securityId is provided, skip the query input and directly add to watchlist
	if (securityId && currentWatchlistId) {
		const targetWatchlistId = currentWatchlistId;
		
		// Check if the security is already in the current list
		const aList = get(currentWatchlistItems);
		const empty = !Array.isArray(aList);
		
		if (!empty && aList.find((l: WatchlistItem) => l.securityId === securityId)) {
			console.log('Security already in watchlist');
			return;
		}

		privateRequest<number>('newWatchlistItem', {
			watchlistId: targetWatchlistId,
			securityId: securityId
		}).then((watchlistItemId: number) => {
			// If we have ticker information, use it directly to avoid unnecessary API call
			if (ticker) {
				const newItem = { 
					securityId: securityId,
					watchlistItemId: watchlistItemId,
					ticker: ticker
				} as WatchlistItem;
				
				updateWatchlistStores(newItem, targetWatchlistId);
				console.log(`Added ${ticker} to watchlist`);
			} else {
				// Only make API call if we don't have ticker information
				publicRequest<WatchlistItem>('getSecurityDetails', { securityId: securityId }).then((securityDetails: WatchlistItem) => {
					const newItem = { 
						...securityDetails, 
						watchlistItemId: watchlistItemId,
						securityId: securityId
					};
					
					updateWatchlistStores(newItem, targetWatchlistId);
					console.log(`Added ${securityDetails.ticker || 'security'} to watchlist`);
				}).catch((error) => {
					console.error('Error fetching security details:', error);
					// Final fallback: add with minimal info
					const newItem = { 
						securityId: securityId,
						watchlistItemId: watchlistItemId,
						ticker: `Security-${securityId}` // Fallback ticker
					} as WatchlistItem;
					
					updateWatchlistStores(newItem, targetWatchlistId);
				});
			}
		}).catch((error) => {
			console.error('Error adding to watchlist:', error);
		});
		return;
	}

	// Original behavior when no securityId is provided
	const inst = { ticker: '' };
	queryInstanceInput(['ticker'], ['ticker'], inst, 'ticker', 'Add Symbol to Watchlist').then(
		(i: WatchlistItem) => {
			if (!currentWatchlistId) {
				console.error('No current watchlist ID available');
				return;
			}
			
			const targetWatchlistId = currentWatchlistId;
			const aList = get(currentWatchlistItems);
			const empty = !Array.isArray(aList);

			if (!empty && aList.find((l: WatchlistItem) => l.securityId === i.securityId)) {
				console.log('Security already in watchlist');
				return;
			}

			privateRequest<number>('newWatchlistItem', {
				watchlistId: targetWatchlistId,
				securityId: i.securityId
			}).then((watchlistItemId: number) => {
				const newItem = { ...i, watchlistItemId: watchlistItemId };
				updateWatchlistStores(newItem, targetWatchlistId);
			});
		}
	);
} 