import { get, writable } from 'svelte/store';
import type { Instance, Watchlist } from '$lib/utils/types/types';
import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
import { queryInstanceInput } from '$lib/components/input/input.svelte';
import { showAuthModal } from '$lib/stores/authModal';
import {
	currentWatchlistItems,
	flagWatchlistId,
	isPublicViewing,
	currentWatchlistId as globalCurrentWatchlistId,
	watchlists
} from '$lib/utils/stores/stores';
import { tick } from 'svelte';
// Extended Instance type to include watchlistItemId
interface WatchlistItem extends Instance {
	watchlistItemId?: number;
}

export const visibleWatchlistIds = writable<number[]>([]);

// Function to initialize visible watchlists
export function initializeVisibleWatchlists(watchlistsArray: Watchlist[], currentWatchlistId: number) {
	if (watchlistsArray && watchlistsArray.length > 0 && currentWatchlistId) {
		const currentIds = get(visibleWatchlistIds);
		if (currentIds.length === 0) {
			visibleWatchlistIds.set(watchlistsArray.slice(0, 3).map(w => w.watchlistId));
		}
	}
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

// Centralized watchlist selection function
export function selectWatchlist(watchlistIdString: string) {
	if (!watchlistIdString) return;

	const watchlistId = parseInt(watchlistIdString);
	if (isNaN(watchlistId)) return;

	// Update the global store so other components know which watchlist is selected
	globalCurrentWatchlistId.set(watchlistId);

	// Fetch items and update the global store
	privateRequest<WatchlistItem[]>('getWatchlistItems', { watchlistId: watchlistId })
		.then((v: WatchlistItem[]) => {
			currentWatchlistItems.set(v || []);
		})
		.catch((err) => {
			console.error('Error fetching watchlist items:', err);
			currentWatchlistItems.set([]);
		});
}

// Centralized new watchlist creation
export function createNewWatchlist(watchlistName: string): Promise<number> {
	if (!watchlistName) {
		throw new Error('Watchlist name is required');
	}

	const existingWatchlist = get(watchlists).find(
		(w) => w.watchlistName.toLowerCase() === watchlistName.toLowerCase()
	);

	if (existingWatchlist) {
		throw new Error('A watchlist with this name already exists');
	}

	return privateRequest<number>('newWatchlist', { watchlistName }).then((newId: number) => {
		watchlists.update((v: Watchlist[]) => {
			const w: Watchlist = {
				watchlistName: watchlistName,
				watchlistId: newId
			};
			if (!Array.isArray(v)) {
				return [w];
			}
			return [w, ...v];
		});
		
		// Automatically select the new watchlist
		selectWatchlist(String(newId));
		// Add the new watchlist to the front of visible tabs
		addToVisibleTabs(newId);
		
		return newId;
	});
}

// Centralized watchlist deletion
export function deleteWatchlist(watchlistId: number): Promise<void> {
	if (watchlistId === flagWatchlistId) {
		throw new Error('The flag watchlist cannot be deleted.');
	}

	return privateRequest<void>('deleteWatchlist', { watchlistId }).then(() => {
		watchlists.update((v: Watchlist[]) => {
			const updatedWatchlists = v.filter(
				(watchlist: Watchlist) => watchlist.watchlistId !== watchlistId
			);

			// If we deleted the current watchlist, select another one
			const currentId = get(globalCurrentWatchlistId);
			if (watchlistId === currentId && updatedWatchlists.length > 0) {
				setTimeout(() => selectWatchlist(String(updatedWatchlists[0].watchlistId)), 10);
			}

			return updatedWatchlists;
		});
	});
}

// Initialize watchlist - select default watchlist on page load
export function initializeDefaultWatchlist() {
	// Try to select flag watchlist first if available
	if (flagWatchlistId !== undefined) {
		selectWatchlist(String(flagWatchlistId));
		return;
	}

	// Subscribe to watchlists store to select initial watchlist when list arrives
	const unsubscribeWatchlists = watchlists.subscribe((list) => {
		const currentWatchlistId = get(globalCurrentWatchlistId);
		if (
			Array.isArray(list) &&
			list.length > 0 &&
			(currentWatchlistId === undefined || isNaN(currentWatchlistId))
		) {
			selectWatchlist(String(list[0].watchlistId));
		}
	});

	// Return cleanup function for the watchlist subscription
	return unsubscribeWatchlists;
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
// Add new watchlist to front of visible tabs
export function addToVisibleTabs(newWatchlistId: number) {
	if (!newWatchlistId) return;
	
	visibleWatchlistIds.update(ids => {
		// If it's already visible, do nothing
		if (ids.includes(newWatchlistId)) return ids;
		
		// Add to front and keep max 3 tabs
		return [newWatchlistId, ...ids].slice(0, 3);
	});
}