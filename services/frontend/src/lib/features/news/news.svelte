<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { writable } from 'svelte/store';
	import List from '$lib/components/list.svelte';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import { activeChartInstance } from '$lib/utils/stores/stores';
	import {
		addGlobalSECFilingsStream,
		releaseGlobalSECFilingsStream
	} from '$lib/utils/stream/interface';
	import { subscribeSECFilings, unsubscribeSECFilings } from '$lib/utils/stream/socket';
	import type { StreamCallback, StreamData } from '$lib/utils/stream/socket';
	import type { Filing } from '$lib/utils/stream/secfilings';

	// Add tab state
	let activeTab = 'filings';

	// Store for SEC filings
	let filings = writable<Filing[]>([]);
	let isLoadingFilings = false;

	// Store for global SEC filings
	let globalFilings = writable<Filing[]>([]);
	let isLoadingGlobalFilings = false;

	let isLoadingNews = false;

	// Current ticker being viewed
	let currentTicker = '';
	let currentSecurityId: number | null = null;

	// Subscribe to active chart instance to get current ticker
	activeChartInstance.subscribe((chartInstance) => {
		if (chartInstance?.ticker && chartInstance?.securityId) {
			currentTicker = chartInstance.ticker;
			currentSecurityId = Number(chartInstance.securityId);

			// Load filings for the new ticker if we're on the filings tab
			if (activeTab === 'filings') {
				loadFilings();
			}
		}
	});

	// Function to load SEC filings for the current ticker
	async function loadFilings() {
		if (!currentSecurityId) return;

		isLoadingFilings = true;

		try {
			// Get current time for "to" parameter
			const now = Date.now();

			// Get filings from 2 years ago to now
			const twoYearsAgo = now - 2 * 365 * 24 * 60 * 60 * 1000;

			const result = await privateRequest<Filing[]>('getEdgarFilings', {
				securityId: currentSecurityId,
				from: twoYearsAgo,
				to: now,
				limit: 100
			});

			filings.set(result);
		} catch (error) {
			console.error('Failed to load SEC filings:', error);
			filings.set([]);
		} finally {
			isLoadingFilings = false;
		}
	}

	// Function to handle incoming global SEC filing messages
	function handleGlobalSECFilingMessage(message: StreamData) {
		// Check if the message has a data property that is an array
		if (typeof message === 'object' && 'data' in message && message.data) {
			if (Array.isArray(message.data)) {
				globalFilings.set(message.data as Filing[]);
				isLoadingGlobalFilings = false;
			} else {
				// Handle single filing update
				globalFilings.update((currentFilings) => {
					// Add the new filing at the beginning of the array
					const updatedFilings = [message.data as Filing, ...currentFilings];
					// Keep only the most recent 100 filings
					if (updatedFilings.length > 100) {
						return updatedFilings.slice(0, 100);
					}
					return updatedFilings;
				});
			}
		} else {
			console.error('Received unexpected message format:', message);
		}
	}

	// Function to subscribe to global SEC filings
	function subscribeToGlobalFilings() {
		isLoadingGlobalFilings = true;

		// First unsubscribe if we're already subscribed
		if (unsubscribeGlobalFilings) {
			unsubscribeGlobalFilings();
		}

		// Clear current filings to show we're refreshing
		globalFilings.set([]);

		// Use the addGlobalSECFilingsStream function to subscribe
		unsubscribeGlobalFilings = addGlobalSECFilingsStream(handleGlobalSECFilingMessage);
	}

	// Load appropriate data when tab changes
	function handleTabChange(tab: string) {
		if (tab === 'filings' && activeTab !== 'filings') {
			// Subscribe when switching to filings tab
			subscribeSECFilings();
			secFilingsSubscribeFn = addGlobalSECFilingsStream(handleGlobalSECFilingMessage);
		} else if (activeTab === 'filings' && tab !== 'filings') {
			// Unsubscribe when switching away from filings tab
			if (secFilingsSubscribeFn) {
				secFilingsSubscribeFn();
				secFilingsSubscribeFn = null;
			}
			unsubscribeSECFilings();
		}

		activeTab = tab;
	}

	// Variable to hold the unsubscribe function
	let unsubscribeGlobalFilings: Function | null = null;
	let secFilingsSubscribeFn: Function | null = null;

	// Initial load
	onMount(() => {
		if (currentSecurityId && activeTab === 'filings') {
			loadFilings();
		}
		if (activeTab === 'filings') {
			subscribeToGlobalFilings();
		}
	});

	// Clean up on destroy
	onDestroy(() => {
		// Unsubscribe from the global SEC filings channel
		if (unsubscribeGlobalFilings) {
			unsubscribeGlobalFilings();
		}
		if (secFilingsSubscribeFn) {
			secFilingsSubscribeFn();
			unsubscribeSECFilings();
		}
	});
</script>

<div class="newsfeed-container">
	<!-- Tab Navigation -->
	<div class="tab-navigation">
		<button
			class={activeTab === 'filings' ? 'active' : ''}
			on:click={() => { activeTab = 'filings'; subscribeToGlobalFilings(); }}
		>
			Global SEC Filings
		</button>
		<button
			class={activeTab === 'ticker-filings' ? 'active' : ''}
			on:click={() => (activeTab = 'ticker-filings')}
		>
			Current Ticker Filings
		</button>
	</div>

	<!-- SEC Filings Tab -->
	{#if activeTab === 'filings'}
		<div class="tab-content">
			<div class="header-section">
				<h2>Global SEC Filings</h2>
				<button
					class="refresh-button"
					on:click={subscribeToGlobalFilings}
					disabled={isLoadingGlobalFilings}
				>
					{isLoadingGlobalFilings ? 'Loading...' : 'Refresh'}
				</button>
			</div>

			{#if isLoadingGlobalFilings}
				<div class="loading-container">
					<div class="loading-spinner"></div>
					<span>Loading global SEC filings...</span>
				</div>
			{:else}
				<List
					list={globalFilings}
					columns={['ticker', 'type', 'timestamp', 'url']}
					displayNames={{
						ticker: 'Ticker',
						type: 'Type',
						timestamp: 'Time',
						url: 'URL'
					}}
					formatters={{
						timestamp: (value) => UTCTimestampToESTString(value),
						url: (value) => value
					}}
				/>
			{/if}
		</div>
	{/if}

	<!-- Current Ticker SEC Filings Tab -->
	{#if activeTab === 'ticker-filings'}
		<div class="tab-content">
			<div class="header-section">
				<h2>SEC Filings for {currentTicker}</h2>
			</div>

			{#if isLoadingFilings}
				<div class="loading-container">
					<div class="loading-spinner"></div>
					<span>Loading SEC filings...</span>
				</div>
			{:else if !currentTicker}
				<div class="no-data">Select a ticker to view its SEC filings</div>
			{:else}
				<List
					list={filings}
					columns={['type', 'timestamp', 'url']}
					displayNames={{
						type: 'Type',
						timestamp: 'Time',
						url: 'URL'
					}}
					formatters={{
						timestamp: (value) => UTCTimestampToESTString(value),
						url: (value) => value
					}}
				/>
			{/if}
		</div>
	{/if}

</div>

<style>
	.newsfeed-container {
		padding: 20px;
		color: white;
		width: 100%;
		min-width: 0; /* Allow container to shrink */
		overflow-x: auto; /* Enable horizontal scrolling if needed */
		height: 100%;
		display: flex;
		flex-direction: column;
	}

	.tab-navigation {
		display: flex;
		gap: 10px;
		margin-bottom: 20px;
		border-bottom: 1px solid #444;
		padding-bottom: 10px;
		flex-wrap: wrap; /* Allow wrapping */
	}

	.tab-navigation button {
		background-color: #222;
		border: 1px solid #444;
		color: #aaa;
		padding: 8px 16px;
		border-radius: 4px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.tab-navigation button.active {
		color: white;
		background-color: #444;
	}

	.tab-navigation button:hover {
		color: white;
		background-color: #333;
	}

	.tab-content {
		padding: 20px 0;
		flex: 1;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
	}

	.header-section {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}

	.header-section h2 {
		margin: 0;
		font-size: 1.5em;
	}

	.refresh-button {
		background-color: #333;
		color: white;
		border: 1px solid #555;
		padding: 8px 16px;
		border-radius: 4px;
		cursor: pointer;
	}

	.refresh-button:hover {
		background-color: #444;
	}

	.refresh-button:disabled {
		background-color: #222;
		color: #666;
		cursor: not-allowed;
	}

	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 40px;
		color: #aaa;
	}

	.loading-spinner {
		width: 30px;
		height: 30px;
		border: 3px solid #333;
		border-top: 3px solid #fff;
		border-radius: 50%;
		animation: spin 1s linear infinite;
		margin-bottom: 10px;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}
</style>
