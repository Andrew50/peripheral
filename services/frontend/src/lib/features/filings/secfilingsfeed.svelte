<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { subscribeSECFilings, unsubscribeSECFilings } from '$lib/utils/stream/socket';
	import {
		addGlobalSECFilingsStream,
		releaseGlobalSECFilingsStream
	} from '$lib/utils/stream/interface';
	import { globalFilings, handleSECFilingMessage, type Filing } from '$lib/utils/stream/secfilings';
	import { queryChart } from '$lib/features/chart/interface';
	import List from '$lib/components/list.svelte';
	import { writable, type Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';

	// Create a writable store that adapts filings to the expected format
	const filingsList: Writable<Instance[]> = writable([]);

	// Local formatTimestamp function
	function formatTimestamp(timestamp: number): string {
		if (!timestamp) return 'N/A';

		const date = new Date(timestamp);
		return date.toLocaleString('en-US', {
			month: 'short',
			day: 'numeric',
			hour: 'numeric',
			minute: '2-digit',
			hour12: true
		});
	}

	// Subscribe to the globalFilings store and transform the data
	const unsubscribeFilings = globalFilings.subscribe((filings) => {
		if (filings) {
			// Convert Filing[] to Instance[]
			filingsList.set(
				filings.map((filing) => ({
					ticker: filing.ticker,
					timestamp: filing.timestamp,
					type: filing.type,
					url: filing.url
				}))
			);
		}
	});

	// Local component state
	let isLoadingGlobalFilings = true;
	let isSubscribed = false;
	let unsubscribeFn: Function | null = null;

	// Function to handle clicking on a filing
	function handleFilingClick(filing: Instance) {
		// Find the security by ticker and load its chart
		if (filing.ticker) {
			queryChart({ ticker: filing.ticker });
		}
	}

	function refreshFilings() {
		isLoadingGlobalFilings = true;
		privateRequest<Filing[]>('getLatestEdgarFilings', {})
			.then((filings) => {
				handleSECFilingMessage(filings);
				isLoadingGlobalFilings = false;
			})
			.catch((error) => {
				console.error('Failed to refresh SEC filings:', error);
				isLoadingGlobalFilings = false;
			});
	}

	// Function to handle WebSocket messages for SEC filings
	function handleSocketMessage(data: any) {
		handleSECFilingMessage(data);
		isLoadingGlobalFilings = false;
	}

	// Subscribe to real-time updates
	onMount(() => {
		subscribeSECFilings();
		isSubscribed = true;

		// Store the unsubscribe function for later cleanup
		unsubscribeFn = addGlobalSECFilingsStream(handleSocketMessage);
	});

	// Clean up subscription on component destroy
	onDestroy(() => {
		if (unsubscribeFn) {
			unsubscribeFn();
		}
		if (isSubscribed) {
			unsubscribeSECFilings();
		}
		unsubscribeFilings();
	});

	// Add this function to handle focus
</script>

<div class="feature-container">
	<div class="feature-header">
		<h2>SEC Filings</h2>
		<div class="feature-controls">
			<button on:click={refreshFilings} disabled={isLoadingGlobalFilings}>
				{isLoadingGlobalFilings ? 'Loading...' : 'Refresh'}
			</button>
		</div>
	</div>

	<div class="feature-content">
		<List
			list={filingsList}
			columns={['ticker', 'type', 'timestamp', 'url']}
			displayNames={{
				ticker: 'Ticker',
				type: 'Type',
				timestamp: 'Time',
				url: 'URL'
			}}
			formatters={{
				timestamp: (value) => formatTimestamp(value)
			}}
			linkColumns={['url']}
			on:rowClick={({ detail }) => handleFilingClick(detail)}
		/>
	</div>
</div>

<style>
	.filings-container {
		display: flex;
		flex-direction: column;
		height: 100%;
		padding: clamp(1rem, 3vw, 1.25rem);
		overflow: hidden; /* Prevent double scrollbars */
	}

	.header-section {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: clamp(1rem, 3vh, 1.25rem);
		flex-shrink: 0; /* Prevent header from shrinking */
	}

	h2 {
		margin: 0;
		font-size: clamp(1.25rem, 2.5vw, 1.5rem);
	}

	.refresh-button {
		background-color: #333;
		color: white;
		border: 1px solid #555;
		padding: clamp(0.375rem, 1vw, 0.5rem) clamp(0.75rem, 2vw, 1rem);
		border-radius: clamp(4px, 0.5vw, 6px);
		cursor: pointer;
	}

	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: clamp(1rem, 3vw, 1.25rem);
		gap: clamp(0.5rem, 1vh, 0.625rem);
		flex: 1;
	}

	.loading-spinner {
		width: clamp(1.5rem, 3vw, 1.875rem);
		height: clamp(1.5rem, 3vw, 1.875rem);
		border: 3px solid #333;
		border-top: 3px solid #fff;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}

	.no-data {
		text-align: center;
		color: #aaa;
		margin-top: clamp(1rem, 3vh, 1.25rem);
		flex: 1;
	}

	/* Make the List component take remaining space and scroll */
	:global(.filings-container > :global(.svelte-list-container)) {
		flex: 1;
		overflow-y: auto;
		min-height: 0;
	}
</style>
