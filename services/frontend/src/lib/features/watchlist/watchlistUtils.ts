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
export function initializeVisibleWatchlists(
	watchlistsArray: Watchlist[],
	currentWatchlistId: number
) {
	if (watchlistsArray && watchlistsArray.length > 0) {
		// Always prioritize flag watchlist if it exists
		const flagWatchlist = watchlistsArray.find((w) => w.watchlistId === flagWatchlistId);
		const otherWatchlists = watchlistsArray.filter((w) => w.watchlistId !== flagWatchlistId);

		let initialVisibleIds: number[] = [];

		if (flagWatchlist) {
			// Add flag watchlist first
			initialVisibleIds.push(flagWatchlist.watchlistId);
			// Add all other watchlists - space limiting will be handled by UI
			initialVisibleIds.push(...otherWatchlists.map((w) => w.watchlistId));
		} else {
			// No flag watchlist, add all watchlists
			initialVisibleIds = watchlistsArray.map((w) => w.watchlistId);
		}

		visibleWatchlistIds.set(initialVisibleIds);
	}
}
// Helper function to update both stores when adding items
function updateWatchlistStores(newItem: WatchlistItem, targetWatchlistId: number) {
	// Always update currentWatchlistItems (what the UI shows)
	currentWatchlistItems.update((v: WatchlistItem[]) => {
		const currentItems = Array.isArray(v) ? v : [];
		// Check if item already exists to avoid duplicates
		if (
			!currentItems.find(
				(item) => item.securityId === newItem.securityId || item.ticker === newItem.ticker
			)
		) {
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
				if (
					!currentItems.find(
						(item) => item.securityId === newItem.securityId || item.ticker === newItem.ticker
					)
				) {
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

			// Also update flagWatchlist if this is the flag watchlist
			if (watchlistId === flagWatchlistId) {
				// Import flagWatchlist here to avoid circular dependency
				import('$lib/utils/stores/stores').then(({ flagWatchlist }) => {
					flagWatchlist.set(v || []);
				});
			}
		})
		.catch((err) => {
			console.error('Error fetching watchlist items:', err);
			currentWatchlistItems.set([]);

			// Also update flagWatchlist if this is the flag watchlist
			if (watchlistId === flagWatchlistId) {
				import('$lib/utils/stores/stores').then(({ flagWatchlist }) => {
					flagWatchlist.set([]);
				});
			}
		});
}

// Centralized new watchlist creation
export function createNewWatchlist(watchlistName: string): Promise<number> {
	if (!watchlistName) {
		throw new Error('Watchlist name is required');
	}

	const existingWatchlist = get(watchlists).find(
		(w: Watchlist) => w.watchlistName.toLowerCase() === watchlistName.toLowerCase()
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

		// Also remove from visible watchlists
		visibleWatchlistIds.update((ids: number[]) =>
			Array.isArray(ids) ? ids.filter((id: number) => id !== watchlistId) : []
		);
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

export function addInstanceToWatchlist(
	currentWatchlistId?: number,
	securityId?: number,
	ticker?: string
) {
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
		})
			.then((watchlistItemId: number) => {
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
					publicRequest<WatchlistItem>('getSecurityDetails', { securityId: securityId })
						.then((securityDetails: WatchlistItem) => {
							const newItem = {
								...securityDetails,
								watchlistItemId: watchlistItemId,
								securityId: securityId
							};

							updateWatchlistStores(newItem, targetWatchlistId);
							console.log(`Added ${securityDetails.ticker || 'security'} to watchlist`);
						})
						.catch((error) => {
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
			})
			.catch((error) => {
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

// New function for multi-add functionality
export function addMultipleInstancesToWatchlist(currentWatchlistId?: number) {
	console.log('addMultipleInstancesToWatchlist', currentWatchlistId);
	if (get(isPublicViewing)) {
		showAuthModal('watchlists', 'signup');
		return;
	}

	if (!currentWatchlistId) {
		console.error('No current watchlist ID available');
		return;
	}

	// Start the multi-add process
	addNextSymbol(currentWatchlistId);
}

// Helper function to add the next symbol in the multi-add process
async function addNextSymbol(targetWatchlistId: number) {
	const inst = { ticker: '' };

	try {
		const i: WatchlistItem = await queryInstanceInput(['ticker'], ['ticker'], inst, 'ticker', 'Add Symbol to Watchlist');

		// Check if the security is already in the current list
		const aList = get(currentWatchlistItems);
		const empty = !Array.isArray(aList);

		if (!empty && aList.find((l: WatchlistItem) => l.securityId === i.securityId)) {
			console.log('Security already in watchlist');
			// Continue with next symbol even if this one was a duplicate
			setTimeout(() => addNextSymbol(targetWatchlistId), 100);
			return;
		}

		// Add the symbol to the watchlist
		const watchlistItemId: number = await privateRequest<number>('newWatchlistItem', {
			watchlistId: targetWatchlistId,
			securityId: i.securityId
		});

		const newItem = { ...i, watchlistItemId: watchlistItemId };
		updateWatchlistStores(newItem, targetWatchlistId);
		console.log(`Added ${i.ticker} to watchlist`);

		// Continue with next symbol automatically
		setTimeout(() => {
			addNextSymbol(targetWatchlistId);
		}, 100);

	} catch (error) {
		// If user cancelled or there was an error, stop the multi-add process
		console.log('Multi-add process ended:', error);
	}
}

// Add new watchlist to front of visible tabs
export function addToVisibleTabs(newWatchlistId: number) {
	if (!newWatchlistId) return;

	visibleWatchlistIds.update((ids) => {
		// If it's already visible, do nothing
		if (ids.includes(newWatchlistId)) return ids;

		// Get current watchlists to check for flag watchlist
		const currentWatchlists = get(watchlists);
		const flagWatchlist = currentWatchlists?.find((w: Watchlist) => w.watchlistId === flagWatchlistId);

		let newIds: number[];

		if (flagWatchlist && !ids.includes(flagWatchlistId)) {
			// Flag watchlist exists but isn't in visible tabs - add it first
			newIds = [flagWatchlistId, newWatchlistId, ...ids];
		} else if (ids.includes(flagWatchlistId)) {
			// Flag watchlist is already visible - add new watchlist after flag
			const flagIndex = ids.indexOf(flagWatchlistId);
			newIds = [
				...ids.slice(0, flagIndex + 1), // Keep flag and everything before it
				newWatchlistId,
				...ids.slice(flagIndex + 1) // Add everything after flag
			];
		} else {
			// No flag watchlist - add to front
			newIds = [newWatchlistId, ...ids];
		}

		// Return all tabs - space limiting will be handled by the UI components
		return newIds;
	});
}
