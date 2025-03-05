<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { subscribeSECFilings, unsubscribeSECFilings } from '$lib/utils/stream/socket';
	import {
		addGlobalSECFilingsStream,
		releaseGlobalSECFilingsStream
	} from '$lib/utils/stream/interface';
	import {
		globalFilings,
		formatTimestamp,
		handleSECFilingMessage,
		type Filing
	} from '$lib/utils/stream/secfilings';
	import { queryChart } from '$lib/features/chart/interface';
	import List from '$lib/utils/modules/list.svelte';
	import { writable, type Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';

	// Create a writable store that adapts filings to the expected format
	const filingsList: Writable<Instance[]> = writable([]);

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
	function handleFilingClick(filing: Filing) {
		// Find the security by ticker and load its chart
		if (filing.ticker) {
			queryChart({ ticker: filing.ticker });
		}
	}

	function refreshFilings() {
		isLoadingGlobalFilings = true;
		privateRequest('getLatestEdgarFilings', {})
			.then((filings) => {
				console.log('Received filings from API:', filings);
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
		console.log('SEC Filing message received via socket:', data);
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
	function handleTableFocus() {
		// This is just to ensure the table container gets focus
		console.log('Table focused');
	}
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
			formatters={{
				timestamp: (value) => formatTimestamp(value)
			}}
			linkColumns={{
				url: (item) => item.url
			}}
			on:rowClick={(e) => handleFilingClick(e.detail)}
		/>
	</div>
</div>

<style>
	.filings-container {
		display: flex;
		flex-direction: column;
		height: 100%;
		padding: 20px;
		overflow: hidden; /* Prevent double scrollbars */
	}

	.header-section {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
		flex-shrink: 0; /* Prevent header from shrinking */
	}

	h2 {
		margin: 0;
		font-size: 1.5rem;
	}

	.refresh-button {
		background-color: #333;
		color: white;
		border: 1px solid #555;
		padding: 8px 16px;
		border-radius: 4px;
		cursor: pointer;
	}

	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 20px;
		gap: 10px;
		flex: 1;
	}

	.loading-spinner {
		width: 30px;
		height: 30px;
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
		margin-top: 20px;
		flex: 1;
	}

	/* Make the List component take remaining space and scroll */
	:global(.filings-container > :global(.svelte-list-container)) {
		flex: 1;
		overflow-y: auto;
		min-height: 0;
	}
</style>
