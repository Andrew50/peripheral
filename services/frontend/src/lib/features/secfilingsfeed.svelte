<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { subscribeSECFilings, unsubscribeSECFilings } from '$lib/utils/stream/socket';
	import { addGlobalSECFilingsStream, releaseGlobalSECFilingsStream } from '$lib/utils/stream/interface';
	import { globalFilings, formatTimestamp, handleSECFilingMessage } from '$lib/utils/stream/secfilings';
	import { queryChart } from '$lib/features/chart/interface';
	import List from '$lib/utils/modules/list.svelte';

	// Local component state
	let isLoadingGlobalFilings = true;
	let isSubscribed = false;
	let unsubscribeFn: Function | null = null;

	// Function to handle clicking on a filing
	function handleFilingClick(filing) {
		// Find the security by ticker and load its chart
		if (filing.ticker) {
			queryChart({ ticker: filing.ticker });
		}
	}

	function refreshFilings() {
		isLoadingGlobalFilings = true;
		privateRequest('getLatestEdgarFilings', {})
			.then(filings => {
				console.log('Received filings from API:', filings);
				handleSECFilingMessage(filings);
				isLoadingGlobalFilings = false;
			})
			.catch(error => {
				console.error('Failed to refresh SEC filings:', error);
				isLoadingGlobalFilings = false;
			});
	}

	// Function to handle WebSocket messages for SEC filings
	function handleSocketMessage(data) {
		console.log("SEC Filing message received via socket:", data);
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
		if (isSubscribed) {
			unsubscribeSECFilings();
			isSubscribed = false;
		}
		
		if (unsubscribeFn) {
			unsubscribeFn();
			unsubscribeFn = null;
		}
	});

	// Add this function to handle focus
	function handleTableFocus() {
		// This is just to ensure the table container gets focus
		console.log("Table focused");
	}
</script>

<div class="filings-container">
	<div class="header-section">
		<h2>SEC Filings Feed</h2>
		<button class="refresh-button" on:click={refreshFilings} disabled={isLoadingGlobalFilings}>
			{isLoadingGlobalFilings ? 'Loading...' : 'Refresh'}
		</button>
	</div>

	{#if isLoadingGlobalFilings}
		<div class="loading-container">
			<div class="loading-spinner"></div>
			<div>Loading SEC filings...</div>
		</div>
	{:else if $globalFilings.length === 0}
		<div class="no-data">No SEC filings found</div>
	{:else}
		<!-- Direct List component like in newsfeed.svelte -->
		<List
			list={$globalFilings}
			columns={['ticker', 'type', 'timestamp', 'url']}
			displayNames={{
				ticker: 'Ticker',
				type: 'Filing Type',
				timestamp: 'Date',
				url: 'Link'
			}}
			formatters={{
				timestamp: (value) => formatTimestamp(value),
				url: (value) => 'View Filing'
			}}
			linkColumns={{
				url: (item) => item.url
			}}
			onRowClick={handleFilingClick}
		/>
	{/if}
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
		0% { transform: rotate(0deg); }
		100% { transform: rotate(360deg); }
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