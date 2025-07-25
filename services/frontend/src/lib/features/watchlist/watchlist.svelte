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
		addMultipleInstancesToWatchlist,
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

	export let showTabs: boolean = true;

	let container: HTMLDivElement;
	// Watchlist tab functionality
	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;
	let showDropdown = false;

	// Track visible watchlists for tabs in fixed order

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

				// Also update flagWatchlist if this is the flag watchlist
				if (currentWatchlistId === flagWatchlistId) {
					import('$lib/utils/stores/stores').then(({ flagWatchlist }) => {
						flagWatchlist.update((items: WatchlistItem[]) => {
							return items.filter((i: WatchlistItem) => i.watchlistItemId !== item.watchlistItemId);
						});
					});
				}
			}
		);
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

		// No manual reset; keeping the value ensures the dropdown remains synced.
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
	// Keep currentWatchlistId in sync with the global store
	$: currentWatchlistId = $globalCurrentWatchlistId || 0;

	// Full name of the currently selected watchlist for display in header row
	$: currentWatchlistName = Array.isArray($watchlists)
		? ($watchlists.find((w) => w.watchlistId === currentWatchlistId)?.watchlistName ?? '')
		: '';

	// Automatically select the first watchlist if none is selected
	// This serves as a fallback in case the store initialization didn't run
	$: if (
		$watchlists &&
		$watchlists.length > 0 &&
		(!currentWatchlistId || isNaN(currentWatchlistId))
	) {
		selectWatchlist(String($watchlists[0].watchlistId));
	}

	// Initialize visible watchlists when watchlists and currentWatchlistId are available
	// This serves as a fallback in case the store initialization didn't run
	$: if (
		$watchlists &&
		$watchlists.length > 0 &&
		currentWatchlistId &&
		$visibleWatchlistIds.length === 0
	) {
		initializeVisibleWatchlists($watchlists, currentWatchlistId);
	}
	// Get all visible watchlists in their fixed positions
	$: visibleWatchlists = $visibleWatchlistIds
		.map((id) => $watchlists?.find((w) => w.watchlistId === id))
		.filter((watchlist): watchlist is Watchlist => Boolean(watchlist));
</script>

<div tabindex="-1" class="feature-container" bind:this={container}>
	<!-- Watchlist Tabs -->
	{#if showTabs}
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
							{watchlist.watchlistName?.[0]?.toUpperCase()}
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
									value={currentWatchlistId?.toString()}
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
											{#if currentWatchlistId !== undefined && currentWatchlistId !== flagWatchlistId}
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
		</div>
	{/if}

	<!-- Wrap List component for scrolling -->
	<div class="list-scroll-container">
		<!-- Top row containing the Add Symbol button -->
		<div class="add-symbol-row">
			<!-- Alert settings button (commented out) -->
			<!--
			<button
				class="alert-settings-button"
				title="Watchlist Alerts Settings"
				on:click={() => {
					/* TODO: implement alert settings */
				}}
			>
				<img src="/alerts.png" alt="Alerts" class="icon" />
			</button>
			-->

			<!-- Add Symbol button -->
			<button
				class="add-symbol-button"
				title="Add Symbol (Multi-add)"
				on:click={() => addMultipleInstancesToWatchlist($globalCurrentWatchlistId)}
			>
				+
			</button>
		</div>

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
		border-radius: 8px;
		cursor: pointer;
		transition: background 0.2s ease;
		flex-shrink: 0;
	}

	.add-symbol-button:hover {
		background: rgba(255, 255, 255, 0.1);
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
		/* Match padding pattern used in alerts container */
		background: transparent;
		border: none;
		border-radius: 0;
		display: flex;
		flex-direction: column;
	}

	/* Row that houses the + button */
	.add-symbol-row {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 4px;
		/* Provide a bit of breathing room above header */
		padding: 0;
		margin: 16px 0 10px 0;
	}

	/* Adjust left padding of first data column (Ticker) to match Alerts list */
	:global(.header-table th:nth-child(2)) {
		padding-left: clamp(4px, 0.5vw, 8px) !important;
	}

	:global(.body-table td:nth-child(2)) {
		padding-left: clamp(4px, 0.5vw, 8px) !important;
	}

	/* Alert Settings button - shares base style with add-symbol-button */
	.alert-settings-button {
		padding: 6px 8px;
		color: #ffffff;
		font-size: 14px;
		font-weight: 600;
		background: transparent;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		transition: background 0.2s ease;
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 32px;
		height: 32px;
		flex-shrink: 0;
	}

	.alert-settings-button:hover {
		background: rgba(255, 255, 255, 0.1);
	}

	.alert-settings-button .icon {
		width: 20px;
		height: 20px;
		/* Make sure the PNG appears white regardless of original color */
		filter: brightness(0) invert(1);
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
		border-radius: 8px;
		cursor: pointer;
		transition: background 0.2s ease;
		flex-shrink: 0;
	}
</style>
