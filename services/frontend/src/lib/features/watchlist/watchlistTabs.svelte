<script lang="ts">
	import type { Watchlist } from '$lib/utils/types/types';
	import {
		flagWatchlistId,
		watchlists,
		currentWatchlistId as globalCurrentWatchlistId
	} from '$lib/utils/stores/stores';
	import { visibleWatchlistIds, addToVisibleTabs } from './watchlistUtils';
	import { selectWatchlist, createNewWatchlist, deleteWatchlist } from './watchlistUtils';

	import { tick } from 'svelte';
	import { get } from 'svelte/store';
	import { onMount } from 'svelte';

	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;

	let tabsContainer: HTMLElement;
	let moreButton: HTMLElement;
	let visibleCount = 10; // Start with a higher number, will be calculated dynamically
	let resizeTimeout: number;

	// Compute visible watchlists based on available space
	$: visibleWatchlists = (() => {
		if (!Array.isArray($watchlists)) return [];

		// Use the visibleWatchlistIds store but limit based on available space
		return $visibleWatchlistIds
			.slice(0, visibleCount) // Use dynamic count instead of hard-coded 3
			.map((id) => $watchlists?.find((w) => w.watchlistId === id))
			.filter((watchlist): watchlist is Watchlist => Boolean(watchlist));
	})();

	// Calculate how many tabs can fit
	function calculateVisibleTabs() {
		if (!tabsContainer || !moreButton || !Array.isArray($watchlists)) return;

		const containerWidth = tabsContainer.offsetWidth;
		const moreButtonWidth = moreButton.offsetWidth + 8; // Include gap
		const availableWidth = containerWidth - moreButtonWidth;

		// Get all tab elements to measure their actual widths
		const tabElements = tabsContainer.querySelectorAll('.watchlist-tab');
		let averageTabWidth: number;

		if (tabElements.length > 0) {
			// Calculate average tab width including gaps from existing tabs
			let totalTabWidth = 0;
			tabElements.forEach((tab) => {
				const rect = tab.getBoundingClientRect();
				totalTabWidth += rect.width;
			});

			// Add gap between tabs (assuming 4px gap from CSS)
			const totalGaps = Math.max(0, tabElements.length - 1) * 4;
			averageTabWidth = (totalTabWidth + totalGaps) / tabElements.length;
		} else {
			// Fallback: estimate tab width based on CSS (padding + content + border)
			// 12px left + 12px right padding + ~20px for single character + 2px border = ~46px
			console.log('no tab elements fallback width used');
			averageTabWidth = 46;
		}

		// Calculate how many tabs can fit
		const maxTabs = Math.floor(availableWidth / averageTabWidth);
		const newVisibleCount = Math.max(1, Math.min(maxTabs, $watchlists.length));

		if (newVisibleCount !== visibleCount) {
			visibleCount = newVisibleCount;
		}
	}

	// Debounced resize handler
	function handleResize() {
		clearTimeout(resizeTimeout);
		resizeTimeout = setTimeout(() => {
			calculateVisibleTabs();
		}, 100);
	}

	onMount(() => {
		// Initial calculation
		tick().then(() => {
			calculateVisibleTabs();
		});

		// Add resize listener
		window.addEventListener('resize', handleResize);

		return () => {
			window.removeEventListener('resize', handleResize);
		};
	});

	// Recalculate when watchlists change
	$: if (Array.isArray($watchlists)) {
		tick().then(() => {
			calculateVisibleTabs();
		});
	}

	function switchToWatchlist(watchlistId: number) {
		if (watchlistId === currentWatchlistId) return;
		const isVisible = $visibleWatchlistIds.includes(watchlistId);
		if (isVisible) {
			selectWatchlist(String(watchlistId));
		} else {
			addToVisibleTabs(watchlistId);
			selectWatchlist(String(watchlistId));
		}
	}

	function handleWatchlistSelection(value: string) {
		if (!value) return;

		if (value === 'new') {
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
				inputElement?.focus();
			});
			return;
		}

		if (value === 'delete') {
			const currentIdNum = Number(currentWatchlistId);
			if (currentIdNum === flagWatchlistId) {
				alert('The flag watchlist cannot be deleted.');
				return;
			}

			const watchlist = Array.isArray($watchlists)
				? $watchlists.find((w) => w.watchlistId === currentIdNum)
				: null;
			const name = watchlist?.watchlistName || `Watchlist #${currentIdNum}`;
			if (confirm(`Delete "${name}"?`)) {
				deleteWatchlist(currentIdNum).catch((e) => alert(e.message));
			}
			return;
		}

		showWatchlistInput = false;
		newWatchlistName = '';
		const idNum = parseInt(value);
		switchToWatchlist(idNum);
	}

	function handleWatchlistChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		const selectedValue = target.value;
		handleWatchlistSelection(selectedValue);

		// Reset the dropdown value to the current watchlist if the action was cancelled or completed
		if (selectedValue === 'delete' || selectedValue === 'new') {
			// Use tick to ensure the DOM is updated before resetting
			tick().then(() => {
				target.value = currentWatchlistId?.toString() || '';
			});
		}
	}

	function newWatchlist() {
		if (!newWatchlistName) return;
		createNewWatchlist(newWatchlistName)
			.then(() => {
				newWatchlistName = '';
				showWatchlistInput = false;
			})
			.catch((e) => alert(e.message));
	}

	function closeInput() {
		showWatchlistInput = false;
		newWatchlistName = '';
		tick().then(() => {
			if (previousWatchlistId) switchToWatchlist(previousWatchlistId);
		});
	}

	// sync current watchlist id
	$: currentWatchlistId = $globalCurrentWatchlistId || 0;
</script>

<div class="watchlist-tabs-container" bind:this={tabsContainer}>
	{#if !showWatchlistInput}
		<div class="watchlist-tabs">
			{#each visibleWatchlists as watchlist}
				<button
					class="watchlist-tab {currentWatchlistId === watchlist.watchlistId ? 'active' : ''}"
					title={watchlist.watchlistName}
					on:click={() => switchToWatchlist(watchlist.watchlistId)}
				>
					{watchlist.watchlistName?.[0]?.toUpperCase()}
				</button>
			{/each}

			<div class="dropdown-wrapper">
				<select
					class="dropdown-select default-select"
					bind:this={moreButton}
					value={currentWatchlistId?.toString()}
					on:change={handleWatchlistChange}
					title="Select Watchlist"
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
		</div>
	{:else}
		<div class="new-watchlist-section">
			<input
				class="new-watchlist-input default-select"
				id="new-watchlist-input"
				bind:this={newNameInput}
				on:keydown={(e) => {
					if (e.key === 'Enter') newWatchlist();
					else if (e.key === 'Escape') closeInput();
				}}
				bind:value={newWatchlistName}
				placeholder="New Watchlist Name"
			/>
			<div class="new-watchlist-buttons">
				<button class="utility-button" on:click={newWatchlist}>✓</button>
				<button class="utility-button" on:click={closeInput}>✕</button>
			</div>
		</div>
	{/if}
</div>

<style>
	.watchlist-tabs-container {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0;
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

	/* Base style to match .metadata-button */
	.watchlist-tab {
		font-family: inherit;
		font-size: 13px;
		line-height: 18px;
		color: rgb(255 255 255 / 90%);
		padding: 6px 10px;
		background: transparent;
		border-radius: 6px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		border: 1px solid transparent;
		cursor: pointer;
		transition: none;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		text-shadow: 0 1px 2px rgb(0 0 0 / 60%);
	}

	.watchlist-tab:hover {
		background: rgb(255 255 255 / 15%);
		border-color: transparent;
		color: #fff;
		box-shadow: 0 2px 8px rgb(0 0 0 / 30%);
	}

	.watchlist-tab:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgb(255 255 255 / 40%);
	}

	/* Active state similar to timeframe-preset-button.active */
	.watchlist-tab.active {
		background: rgb(255 255 255 / 20%);
		border-color: transparent;
		color: #fff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgb(255 255 255 / 20%);
	}

	/* Adjust radius to keep tab top corners rounded and flat bottom */
	.watchlist-tab {
		border-radius: 6px;
		padding: 6px 12px;
	}

	.dropdown-wrapper {
		position: relative;

		/* Push the dropdown to the far right within the flex container */
		margin-left: auto;
	}

	.dropdown-select {
		min-width: 150px;
		background: rgb(0 0 0 / 90%);
		border: 1px solid transparent;
		box-shadow: 0 4px 12px rgb(0 0 0 / 50%);
		color: rgb(255 255 255 / 90%);
		font-size: 13px;
		padding: 6px 10px;
		border-radius: 6px;
		cursor: pointer;
		text-shadow: 0 1px 2px rgb(0 0 0 / 60%);
	}

	.dropdown-select:hover {
		background: rgb(255 255 255 / 15%);
		border-color: transparent;
		color: #fff;
	}

	.dropdown-select:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgb(255 255 255 / 40%);
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
		border: 1px solid rgb(255 255 255 / 20%);
		background: rgb(0 0 0 / 30%);
		color: #fff;
		font-size: 14px;
		text-shadow: 0 1px 2px rgb(0 0 0 / 50%);
	}
</style>
