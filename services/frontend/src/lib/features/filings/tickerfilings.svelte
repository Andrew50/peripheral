<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { activeChartInstance } from '$lib/utils/stores/stores';
	import { writable } from 'svelte/store';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import List from '$lib/components/list.svelte';

	// Store for ticker filings
	let filings = writable<Filing[]>([]);
	let isLoading = false;
	let message = '';

	// Current ticker being viewed
	let currentTicker = '';
	let currentSecurityId: number | null = null;

	// Interface for SEC filings
	interface Filing {
		type: string;
		date: string;
		url: string;
		timestamp: number;
	}

	// Subscribe to active chart instance to get current ticker
	const unsubscribe = activeChartInstance.subscribe((chartInstance) => {
		if (chartInstance?.ticker && chartInstance?.securityId) {
			if (currentTicker !== chartInstance.ticker) {
				currentTicker = chartInstance.ticker;
				currentSecurityId = Number(chartInstance.securityId);
				fetchFilings();
			}
		}
	});

	// Function to fetch SEC filings for the current ticker
	function fetchFilings() {
		if (!currentSecurityId) {
			message = 'No ticker selected';
			filings.set([]);
			return;
		}

		isLoading = true;
		message = `Loading SEC filings for ${currentTicker}...`;

		privateRequest('getEdgarFilings', {
			securityId: currentSecurityId
		})
			.then((response) => {
				if (Array.isArray(response) && response.length > 0) {
					filings.set(response);
					message = `Found ${response.length} SEC filings for ${currentTicker}`;
				} else {
					filings.set([]);
					message = `No SEC filings found for ${currentTicker}`;
				}
				isLoading = false;
			})
			.catch((error) => {
				console.error('Error fetching SEC filings:', error);
				message = `Error loading SEC filings for ${currentTicker}`;
				filings.set([]);
				isLoading = false;
			});
	}

	// Format timestamp for display
	function formatDate(timestamp: number): string {
		if (!timestamp) return 'N/A';
		return UTCTimestampToESTString(timestamp);
	}

	// Open filing URL in a new tab
	function openFiling(url: string) {
		window.open(url, '_blank');
	}

	// Clean up subscription when component is destroyed
	onDestroy(() => {
		unsubscribe();
	});
</script>

<div class="ticker-filings-container">
	<div class="header-section">
		<h2>{currentTicker ? `${currentTicker} SEC Filings` : 'SEC Filings'}</h2>
		<button class="refresh-button" on:click={fetchFilings} disabled={isLoading}> Refresh </button>
	</div>

	{#if message}
		<div class="message">{message}</div>
	{/if}

	{#if isLoading}
		<div class="loading-container">
			<div class="loading-spinner"></div>
			<p>Loading filings...</p>
		</div>
	{:else if $filings.length > 0}
		<div class="filings-list">
			{#each $filings as filing}
				<div
					class="filing-item"
					role="button"
					tabindex="0"
					on:click={() => openFiling(filing.url)}
					on:keydown={(e) => (e.key === 'Enter' || e.key === ' ' ? openFiling(filing.url) : null)}
				>
					<div class="filing-header">
						<span class="filing-type">{filing.type}</span>
						<span class="filing-date">{formatDate(filing.timestamp)}</span>
					</div>
					<div class="filing-actions">
						<a href={filing.url} target="_blank" rel="noopener noreferrer" class="filing-link">
							View Filing
						</a>
					</div>
				</div>
			{/each}
		</div>
	{:else if !isLoading && currentTicker}
		<div class="no-data">No SEC filings found for {currentTicker}.</div>
	{:else}
		<div class="no-data">Select a ticker to view SEC filings.</div>
	{/if}
</div>

<style>
	.ticker-filings-container {
		display: flex;
		flex-direction: column;
		height: 100%;
		padding: 16px;
		background-color: #1a1a1a;
		color: #f0f0f0;
		overflow: hidden;
	}

	.header-section {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
		flex-shrink: 0;
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

	.message {
		margin: 10px 0;
		color: #ddd;
		flex-shrink: 0;
	}

	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 40px;
		color: #aaa;
		flex: 1;
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

	.filings-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		overflow-y: auto;
		flex: 1;
		min-height: 0; /* Critical for flex container scrolling */
	}

	.filing-item {
		background-color: #222;
		border: 1px solid #444;
		border-radius: 4px;
		padding: 12px;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.filing-item:hover {
		background-color: #333;
	}

	.filing-header {
		display: flex;
		justify-content: space-between;
		margin-bottom: 8px;
	}

	.filing-type {
		font-weight: bold;
		color: #4caf50;
	}

	.filing-date {
		color: #aaa;
		font-size: 0.9em;
	}

	.filing-actions {
		display: flex;
		justify-content: flex-end;
	}

	.filing-link {
		color: #2196f3;
		text-decoration: none;
	}

	.filing-link:hover {
		text-decoration: underline;
	}

	.no-data {
		text-align: center;
		color: #aaa;
		margin-top: 20px;
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
	}
</style>
