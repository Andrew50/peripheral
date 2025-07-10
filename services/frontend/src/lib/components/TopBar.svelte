<script lang="ts">
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import type { Instance } from '$lib/utils/types/types';
	import { streamInfo } from '$lib/utils/stores/stores';
	import { timeframeToSeconds } from '$lib/utils/helpers/timestamp';
	import { onMount, onDestroy } from 'svelte';
	import { writable } from 'svelte/store';

	// Add imports for sidebar controls
	import {
		activeMenu,
		watchlists,
		flagWatchlistId,
		currentWatchlistId as globalCurrentWatchlistId,
		currentWatchlistItems
	} from '$lib/utils/stores/stores';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { addInstanceToWatchlist as addToWatchlist } from '$lib/features/watchlist/watchlistUtils';
	import { newPriceAlert } from '$lib/features/alerts/interface';
	import type { Watchlist } from '$lib/utils/types/types';
	import { get } from 'svelte/store';
	import { tick } from 'svelte';

	export let instance: Instance;
	export let menuWidth: number = 0;

	const commonTimeframes = ['1', '1h', '1d', '1w'];
	let countdown = writable('--');
	let countdownInterval: ReturnType<typeof setInterval>;
	// Helper computed value to check if current timeframe is custom
	$: isCustomTimeframe = instance?.timeframe && !commonTimeframes.includes(instance.timeframe);

	// Watchlist controls state
	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;

	// Keep local ID in sync with global store
	$: currentWatchlistId = $globalCurrentWatchlistId || 0;

	// Extended Instance type to include watchlistItemId
	interface WatchlistItem extends Instance {
		watchlistItemId?: number;
	}

	// --- New Handlers for Buttons ---
	function handleTickerClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		queryInstanceInput([], ['ticker'], instance, 'ticker')
			.then((v: Instance) => {
				if (v) queryChart(v, true);
			})
			.catch((error) => {
				// Handle cancellation silently
				if (error.message !== 'User cancelled input') {
					console.error('Error in ticker input:', error);
				}
			});
	}
	function handleTickerKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			event.stopPropagation(); // Prevent legend collapse toggle
			queryInstanceInput('any', ['ticker'], instance, 'ticker')
				.then((v: Instance) => {
					if (v) queryChart(v, true);
				})
				.catch((error) => {
					// Handle cancellation silently
					if (error.message !== 'User cancelled input') {
						console.error('Error in ticker input:', error);
					}
				});
		}
	}
	function handleSessionClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		if (instance) {
			const updatedInstance = { ...instance, extendedHours: !instance.extendedHours };
			queryChart(updatedInstance, true);
		}
	}
	// Function to handle clicking the "..." timeframe button
	function handleCustomTimeframeClick() {
		// Start with empty input but force timeframe type
		queryInstanceInput(['timeframe'], ['timeframe'], instance, 'timeframe')
			.then((v: Instance) => {
				if (v) queryChart(v, true);
			})
			.catch((error) => {
				// Handle cancellation silently
				if (error.message !== 'User cancelled input') {
					console.error('Error in timeframe input:', error);
				}
			});
	}

	// Function to handle selecting a preset timeframe button
	function selectTimeframe(newTimeframe: string) {
		if (instance && instance.timeframe !== newTimeframe) {
			const updatedInstance = { ...instance, timeframe: newTimeframe };
			queryChart(updatedInstance, true);
		}
	}

	// Function to handle calendar button click
	function handleCalendarClick() {
		// Dispatch a custom event that the parent can listen to
		const event = new CustomEvent('calendar-click');
		document.dispatchEvent(event);
	}

	// Watchlist control functions
	function addInstanceToWatchlist(securityId?: number) {
		addToWatchlist(currentWatchlistId, securityId);
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
			selectWatchlist(String(previousWatchlistId));
		});
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
				const inputElement = document.getElementById('new-watchlist-input') as HTMLInputElement;
				if (inputElement) {
					inputElement.focus();
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
			})
			.catch((err) => {
				currentWatchlistItems.set([]);
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

	// Alert control functions
	async function createPriceAlert() {
		// Prompt user for ticker & price, then save via API helper
		const inst = await queryInstanceInput('any', ['ticker'], {
			ticker: ''
		});
		await newPriceAlert(inst);
	}

	function formatTime(seconds: number): string {
		const years = Math.floor(seconds / (365 * 24 * 60 * 60));
		const months = Math.floor((seconds % (365 * 24 * 60 * 60)) / (30 * 24 * 60 * 60));
		const weeks = Math.floor((seconds % (30 * 24 * 60 * 60)) / (7 * 24 * 60 * 60));
		const days = Math.floor((seconds % (7 * 24 * 60 * 60)) / (24 * 60 * 60));
		const hours = Math.floor((seconds % (24 * 60 * 60)) / (60 * 60));
		const minutes = Math.floor((seconds % (60 * 60)) / 60);
		const secs = Math.floor(seconds % 60);

		if (years > 0) return `${years}y ${months}m`;
		if (months > 0) return `${months}m ${weeks}w`;
		if (weeks > 0) return `${weeks}w ${days}d`;
		if (days > 0) return `${days}d ${hours}h`;
		if (hours > 0) return `${hours}h ${minutes}m`;
		if (minutes > 0) return `${minutes}m ${secs < 10 ? '0' : ''}${secs}s`;
		return `${secs < 10 ? '0' : ''}${secs}s`;
	}

	function calculateCountdown() {
		if (!instance?.timeframe) {
			countdown.set('--');
			return;
		}

		const currentTimeInSeconds = Math.floor($streamInfo.timestamp / 1000);
		const chartTimeframeInSeconds = timeframeToSeconds(instance.timeframe);

		let nextBarClose =
			currentTimeInSeconds -
			(currentTimeInSeconds % chartTimeframeInSeconds) +
			chartTimeframeInSeconds;

		// For daily timeframes, adjust to market close (4:00 PM EST)
		if (instance.timeframe.includes('d')) {
			const currentDate = new Date(currentTimeInSeconds * 1000);
			const estOptions = { timeZone: 'America/New_York' };
			const formatter = new Intl.DateTimeFormat('en-US', {
				...estOptions,
				year: 'numeric',
				month: 'numeric',
				day: 'numeric'
			});

			const [month, day, year] = formatter.format(currentDate).split('/');

			const marketCloseDate = new Date(
				`${year}-${month.padStart(2, '0')}-${day.padStart(2, '0')}T16:00:00-04:00`
			);

			nextBarClose = Math.floor(marketCloseDate.getTime() / 1000);

			if (currentTimeInSeconds >= nextBarClose) {
				marketCloseDate.setDate(marketCloseDate.getDate() + 1);

				const dayOfWeek = marketCloseDate.getDay(); // 0 = Sunday, 6 = Saturday
				if (dayOfWeek === 0) {
					// Sunday
					marketCloseDate.setDate(marketCloseDate.getDate() + 1); // Move to Monday
				} else if (dayOfWeek === 6) {
					// Saturday
					marketCloseDate.setDate(marketCloseDate.getDate() + 2); // Move to Monday
				}

				nextBarClose = Math.floor(marketCloseDate.getTime() / 1000);
			}
		}

		const remainingTime = nextBarClose - currentTimeInSeconds;

		if (remainingTime > 0) {
			countdown.set(formatTime(remainingTime));
		} else {
			countdown.set('Bar Closed');
		}
	}

	onMount(() => {
		countdownInterval = setInterval(calculateCountdown, 1000);
		calculateCountdown(); // Initial calculation

		// Initialize watchlist functionality
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

	onDestroy(() => {
		if (countdownInterval) {
			clearInterval(countdownInterval);
		}
	});
</script>

<div class="top-bar">
	<!-- Left side content -->
	<div class="top-bar-left">
		<button
			class="symbol metadata-button"
			on:click={handleTickerClick}
			on:keydown={handleTickerKeydown}
			aria-label="Change ticker"
		>
			<svg class="search-icon" viewBox="0 0 24 24" width="18" height="18" fill="none">
				<path
					d="M21 21L16.514 16.506L21 21ZM19 10.5C19 15.194 15.194 19 10.5 19C5.806 19 2 15.194 2 10.5C2 5.806 5.806 2 10.5 2C15.194 2 19 5.806 19 10.5Z"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
			{instance?.ticker || 'NaN'}
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<!-- Add common timeframe buttons -->
		{#each commonTimeframes as tf}
			<button
				class="timeframe-preset-button metadata-button {instance?.timeframe === tf ? 'active' : ''}"
				on:click={() => selectTimeframe(tf)}
				aria-label="Set timeframe to {tf}"
				aria-pressed={instance?.timeframe === tf}
			>
				{tf}
			</button>
		{/each}
		<!-- Button to open custom timeframe input -->
		<button
			class="timeframe-custom-button metadata-button {isCustomTimeframe ? 'active' : ''}"
			on:click={handleCustomTimeframeClick}
			aria-label="Select custom timeframe"
			aria-pressed={isCustomTimeframe ? 'true' : 'false'}
		>
			{#if isCustomTimeframe}
				{instance.timeframe}
			{:else}
				...
			{/if}
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<button
			class="session-type metadata-button"
			on:click={handleSessionClick}
			aria-label="Toggle session type"
		>
			{instance?.extendedHours ? 'Extended' : 'Regular'}
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<!-- Calendar button for timestamp selection -->
		<button
			class="calendar-button metadata-button"
			on:click={handleCalendarClick}
			title="Go to Date"
			aria-label="Go to Date"
		>
			<svg
				viewBox="0 0 24 24"
				width="16"
				height="16"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					d="M19 3H18V1H16V3H8V1H6V3H5C3.89 3 3 3.9 3 5V19C3 20.1 3.89 21 5 21H19C20.11 21 21 20.1 21 19V5C21 3.9 20.11 3 19 3ZM19 19H5V8H19V19ZM7 10H12V15H7V10Z"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<!-- Countdown -->
		<div class="countdown-container">
			<span class="countdown-label">Next Bar Close:</span>
			<span class="countdown-value">{$countdown}</span>
		</div>
	</div>

	<!-- Right side - Sidebar Controls -->
	<div class="top-bar-right" style="right: {menuWidth > 0 ? menuWidth + 45 : 45}px;">
		{#if $activeMenu === 'watchlist'}
			<!-- Watchlist Controls -->
			<div class="sidebar-controls">
				{#if Array.isArray($watchlists)}
					<div class="watchlist-controls-left">
						<div class="watchlist-selector">
							<div class="select-wrapper">
								<select
									class="default-select metadata-button"
									id="watchlists-topbar"
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
										{#if currentWatchlistId !== undefined && currentWatchlistId !== flagWatchlistId}
											<option value="delete">- Delete Current Watchlist</option>
										{/if}
									</optgroup>
								</select>
								<div class="caret-icon">
									<svg
										xmlns="http://www.w3.org/2000/svg"
										viewBox="0 0 24 24"
										fill="none"
										stroke="currentColor"
										stroke-width="2"
										stroke-linecap="round"
										stroke-linejoin="round"
									>
										<polyline points="6,9 12,15 18,9"></polyline>
									</svg>
								</div>
							</div>
						</div>
					</div>

					<div class="watchlist-controls-right">
						<button
							class="add-item-button metadata-button"
							title="Add Symbol"
							on:click={() => addInstanceToWatchlist()}>+</button
						>
					</div>
				{/if}

				{#if showWatchlistInput}
					<div class="new-watchlist-container">
						<input
							class="input metadata-button"
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
							<button class="utility-button metadata-button" on:click={newWatchlist}>✓</button>
							<button class="utility-button metadata-button" on:click={closeNewWatchlistWindow}
								>✕</button
							>
						</div>
					</div>
				{/if}
			</div>
		{:else if $activeMenu === 'alerts'}
			<!-- Alert Controls -->
			<div class="sidebar-controls">
				<div class="alert-controls-right">
					<button
						class="create-alert-btn metadata-button"
						on:click={createPriceAlert}
						title="Create New Price Alert"
					>
						Create Alert
					</button>
				</div>
			</div>
		{/if}
	</div>
</div>

<style>
	.top-bar {
		height: 40px;
		min-height: 40px;
		background-color: #0f0f0f;
		display: flex;
		justify-content: space-between; /* Restore space-between for proper positioning */
		align-items: center;
		padding: 0 10px;
		flex-shrink: 0;
		width: 100%;
		z-index: 10;
		border-bottom: 4px solid var(--c1);
		position: absolute; /* Position absolutely */
		top: 0;
		left: 0;
		right: 0;
	}

	.top-bar-left {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.top-bar-right {
		position: absolute;
		top: 0;
		height: 40px;
		display: flex;
		align-items: center;
		padding-left: 16px; /* Add space after countdown section */
	}

	/* Base styles for metadata buttons */
	.metadata-button {
		font-family: inherit;
		font-size: 13px;
		line-height: 18px;
		color: rgba(255, 255, 255, 0.9);
		padding: 6px 10px;
		background: transparent;
		border-radius: 6px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		border: 1px solid transparent;
		cursor: pointer;
		transition: none;
		text-align: left;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.metadata-button:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.4);
	}

	.metadata-button:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: transparent;
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	/* Specific style adjustments for symbol button */
	.symbol.metadata-button {
		font-size: 14px;
		line-height: 20px;
		color: #ffffff;
		padding: 6px 12px;
		gap: 4px;
	}

	.top-bar .search-icon {
		opacity: 0.8;
		transition: opacity 0.2s ease;
		position: static;
		padding: 0;
		left: auto;
	}

	.top-bar .symbol.metadata-button:hover .search-icon {
		opacity: 1;
	}

	/* Styles for preset timeframe buttons */
	.timeframe-preset-button {
		min-width: 24px;
		text-align: center;
		padding: 6px 4px;
		display: inline-flex;
		justify-content: center;
		align-items: center;
		margin-left: -2px; /* Reduce spacing between timeframe buttons */
	}

	.timeframe-preset-button:first-of-type {
		margin-left: 0; /* Don't apply negative margin to first timeframe button */
	}

	.timeframe-preset-button.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Styles for the custom timeframe '...' button */
	.timeframe-custom-button {
		padding: 6px 4px;
		min-width: 24px;
		text-align: center;
		display: inline-flex;
		justify-content: center;
		align-items: center;
		margin-left: -2px; /* Reduce spacing with preceding timeframe buttons */
	}

	.timeframe-custom-button.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Calendar button styles */
	.calendar-button {
		padding: 6px 8px;
		min-width: auto;
		display: inline-flex;
		justify-content: center;
		align-items: center;
	}

	.calendar-button svg {
		opacity: 0.8;
		transition: opacity 0.2s ease;
	}

	.calendar-button:hover svg {
		opacity: 1;
	}

	/* Countdown styles */
	.countdown-container {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		background: transparent;
		border-radius: 6px;
		border: none;
		color: rgba(255, 255, 255, 0.9);
		font-size: 13px;
		line-height: 18px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
		transition: none;
	}

	.countdown-container:hover {
		background: rgba(255, 255, 255, 0.15);
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	.countdown-label {
		color: inherit;
		font-size: inherit;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.countdown-value {
		font-family: inherit;
		font-weight: 600;
		font-size: inherit;
		color: inherit;
		min-width: 45px;
		text-align: center;
	}

	/* Divider styles */
	.divider {
		width: 1px;
		height: 28px;
		background: rgba(255, 255, 255, 0.15);
		margin: 0 6px;
		flex-shrink: 0;
	}

	/* Sidebar controls styles */
	.sidebar-controls {
		display: flex;
		align-items: center;
		justify-content: flex-start; /* Left align the controls */
		width: auto; /* Don't force full width */
		min-width: 200px;
		height: 40px;
		gap: 8px; /* Add space between left and right controls */
	}

	.watchlist-controls-left {
		display: flex;
		align-items: center;
		justify-content: flex-start;
		margin-left: 0;
		padding-left: 0;
	}

	.watchlist-controls-right {
		display: flex;
		align-items: center;
		justify-content: flex-start; /* Keep close to left controls */
	}

	.alert-controls-right {
		display: flex;
		align-items: center;
		justify-content: flex-start; /* Left align like watchlist controls */
		width: 100%;
		padding-right: 8px; /* Add padding for visual breathing room */
	}

	.watchlist-selector .select-wrapper {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: flex-start; /* Ensure left alignment */
		width: fit-content;
		background: transparent;
		border-radius: 6px;
		padding: 0;
		margin-left: 0;
	}

	.watchlist-selector select {
		flex: 0 1 auto;
		min-width: fit-content;
		width: fit-content;
		background: transparent;
		color: #ffffff;
		border: none;
		border-radius: 0;
		padding: 6px 8px 6px 8px;
		margin-left: 0;
		font-size: 13px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
		appearance: none;
		-webkit-appearance: none;
		-moz-appearance: none;
		text-align: left;
	}

	.watchlist-selector .caret-icon {
		margin-left: 4px;
		margin-right: 6px;
		width: 12px;
		height: 12px;
		color: #ffffff;
		pointer-events: none;
		flex-shrink: 0;
	}

	.watchlist-selector .caret-icon svg {
		width: 100%;
		height: 100%;
	}

	.watchlist-selector .select-wrapper:hover {
		background: rgba(255, 255, 255, 0.15);
	}

	.add-item-button {
		color: #ffffff;
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 16px;
		font-weight: 300;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		background: transparent;
		border: none;
		border-radius: 6px;
		transition: all 0.2s ease;
		padding: 6px 8px;
	}

	.add-item-button:hover {
		background: rgba(255, 255, 255, 0.15);
		color: #ffffff;
	}

	.new-watchlist-container {
		position: absolute;
		top: 100%;
		left: 0; /* Align with left edge of top-bar-right (which is positioned at drag bar edge) */
		margin-top: 8px;
		padding: 12px;
		background: rgba(0, 0, 0, 0.9);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 8px;
		z-index: 100;
		min-width: 250px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
	}

	.new-watchlist-container .input {
		width: 100%;
		margin-bottom: 8px;
		padding: 8px 12px;
		border-radius: 6px;
		border: 1px solid rgba(255, 255, 255, 0.2);
		background: rgba(0, 0, 0, 0.3);
		color: #ffffff;
		font-size: 13px;
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
		gap: 6px;
	}

	.new-watchlist-buttons .utility-button {
		padding: 6px 12px;
		color: #ffffff;
		font-size: 13px;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
		min-width: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
	}

	.new-watchlist-buttons .utility-button:hover {
		background: rgba(255, 255, 255, 0.2);
		border-color: rgba(255, 255, 255, 0.4);
	}

	.create-alert-btn {
		padding: 6px 12px;
		font-size: 13px;
		font-weight: 600;
		color: #ffffff;
		background: transparent;
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		transition: all 0.2s ease;
	}

	.create-alert-btn:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: rgba(255, 255, 255, 0.4);
	}
</style>
