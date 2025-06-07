<script lang="ts">
	import List from '$lib/components/list.svelte';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';
	import type { Instance, Watchlist } from '$lib/utils/types/types';
	import { onMount, tick } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { flagWatchlistId, watchlists, flagWatchlist } from '$lib/utils/stores/stores';
	import '$lib/styles/global.css';

	// Extended Instance type to include watchlistItemId
	interface WatchlistItem extends Instance {
		watchlistItemId?: number;
	}

	let activeList: Writable<WatchlistItem[]> = writable([]);
	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let container: HTMLDivElement;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;

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
			if (Array.isArray(list) && list.length > 0 && (currentWatchlistId === undefined || isNaN(currentWatchlistId))) {
				selectWatchlist(String(list[0].watchlistId));
			}
		});
		// Cleanup the subscription when component unmounts
		return () => {
			unsubscribeWatchlists();
		};
	});

	function addInstance() {
		const inst = { ticker: '', timestamp: 0 };
		queryInstanceInput(['ticker'], ['ticker', 'timestamp'], inst).then((i: WatchlistItem) => {
			const aList = get(activeList);
			const empty = !Array.isArray(aList);
			if (empty || !aList.find((l: WatchlistItem) => l.ticker === i.ticker)) {
				privateRequest<number>('newWatchlistItem', {
					watchlistId: currentWatchlistId,
					securityId: i.securityId
				}).then((watchlistItemId: number) => {
					activeList.update((v: WatchlistItem[]) => {
						i.watchlistItemId = watchlistItemId;
						if (empty) {
							return [i];
						} else {
							return [...v, i];
						}
					});
				});
			}
			setTimeout(() => {
				addInstance();
			}, 1);
		});
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
				activeList.update((items) => {
					return items.filter((i) => i.watchlistItemId !== item.watchlistItemId);
				});
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

		// Decide whether to use the global flagWatchlist store or a local one
		if (watchlistId === flagWatchlistId) {
			activeList = flagWatchlist; // Point to the global store

			// Fetch items and update the GLOBAL flagWatchlist store
			privateRequest<WatchlistItem[]>('getWatchlistItems', { watchlistId: watchlistId }).then(
				(v: WatchlistItem[]) => {
					flagWatchlist.set(v || []); // Update the global store
				}
			).catch(err => {
				flagWatchlist.set([]); // Set global store empty on error
			});
		} else {
			// For regular watchlists, create a new local writable store
			activeList = writable<WatchlistItem[]>([]); 
			currentWatchlistId = watchlistId;

			// Fetch items and update the LOCAL activeList store
			privateRequest<WatchlistItem[]>('getWatchlistItems', { watchlistId: watchlistId }).then(
				(v: WatchlistItem[]) => {
					activeList.set(v || []); // Update the local store
				}
			).catch(err => {
				activeList.set([]); // Set local store empty on error
			});
		}
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
		selectWatchlist(value);
	}

</script>

<div tabindex="-1" class="feature-container" bind:this={container}>
	<!-- Controls container first -->
	<div class="controls-container">
		{#if Array.isArray($watchlists)}
			<div class="watchlist-selector">
				<select
					class="default-select"
					id="watchlists"
					value={currentWatchlistId?.toString()}
					on:change={handleWatchlistChange}
				>
					<optgroup label="My Watchlists">
						{#each $watchlists as watchlist}
							<option value={watchlist.watchlistId.toString()}>
								{watchlist.watchlistName === 'flag'
									? 'Flag (Protected)'
									: watchlist.watchlistName}
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
				{#if !showWatchlistInput}
					<button class="utility-button" title="Add Symbol" on:click={addInstance}>+</button>
					<button
						class="utility-button new-watchlist-button"
						title="New Watchlist"
						on:click={() => selectWatchlist('new')}
					>
						<span>+</span>
						<span class="list-icon">📋</span>
					</button>
				{/if}
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

	<!-- Shortcut buttons between controls and list -->
	<div class="shortcut-container">
		{#if Array.isArray($watchlists)}
			{#each $watchlists as watchlist}
				<button
					class="shortcut-button {currentWatchlistId === watchlist.watchlistId ? 'active' : ''}"
					on:click={() => selectWatchlist(String(watchlist.watchlistId))}
					title={watchlist.watchlistName}
				>
					{#if watchlist.watchlistName.toLowerCase() === 'flag'}
						<span class="flag-shortcut-icon">
							<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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

	<!-- Wrap List component for scrolling -->
	<div class="list-scroll-container">
		<List
			parentDelete={deleteItem}
			columns={['Ticker', 'Price', 'Chg', 'Chg%', 'Ext']}
			list={activeList}
		/>
	</div>
</div>

<style>
	.watchlist-selector {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 4px;
		background: var(--ui-bg-secondary);
		border-radius: 6px;
		border: 1px solid var(--ui-border);
	}

	.watchlist-selector select {
		flex: 1;
		min-width: 200px;
	}

	.watchlist-selector .utility-button {
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 18px;
		transition: all 0.2s ease;
	}

	.watchlist-selector .utility-button:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.watchlist-selector .new-watchlist-button {
		font-size: 14px;
		position: relative;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.watchlist-selector .new-watchlist-button .list-icon {
		font-size: 12px;
		position: absolute;
		right: 4px;
		bottom: 2px;
	}

	.new-watchlist-container {
		margin-top: 12px;
		padding: 16px;
		background: var(--ui-bg-secondary);
		border-radius: 8px;
		border: 1px solid var(--ui-border);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
		animation: slideDown 0.2s ease-out;
	}

	@keyframes slideDown {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.new-watchlist-container .input {
		width: 100%;
		margin-bottom: 12px;
		padding: 10px 12px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		transition: all 0.2s ease;
	}

	.new-watchlist-container .input:focus {
		border-color: var(--accent-color);
		box-shadow: 0 0 0 2px rgba(var(--accent-color-rgb), 0.1);
	}

	.new-watchlist-buttons {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.new-watchlist-buttons .utility-button {
		padding: 8px 16px;
		border-radius: 4px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		transition: all 0.2s ease;
		min-width: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.new-watchlist-buttons .utility-button:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.shortcut-container {
		display: flex;
		gap: 8px;
		padding: 12px 16px;
		flex-wrap: wrap;
		border-bottom: 1px solid var(--ui-border);
		background: var(--ui-bg-secondary);
	}

	.shortcut-button {
		padding: 8px 12px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		font-weight: 500;
		transition: all 0.2s ease;
		min-width: 36px;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.shortcut-button:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.shortcut-button.active {
		background: var(--accent-color);
		color: var(--text-on-accent);
		border-color: var(--accent-color);
	}

	.feature-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		height: 100%;
		background: var(--ui-bg-primary);
		border-radius: 8px;
		overflow: visible;
	}

	.controls-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 16px;
		background: var(--ui-bg-primary);
		border-bottom: 1px solid var(--ui-border);
	}

	:global(.default-select) {
		padding: 8px 12px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		font-size: 14px;
		transition: all 0.2s ease;
	}

	:global(.default-select:hover) {
		border-color: var(--accent-color);
	}

	:global(.default-select:focus) {
		border-color: var(--accent-color);
		box-shadow: 0 0 0 2px rgba(var(--accent-color-rgb), 0.1);
	}

	:global(.default-select option) {
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		padding: 8px;
	}

	:global(.default-select optgroup) {
		font-weight: 600;
		color: var(--text-secondary);
	}

	/* New style for the list container */
	.list-scroll-container {
		flex-grow: 1; /* Take remaining vertical space */
		overflow-y: auto; /* Allow vertical scrolling */
		min-height: 0; /* Necessary for flex-grow in some cases */
	}

	/* Custom scrollbar for WebKit browsers */
	.list-scroll-container::-webkit-scrollbar {
		width: 8px; /* Width of the scrollbar */
	}

	.list-scroll-container::-webkit-scrollbar-track {
		background: var(--ui-bg-primary); /* Match the primary background */
		border-radius: 4px;
	}

	.list-scroll-container::-webkit-scrollbar-thumb {
		background-color: var(--ui-border); /* Use border color for the thumb */
		border-radius: 4px;
		border: 2px solid var(--ui-bg-primary); /* Creates padding around thumb */
	}

	.list-scroll-container::-webkit-scrollbar-thumb:hover {
		background-color: var(--text-secondary); /* Slightly lighter on hover */
	}

	/* Shortcut flag icon styling */
	.flag-shortcut-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}
	.flag-shortcut-icon svg {
		width: 14px;
		height: 14px;
		color: var(--accent-color);
	}
</style>
