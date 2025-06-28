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
	import { addInstanceToWatchlist as addToWatchlist } from './watchlistUtils';
	// Extended Instance type to include watchlistItemId
	interface WatchlistItem extends Instance {
		watchlistItemId?: number;
	}

	let container: HTMLDivElement;

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
		}
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
			}
		);
	}

	// Helper function to get first letter of watchlist name
	function getWatchlistInitial(name: string): string {
		return name.charAt(0).toUpperCase();
	}
</script>

<div tabindex="-1" class="feature-container" bind:this={container}>
	<!-- Shortcut buttons -->
	<div class="shortcut-container">
		<div class="watchlist-shortcuts">
			{#if Array.isArray($watchlists)}
				{#each $watchlists as watchlist}
					<button
						class="shortcut-button {$globalCurrentWatchlistId === watchlist.watchlistId
							? 'active'
							: ''}"
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
