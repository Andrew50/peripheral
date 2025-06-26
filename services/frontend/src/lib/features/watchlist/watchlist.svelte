<script lang="ts">
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';
	import type { Instance, Watchlist } from '$lib/utils/types/types';
	import { onMount, tick } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import {
		flagWatchlistId,
		watchlists,
		flagWatchlist,
		isPublicViewing,
		currentWatchlistId as globalCurrentWatchlistId,
		currentWatchlistItems
	} from '$lib/utils/stores/stores';
	import '$lib/styles/global.css';
	import WatchlistList from './watchlistList.svelte';
	import { showAuthModal } from '$lib/stores/authModal';
	// Extended Instance type to include watchlistItemId
	interface WatchlistItem extends Instance {
		watchlistItemId?: number;
	}


	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let container: HTMLDivElement;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;

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
			flagWatchlist.update((v: WatchlistItem[]) => {
				const currentItems = Array.isArray(v) ? v : [];
				// Check if item already exists to avoid duplicates
				if (!currentItems.find(item => item.securityId === newItem.securityId || item.ticker === newItem.ticker)) {
					return [...currentItems, newItem];
				}
				return currentItems;
			});
		}
	}

	function closeNewWatchlistWindow() {
		showWatchlistInput = false;
		newWatchlistName = '';

		// Make sure we have a valid previousWatchlistId to fall back to
		if (previousWatchlistId === undefined || isNaN(previousWatchlistId)) {
			// If no valid previous ID, try to use the first available watchlist
			const watchlistsValue = get(watchlists);
			if (Array.isArray(watchlistsValue) && watchlistsValue.length > 0) {
				previousWatchlistId = watchlistsValue[0].watchlistId;
			}
		}

		// Set the current ID back to the previous one
		currentWatchlistId = previousWatchlistId;

		// Force a UI refresh by selecting the watchlist
		tick().then(() => {
			// Force the select dropdown to reset by accessing the DOM element
			const selectElement = document.getElementById('watchlists') as HTMLSelectElement;
			if (selectElement) {
				selectElement.value = String(previousWatchlistId);
			}

			selectWatchlist(String(previousWatchlistId));
		});
	}

	onMount(() => {
		// Check if the flag watchlist exists in the loaded watchlists
		const checkAndCreateFlagWatchlist = () => {
			const watchlistsValue = get(watchlists);

			// Don't proceed if watchlists isn't properly loaded yet
			if (!Array.isArray(watchlistsValue)) return;

			const flagWatch = watchlistsValue.find((w) => w.watchlistName.toLowerCase() === 'flag');

			if (!flagWatch) {
				// Flag watchlist doesn't exist, create it
				privateRequest<number>('newWatchlist', { watchlistName: 'flag' }).then((newId: number) => {
					// Update the watchlists store with the new flag watchlist
					watchlists.update((v: Watchlist[]) => {
						const w: Watchlist = {
							watchlistName: 'flag',
							watchlistId: newId
						};
						if (!Array.isArray(v)) {
							return [w];
						}
						return [w, ...v];
					});

					// Initialize the flagWatchlist store via the backend
					privateRequest<WatchlistItem[]>('getWatchlistItems', { watchlistId: newId }).then(
						(items: WatchlistItem[]) => {
							// The store will be updated by the backend via the global store
						}
					);
				});
			}
		};

		// Wait a short delay to ensure stores are initialized
		setTimeout(checkAndCreateFlagWatchlist, 100);

		// Convert flagWatchlistId to string to fix type issue
		if (flagWatchlistId !== undefined) {
			selectWatchlist(String(flagWatchlistId));
		}

		// Subscribe to watchlists store to select initial watchlist when list arrives
		const unsubscribeWatchlists = watchlists.subscribe((list) => {
			if (
				Array.isArray(list) &&
				list.length > 0 &&
				(currentWatchlistId === undefined || isNaN(currentWatchlistId))
			) {
				selectWatchlist(String(list[0].watchlistId));
			}
		});
		// Cleanup the subscription when component unmounts
		return () => {
			unsubscribeWatchlists();
		};
	});

	export function addInstanceToWatchlist(securityId?: number) {
		if (get(isPublicViewing)) {
			showAuthModal('watchlists', 'signup');
			return;
		}

		// If securityId is provided, skip the query input and directly add to watchlist
		if (securityId) {
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
				// We need to get the security details to display properly
				privateRequest<WatchlistItem>('getSecurityDetails', { securityId: securityId }).then((securityDetails: WatchlistItem) => {
					const newItem = { 
						...securityDetails, 
						watchlistItemId: watchlistItemId,
						securityId: securityId
					};
					
					updateWatchlistStores(newItem, targetWatchlistId);
					console.log(`Added ${securityDetails.ticker || 'security'} to watchlist`);
				}).catch((error) => {
					console.error('Error fetching security details:', error);
					// Fallback: add with minimal info
					const newItem = { 
						securityId: securityId,
						watchlistItemId: watchlistItemId,
						ticker: `Security-${securityId}` // Fallback ticker
					} as WatchlistItem;
					
					updateWatchlistStores(newItem, targetWatchlistId);
				});
			}).catch((error) => {
				console.error('Error adding to watchlist:', error);
			});
			return;
		}

		// Original behavior when no securityId is provided
		const inst = { ticker: '' };
		queryInstanceInput(['ticker'], ['ticker'], inst, 'ticker', 'Add Symbol to Watchlist').then(
			(i: WatchlistItem) => {
				const targetWatchlistId = currentWatchlistId;
				const aList = get(currentWatchlistItems);
				const empty = !Array.isArray(aList);
				
				if (empty || !aList.find((l: WatchlistItem) => l.ticker === i.ticker)) {
					privateRequest<number>('newWatchlistItem', {
						watchlistId: targetWatchlistId,
						securityId: i.securityId
					}).then((watchlistItemId: number) => {
						i.watchlistItemId = watchlistItemId;
						updateWatchlistStores(i, targetWatchlistId);
					});
				}
				setTimeout(() => {
					addInstanceToWatchlist();
				}, 1);
			}
		);
	}

	function newWatchlist() {
		if (newWatchlistName === '') return;

		// Check for duplicate names
		const existingWatchlist = get(watchlists).find(
			(w) => w.watchlistName.toLowerCase() === newWatchlistName.toLowerCase()
		);

		if (existingWatchlist) {
			alert('A watchlist with this name already exists');
			return;
		}

		privateRequest<number>('newWatchlist', { watchlistName: newWatchlistName }).then(
			(newId: number) => {
				watchlists.update((v: Watchlist[]) => {
					const w: Watchlist = {
						watchlistName: newWatchlistName,
						watchlistId: newId
					};
					selectWatchlist(String(newId));
					newWatchlistName = '';
					showWatchlistInput = false;
					if (!Array.isArray(v)) {
						return [w];
					}
					return [w, ...v];
				});
			}
		);
	}

	function deleteItem(item: WatchlistItem) {
		if (!item.watchlistItemId) {
			throw new Error('missing id on delete');
		}
		privateRequest<void>('deleteWatchlistItem', { watchlistItemId: item.watchlistItemId }).then(
			() => {
				// Update currentWatchlistItems (what the UI shows)
				currentWatchlistItems.update((items) => {
					return items.filter((i) => i.watchlistItemId !== item.watchlistItemId);
				});
				
				// Also update flagWatchlist if this is the flag watchlist
				if (currentWatchlistId === flagWatchlistId) {
					flagWatchlist.update((items) => {
						return items.filter((i) => i.watchlistItemId !== item.watchlistItemId);
					});
				}
			}
		);
	}

	function selectWatchlist(watchlistIdString: string) {
		if (!watchlistIdString) return;

		if (watchlistIdString === 'new') {
			// Store the current watchlist ID before showing the form
			if (currentWatchlistId !== undefined && !isNaN(currentWatchlistId)) {
				previousWatchlistId = currentWatchlistId;
			} else {
				// If no valid current ID, try to find a valid one from the watchlists
				const watchlistsValue = get(watchlists);
				if (Array.isArray(watchlistsValue) && watchlistsValue.length > 0) {
					previousWatchlistId = watchlistsValue[0].watchlistId;
				}
			}

			// Show the form
			showWatchlistInput = true;
			tick().then(() => {
				if (newNameInput) {
					newNameInput.focus();
				}
			});
			return;
		}

		if (watchlistIdString === 'delete') {
			// Get watchlist name before showing confirmation
			const watchlistsValue = get(watchlists);

			// Make sure we have the current watchlist ID as a number for comparison
			const currentWatchlistIdNum = Number(currentWatchlistId);

			// Don't allow deletion of the flag watchlist
			if (currentWatchlistIdNum === flagWatchlistId) {
				alert('The flag watchlist cannot be deleted.');
				// Reset the dropdown
				selectWatchlist(String(currentWatchlistId));
				return;
			}

			// Debug the watchlists and current ID to verify what we're looking for

			// Find the watchlist by ID, ensuring type consistency
			const watchlist = Array.isArray(watchlistsValue)
				? watchlistsValue.find((w) => Number(w.watchlistId) === currentWatchlistIdNum)
				: null;

			// Use the actual name or fall back to the ID if name is not available
			const watchlistName = watchlist?.watchlistName || `Watchlist #${currentWatchlistIdNum}`;

			if (confirm(`Are you sure you want to delete "${watchlistName}"?`)) {
				// Ensure we're passing a number to deleteWatchlist
				deleteWatchlist(Number(currentWatchlistId));
			} else {
				// User canceled, reset the dropdown
				selectWatchlist(String(currentWatchlistId));
			}
			return;
		}

		showWatchlistInput = false;
		newWatchlistName = '';
		const watchlistId = parseInt(watchlistIdString);
		currentWatchlistId = watchlistId;
		
		// Update the global store so other components know which watchlist is selected
		globalCurrentWatchlistId.set(watchlistId);

		// Set the current watchlist ID
		currentWatchlistId = watchlistId;

		// Fetch items and update the global store
		privateRequest<WatchlistItem[]>('getWatchlistItems', { watchlistId: watchlistId })
			.then((v: WatchlistItem[]) => {
				currentWatchlistItems.set(v || []);
				// Also update flagWatchlist if this is the flag watchlist
				if (watchlistId === flagWatchlistId) {
					flagWatchlist.set(v || []);
				}
			})
			.catch((err) => {
				currentWatchlistItems.set([]);
				if (watchlistId === flagWatchlistId) {
					flagWatchlist.set([]);
				}
			});
	}

	function deleteWatchlist(id: number) {
		// Ensure id is a number before sending to the backend
		const watchlistId = typeof id === 'string' ? parseInt(id, 10) : id;

		// Safety check to prevent deleting the flag watchlist
		if (watchlistId === flagWatchlistId) {
			alert('The flag watchlist cannot be deleted.');
			return;
		}

		privateRequest<void>('deleteWatchlist', { watchlistId }).then(() => {
			watchlists.update((v: Watchlist[]) => {
				// After deletion, select another watchlist if available
				const updatedWatchlists = v.filter((watchlist: Watchlist) => watchlist.watchlistId !== id);

				// If we deleted the current watchlist, select another one
				if (id === currentWatchlistId && updatedWatchlists.length > 0) {
					// Select the first available watchlist
					setTimeout(() => selectWatchlist(String(updatedWatchlists[0].watchlistId)), 10);
				}

				return updatedWatchlists;
			});
		});
	}

	// Helper function to get first letter of watchlist name
	function getWatchlistInitial(name: string): string {
		return name.charAt(0).toUpperCase();
	}

	function handleWatchlistChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		const value = target.value;

		// Always handle new watchlist selection, even if it's already selected
		if (value === 'new') {
			// We're trying to open the new watchlist form, save the current selection
			if (!showWatchlistInput) {
				previousWatchlistId = currentWatchlistId;
				selectWatchlist('new');
			}
			return;
		}

		// For delete and other watchlist IDs
		if (value === 'delete') {
			selectWatchlist('delete');
			return;
		}

		// For regular watchlist selections
		previousWatchlistId = parseInt(value, 10);
		currentWatchlistId = parseInt(value, 10);
		globalCurrentWatchlistId.set(parseInt(value, 10));
		selectWatchlist(value);
	}
</script>

<div tabindex="-1" class="feature-container" bind:this={container}>
	<!-- Controls container first -->
	<div class="controls-container">
		{#if Array.isArray($watchlists)}
			<div class="watchlist-selector">
				<div class="select-wrapper">
					<select
						class="default-select"
						id="watchlists"
						value={currentWatchlistId?.toString()}
						on:change={handleWatchlistChange}
					>
					<optgroup label="My Watchlists">
						{#each $watchlists as watchlist}
							<option value={watchlist.watchlistId.toString()}>
								{watchlist.watchlistName === 'flag' ? 'Flag (Protected)' : watchlist.watchlistName}
							</option>
						{/each}
					</optgroup>
					<optgroup label="Actions">
						<option value="new">+ Create New Watchlist</option>
						{#if currentWatchlistId && currentWatchlistId !== flagWatchlistId}
							<option value="delete">- Delete Current Watchlist</option>
						{/if}
					</optgroup>
					</select>
					<div class="caret-icon">
						<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<polyline points="6,9 12,15 18,9"></polyline>
						</svg>
					</div>
				</div>
			</div>

			{#if showWatchlistInput}
				<div class="new-watchlist-container">
					<input
						class="input"
						bind:this={newNameInput}
						on:keydown={(event) => {
							if (event.key === 'Enter') {
								newWatchlist();
							} else if (event.key === 'Escape') {
								closeNewWatchlistWindow();
							}
						}}
						bind:value={newWatchlistName}
						placeholder="New Watchlist Name"
					/>
					<div class="new-watchlist-buttons">
						<button class="utility-button" on:click={newWatchlist}>✓</button>
						<button class="utility-button" on:click={closeNewWatchlistWindow}>✕</button>
					</div>
				</div>
			{/if}
		{/if}
	</div>

	<!-- Shortcut buttons -->
	<div class="shortcut-container">
		<div class="watchlist-shortcuts">
			{#if Array.isArray($watchlists)}
				{#each $watchlists as watchlist}
					<button
						class="shortcut-button {currentWatchlistId === watchlist.watchlistId ? 'active' : ''}"
						on:click={() => selectWatchlist(String(watchlist.watchlistId))}
						title={watchlist.watchlistName}
					>
						{#if watchlist.watchlistName.toLowerCase() === 'flag'}
							<span class="flag-shortcut-icon">
								<svg
									xmlns="http://www.w3.org/2000/svg"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
								>
									<path d="M5 5v14"></path>
									<path d="M19 5l-6 4 6 4-6 4"></path>
								</svg>
							</span>
						{:else}
							{getWatchlistInitial(watchlist.watchlistName)}
						{/if}
					</button>
				{/each}
			{/if}
		</div>

		<!-- Add button on the same line -->
		{#if !showWatchlistInput}
			<button class="add-item-button shortcut-button" title="Add Symbol" on:click={() => addInstanceToWatchlist()}
				>+</button
			>
		{/if}
	</div>

	<!-- Wrap List component for scrolling -->
	<div class="list-scroll-container">
		<WatchlistList
			parentDelete={deleteItem}
			columns={['Ticker', 'Price', 'Chg', 'Chg%', 'Ext']}
			list={currentWatchlistItems}
		/>
	</div>
</div>

<style>
	.watchlist-selector {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 6px 12px 6px 4px; /* Reduced left padding to align with container */
	}

	.select-wrapper {
		position: relative;
		display: inline-flex;
		align-items: center;
		width: fit-content;
		background: transparent;
		border-radius: clamp(6px, 1vw, 8px);
		padding: clamp(6px, 1vw, 8px) 0 clamp(6px, 1vw, 8px) clamp(6px, 1vw, 8px);
	}

	.watchlist-selector select {
		flex: 0 1 auto;
		min-width: fit-content;
		width: fit-content;
		background: transparent;
		color: #ffffff;
		border: none;
		border-radius: 0;
		padding: 0;
		font-size: clamp(0.7rem, 0.5rem + 0.5vw, 0.875rem);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		appearance: none;
		-webkit-appearance: none;
		-moz-appearance: none;
	}

	.caret-icon {
		margin-left: clamp(6px, 1vw, 8px);
		margin-right: clamp(6px, 1vw, 8px);
		width: clamp(10px, 1.5vw, 14px);
		height: clamp(10px, 1.5vw, 14px);
		color: #ffffff;
		pointer-events: none;
		flex-shrink: 0;
	}

	.caret-icon svg {
		width: 100%;
		height: 100%;
	}

	.select-wrapper:hover {
		background: rgba(255, 255, 255, 0.15);
	}

	.watchlist-selector select:focus,
	.watchlist-selector select:focus-visible {
		outline: none;
	}

	.select-wrapper:has(select:focus) {
		background: rgba(255, 255, 255, 0.15);
	}

	.new-watchlist-container {
		margin-top: 12px;
		padding: 16px;
		animation: slideDown 0.2s ease-out;
		background: rgba(0, 0, 0, 0.3);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 8px;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px) scale(0.95);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	.new-watchlist-container .input {
		width: 100%;
		margin-bottom: 12px;
		padding: 12px 16px;
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.2);
		background: rgba(0, 0, 0, 0.3);
		color: #ffffff;
		font-size: 14px;
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
	}

	.new-watchlist-container .input:focus {
		border-color: rgba(255, 255, 255, 0.6);
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.1);
		outline: none;
	}

	.new-watchlist-container .input::placeholder {
		color: rgba(255, 255, 255, 0.6);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.new-watchlist-buttons {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.new-watchlist-buttons .utility-button {
		padding: 8px 16px;
		color: #ffffff;
		font-size: 14px;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
		min-width: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
	}

	.new-watchlist-buttons .utility-button:hover {
		background: rgba(255, 255, 255, 0.1);
		border-color: rgba(255, 255, 255, 0.4);
		transform: translateY(-1px);
	}

	.shortcut-container {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 12px;
		width: 100%;
	}

	.watchlist-shortcuts {
		display: flex;
		gap: 6px;
		flex-wrap: wrap;
	}

	.shortcut-button {
		padding: 0;
		color: #ffffff;
		font-size: 0.65rem;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
		width: 24px;
		height: 24px;
		min-width: 24px;
		min-height: 24px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 50%;
	}

	.shortcut-button:hover {
		background: rgba(255, 255, 255, 0.9);
		border-color: rgba(255, 255, 255, 0.4);
		color: #000;
	}

	.shortcut-button.active {
		background: rgba(255, 255, 255, 0.2);
		color: #ffffff;
		border-color: rgba(255, 255, 255, 0.6);
	}

	.add-item-button {
		color: #ffffff;
		width: clamp(28px, 4vw, 32px);
		height: clamp(28px, 4vw, 32px);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: clamp(1rem, 0.7rem + 0.6vw, 1.2rem);
		font-weight: 300;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		background: transparent;
		border: none;
		border-radius: 6px;
		transition: none;
	}

	.add-item-button:hover {
		background: rgba(255, 255, 255, 0.2);
		color: #ffffff;
	}

	.feature-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		height: 100%;
		background: transparent;
		border-radius: 0;
		overflow: visible;
		padding: clamp(0.25rem, 0.5vw, 0.5rem) clamp(0.5rem, 1vw, 1rem);
	}

	.controls-container {
		display: flex;
		flex-direction: column;
		gap: 12px;
		background: transparent;
	}

	:global(.default-select) {
		padding: 8px 12px;
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.2);
		background: rgba(0, 0, 0, 0.3);
		color: #ffffff;
		font-size: 14px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
	}

	:global(.default-select:hover) {
		border-color: rgba(255, 255, 255, 0.4);
	}

	:global(.default-select:focus) {
		border-color: rgba(255, 255, 255, 0.6);
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.1);
		outline: none;
	}

	:global(.default-select option) {
		background: rgba(0, 0, 0, 0.9);
		color: #ffffff;
		padding: 8px;
	}

	:global(.default-select optgroup) {
		font-weight: 600;
		color: rgba(255, 255, 255, 0.7);
		background: rgba(0, 0, 0, 0.9);
	}

	/* New style for the list container */
	.list-scroll-container {
		flex-grow: 1; /* Take remaining vertical space */
		overflow-y: auto; /* Allow vertical scrolling */
		min-height: 0; /* Necessary for flex-grow in some cases */
		padding: 4px;
		background: rgba(0, 0, 0, 0.3);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 8px;
	}

	/* Custom scrollbar for WebKit browsers */
	.list-scroll-container::-webkit-scrollbar {
		width: 6px; /* Width of the scrollbar */
	}

	.list-scroll-container::-webkit-scrollbar-track {
		background: transparent; /* Transparent background */
		border-radius: 3px;
	}

	.list-scroll-container::-webkit-scrollbar-thumb {
		background-color: rgba(255, 255, 255, 0.2); /* Semi-transparent white */
		border-radius: 3px;
		border: 1px solid transparent; /* Creates padding around thumb */
		background-clip: content-box;
	}

	.list-scroll-container::-webkit-scrollbar-thumb:hover {
		background-color: rgba(255, 255, 255, 0.4); /* Slightly more opaque on hover */
	}

	/* Shortcut flag icon styling */
	.flag-shortcut-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}
	.flag-shortcut-icon svg {
		width: 10px;
		height: 10px;
		color: #4a80f0;
		filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.8));
	}
</style>
