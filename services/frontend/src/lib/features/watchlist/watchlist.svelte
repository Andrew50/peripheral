<script lang="ts">
	import type { Instance, Watchlist } from '$lib/utils/types/types';
	import { onMount, tick } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import {
		flagWatchlistId,
		watchlists,
		isPublicViewing,
		currentWatchlistId as globalCurrentWatchlistId,
		currentWatchlistItems
	} from '$lib/utils/stores/stores';
	import '$lib/styles/global.css';
	import WatchlistList from './watchlistList.svelte';
	import { showAuthModal } from '$lib/stores/authModal';
	import {
		addInstanceToWatchlist as addToWatchlist,
		selectWatchlist,
		createNewWatchlist,
		deleteWatchlist,
		addToVisibleTabs,
		visibleWatchlistIds,
		initializeVisibleWatchlists
	} from './watchlistUtils';
	// Extended Instance type to include watchlistItemId
	interface WatchlistItem extends Instance {
		watchlistItemId?: number;
	}

	let container: HTMLDivElement;

	function deleteItem(item: WatchlistItem) {
		if (!item.watchlistItemId) {
			throw new Error('missing id on delete');
		}
		privateRequest<void>('deleteWatchlistItem', { watchlistItemId: item.watchlistItemId }).then(
			() => {
				// Update currentWatchlistItems (what the UI shows)
				currentWatchlistItems.update((items: WatchlistItem[]) => {
					return items.filter((i: WatchlistItem) => i.watchlistItemId !== item.watchlistItemId);
				});
			}
		);
	}

	// Watchlist tab functionality
	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;
	let showDropdown = false;

	// Track visible watchlists for tabs in fixed order

	// Get all visible watchlists in their fixed positions
	$: visibleWatchlists = $visibleWatchlistIds
		.slice(0, 3) // Show max 3 tabs
		.map((id) => $watchlists?.find((w) => w.watchlistId === id))
		.filter((watchlist): watchlist is Watchlist => Boolean(watchlist));

	function newWatchlist() {
		if (newWatchlistName === '') return;

		createNewWatchlist(newWatchlistName)
			.then((newWatchlistId: number) => {
				newWatchlistName = '';
				showWatchlistInput = false;
			})
			.catch((error) => {
				alert(error.message);
			});
	}

	function closeNewWatchlistWindow() {
		showWatchlistInput = false;
		newWatchlistName = '';

		if (previousWatchlistId === undefined || isNaN(previousWatchlistId)) {
			if (Array.isArray($watchlists) && $watchlists.length > 0) {
				previousWatchlistId = $watchlists[0].watchlistId;
			}
		}

		// Use switchToWatchlist for consistency
		tick().then(() => {
			switchToWatchlist(previousWatchlistId);
		});
	}

	function handleWatchlistSelection(watchlistIdString: string) {
		if (!watchlistIdString) return;

		if (watchlistIdString === 'new') {
			if (currentWatchlistId !== undefined && !isNaN(currentWatchlistId)) {
				previousWatchlistId = currentWatchlistId;
			} else {
				if (Array.isArray($watchlists) && $watchlists.length > 0) {
					previousWatchlistId = $watchlists[0].watchlistId;
				}
			}

			showWatchlistInput = true;
			tick().then(() => {
				const inputElement = document.getElementById('new-watchlist-input') as HTMLInputElement;
				if (inputElement) {
					inputElement.focus();
				}
			});
			return;
		}

		if (watchlistIdString === 'delete') {
			const currentWatchlistIdNum = Number(currentWatchlistId);

			if (currentWatchlistIdNum === flagWatchlistId) {
				alert('The flag watchlist cannot be deleted.');
				handleWatchlistSelection(String(currentWatchlistId));
				return;
			}

			const watchlist = Array.isArray($watchlists)
				? $watchlists.find((w) => Number(w.watchlistId) === currentWatchlistIdNum)
				: null;

			const watchlistName = watchlist?.watchlistName || `Watchlist #${currentWatchlistIdNum}`;

			if (confirm(`Are you sure you want to delete "${watchlistName}"?`)) {
				deleteWatchlist(Number(currentWatchlistId)).catch((error) => {
					alert(error.message);
				});
			} else {
				handleWatchlistSelection(String(currentWatchlistId));
			}
			return;
		}

		showWatchlistInput = false;
		newWatchlistName = '';
		const watchlistId = parseInt(watchlistIdString);

		// Use our switchToWatchlist function for consistent behavior
		switchToWatchlist(watchlistId);
	}

	function handleWatchlistChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		const value = target.value;

		if (value === 'new') {
			if (!showWatchlistInput) {
				previousWatchlistId = currentWatchlistId;
				handleWatchlistSelection('new');
			}
			showDropdown = false;
			return;
		}

		if (value === 'delete') {
			handleWatchlistSelection('delete');
			showDropdown = false;
			return;
		}

		// Use switchToWatchlist for consistency
		switchToWatchlist(parseInt(value, 10));
		showDropdown = false;

		// Reset select value to placeholder
		target.value = '';
	}

	// Keep currentWatchlistId in sync with the global store
	$: currentWatchlistId = $globalCurrentWatchlistId || 0;

	// Automatically select the first watchlist if none is selected
	$: if (
		$watchlists &&
		$watchlists.length > 0 &&
		(!currentWatchlistId || isNaN(currentWatchlistId))
	) {
		selectWatchlist(String($watchlists[0].watchlistId));
	}

	// Initialize visible watchlists when watchlists and currentWatchlistId are available
	$: if (
		$watchlists &&
		$watchlists.length > 0 &&
		currentWatchlistId &&
		$visibleWatchlistIds.length === 0
	) {
		initializeVisibleWatchlists($watchlists, currentWatchlistId);
	}

	// Handle direct tab switching - maintain fixed positions
	function switchToWatchlist(watchlistId: number) {
		if (watchlistId === currentWatchlistId) return;

		// Check if this watchlist is already visible
		const isVisible = $visibleWatchlistIds.includes(watchlistId);

		if (isVisible) {
			// Just switch - no reordering for visible tabs
			selectWatchlist(String(watchlistId));
		} else {
			// New watchlist from dropdown: add it to the front
			addToVisibleTabs(watchlistId);
			selectWatchlist(String(watchlistId));
		}

		showDropdown = false; // Close dropdown after selection
	}

	// Close dropdown when clicking outside
	function handleClickOutside(event: MouseEvent) {
		const target = event.target as HTMLElement;
		if (showDropdown && target && !target.closest('.dropdown-wrapper')) {
			showDropdown = false;
		}
	}

	// Initialize recent watchlists on mount
	onMount(() => {
		// Set up click outside listener
		document.addEventListener('click', handleClickOutside);

		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});
</script>

<div tabindex="-1" class="feature-container" bind:this={container}>
	<!-- Watchlist Tabs -->
	<div class="watchlist-tabs-container">
		{#if !showWatchlistInput}
			<div class="watchlist-tabs">
				<!-- All Visible Watchlist Tabs in Fixed Positions -->
				{#each visibleWatchlists as watchlist}
					<button
						class="watchlist-tab {currentWatchlistId === watchlist.watchlistId ? 'active' : ''}"
						title={watchlist.watchlistName}
						on:click={() => switchToWatchlist(watchlist.watchlistId)}
					>
						{watchlist.watchlistName}
					</button>
				{/each}

				<!-- More button for dropdown -->
				<div class="dropdown-wrapper">
					<button
						class="more-button"
						title="More Watchlists"
						on:click={() => (showDropdown = !showDropdown)}
					>
						⋯
					</button>

					{#if showDropdown}
						<div class="watchlist-dropdown">
							<select
								class="dropdown-select default-select"
								value=""
								on:change={handleWatchlistChange}
								on:blur={() => (showDropdown = false)}
							>
								<option value="" disabled>Select Watchlist</option>
								{#if Array.isArray($watchlists)}
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
								{/if}
							</select>
						</div>
					{/if}
				</div>
			</div>
		{:else}
			<div class="new-watchlist-section">
				<input
					class="new-watchlist-input default-select"
					id="new-watchlist-input"
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

		<!-- Add Symbol button -->
		<button
			class="add-symbol-button"
			title="Add Symbol"
			on:click={() => addToWatchlist($globalCurrentWatchlistId)}
		>
			+
		</button>
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
	.watchlist-tabs-container {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 12px 2px 12px;
		width: 100%;
		gap: 8px;
	}

	.watchlist-tabs {
		display: flex;
		align-items: center;
		flex-grow: 1;
		min-width: 0;
		gap: 0;
		margin-right: 8px;
	}

	.watchlist-tab {
		padding: 4px 16px;
		color: rgba(255, 255, 255, 0.7);
		font-size: 13px;
		font-weight: normal;
		background: rgba(255, 255, 255, 0.05);
		border: none;
		border-radius: 8px 8px 0 0;
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 140px;
		position: relative;
		margin-right: 1px;
	}

	.watchlist-tab:hover {
		background: rgba(255, 255, 255, 0.1);
		color: rgba(255, 255, 255, 0.9);
	}

	.watchlist-tab.active {
		background: rgba(255, 255, 255, 0.2);
		color: #ffffff;
		font-weight: normal;
	}

	.dropdown-wrapper {
		position: relative;
	}

	.more-button {
		padding: 4px 8px;
		color: rgba(255, 255, 255, 0.7);
		font-size: 16px;
		font-weight: 600;
		background: rgba(255, 255, 255, 0.05);
		border: none;
		border-radius: 8px;
		cursor: pointer;
		min-width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.more-button:hover {
		background: rgba(255, 255, 255, 0.1);
		color: rgba(255, 255, 255, 0.9);
	}

	.watchlist-dropdown {
		position: absolute;
		top: 100%;
		right: 0;
		margin-top: 4px;
		z-index: 100;
		min-width: 200px;
	}

	.dropdown-select {
		width: 100%;
		background: rgba(0, 0, 0, 0.9);
		border: 1px solid rgba(255, 255, 255, 0.3);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
	}

	.new-watchlist-section {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-grow: 1;
		min-width: 0;
	}

	.new-watchlist-input {
		flex-grow: 1;
		min-width: 0;
		padding: 8px 12px;
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.2);
		background: rgba(0, 0, 0, 0.3);
		color: #ffffff;
		font-size: 14px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
	}

	.new-watchlist-input:focus {
		border-color: rgba(255, 255, 255, 0.6);
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.1);
		outline: none;
	}

	.new-watchlist-input::placeholder {
		color: rgba(255, 255, 255, 0.6);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.new-watchlist-buttons {
		display: flex;
		gap: 4px;
	}

	.utility-button {
		padding: 6px 8px;
		color: #ffffff;
		font-size: 14px;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		min-width: 28px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		cursor: pointer;
		transition: all 0.2s ease;
		flex-shrink: 0;
	}

	.utility-button:hover {
		background: rgba(255, 255, 255, 0.2);
		border-color: rgba(255, 255, 255, 0.4);
	}

	.add-symbol-button {
		padding: 6px 8px;
		color: #ffffff;
		font-size: 14px;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		min-width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		transition: none;
		flex-shrink: 0;
	}

	.add-symbol-button:hover {
		background: rgba(255, 255, 255, 0.1);
		color: #ffffff;
	}

	.feature-container {
		display: flex;
		flex-direction: column;
		gap: 4px;
		height: 100%;
		background: transparent;
		border-radius: 0;
		overflow: visible;
		padding: 0;
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
		overflow: visible; /* Remove scrolling from this container */
		min-height: 0; /* Necessary for flex-grow in some cases */
		padding: 0;
		background: transparent;
		border: none;
		border-radius: 0;
		display: flex;
		flex-direction: column;
	}
</style>
