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

	let newWatchlistName = '';
	let currentWatchlistId: number;
	let previousWatchlistId: number;
	let newNameInput: HTMLInputElement;
	let showWatchlistInput = false;
	let showDropdown = false;

	// Compute visible watchlists
	$: visibleWatchlists = $visibleWatchlistIds
		.slice(0, 3)
		.map((id) => $watchlists?.find((w) => w.watchlistId === id))
		.filter((watchlist): watchlist is Watchlist => Boolean(watchlist));

	function switchToWatchlist(watchlistId: number) {
		if (watchlistId === currentWatchlistId) return;
		const isVisible = $visibleWatchlistIds.includes(watchlistId);
		if (isVisible) {
			selectWatchlist(String(watchlistId));
		} else {
			addToVisibleTabs(watchlistId);
			selectWatchlist(String(watchlistId));
		}
		showDropdown = false;
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
		handleWatchlistSelection(target.value);
		// No need to reset the value; keeping it allows the dropdown to reopen with the current selection.
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

<div class="watchlist-tabs-container">
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
	.watchlist-tab,
	.more-button {
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
		display: inline-flex;
		align-items: center;
		gap: 4px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.watchlist-tab:hover,
	.more-button:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: transparent;
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	.watchlist-tab:focus,
	.more-button:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.4);
	}

	/* Active state similar to timeframe-preset-button.active */
	.watchlist-tab.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Adjust radius to keep tab top corners rounded and flat bottom */
	.watchlist-tab {
		border-radius: 6px;
		padding: 6px 12px;
	}

	.more-button {
		padding: 6px 8px;
		min-width: 28px;
		height: 28px;
		justify-content: center;
	}

	.dropdown-wrapper {
		position: relative;
		/* Push the dropdown (three-dots button) to the far right within the flex container */
		margin-left: auto;
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
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
	}
</style>
