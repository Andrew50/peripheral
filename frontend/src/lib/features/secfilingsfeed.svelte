<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { subscribeSECFilings, unsubscribeSECFilings } from '$lib/utils/stream/socket';
	import { addGlobalSECFilingsStream } from '$lib/utils/stream/interface';
	import { globalFilings, formatTimestamp, handleSECFilingMessage } from '$lib/utils/stream/secfilings';
	import { queryChart } from '$lib/features/chart/interface';

	// Local component state
	let isLoadingGlobalFilings = true;
	let globalFilingsMessage = 'Loading SEC filings...';

	// Function to handle clicking on a filing
	function handleFilingClick(filing) {
		// Find the security by ticker and load its chart
		if (filing.ticker) {
			queryChart({ ticker: filing.ticker });
		}
	}

	function refreshFilings() {
		isLoadingGlobalFilings = true;
		globalFilingsMessage = 'Refreshing SEC filings...';
		privateRequest('getLatestEdgarFilings', {})
			.then(filings => {
				console.log('Received filings from API:', filings);
				handleSECFilingMessage(filings); // Use the same handler for consistency
				isLoadingGlobalFilings = false;
				globalFilingsMessage = filings.length > 0 ? 
					`Loaded ${filings.length} recent SEC filings` : 
					'No recent SEC filings found';
			})
			.catch(error => {
				console.error('Failed to refresh SEC filings:', error);
				globalFilingsMessage = `Error refreshing SEC filings: ${error}`;
				isLoadingGlobalFilings = false;
			});
	}

	// Function to handle WebSocket messages for SEC filings
	function handleSocketMessage(data) {
		console.log("SEC Filing message received via socket:", data);
		handleSECFilingMessage(data);
		isLoadingGlobalFilings = false;
		
		// Update message based on filings count
		const filingsCount = $globalFilings.length;
		globalFilingsMessage = filingsCount > 0 ? 
			`Loaded ${filingsCount} recent SEC filings` : 
			'No recent SEC filings found';
	}

	onMount(async () => {
		try {
			// Subscribe to real-time updates
			const unsubscribe = addGlobalSECFilingsStream(handleSocketMessage);
			
			return () => {
				unsubscribe();
			};
		} catch (error) {
			console.error('Failed to subscribe to SEC filings:', error);
			globalFilingsMessage = `Error loading SEC filings: ${error}`;
			isLoadingGlobalFilings = false;
		}
	});

	// Clean up subscription on component destroy
	onDestroy(() => {
		unsubscribeSECFilings();
	});
</script>

<div class="sec-filings-feed">
	<div class="header-section">
		<h2>Latest SEC Filings</h2>
		<button 
			class="refresh-button" 
			on:click={refreshFilings}
			disabled={isLoadingGlobalFilings}
		>
			Refresh
		</button>
	</div>

	{#if isLoadingGlobalFilings}
		<div class="loading-container">
			<div class="loading-spinner"></div>
			<p>{globalFilingsMessage}</p>
		</div>
	{:else}
		<p class="message">{globalFilingsMessage}</p>
		
		{#if $globalFilings.length > 0}
			<div class="filings-list">
				{#each $globalFilings as filing}
					<div class="filing-item" on:click={() => handleFilingClick(filing)}>
						<div class="filing-header">
							<span class="filing-type">{filing.type || 'Unknown'}</span>
							<span class="filing-company">{filing.company_name || 'Unknown Company'}</span>
							<span class="filing-ticker">{filing.ticker || 'Unknown'}</span>
						</div>
						<div class="filing-date">
							Filed: {filing.date || 'Unknown date'} 
							{#if filing.timestamp}
								({formatTimestamp(filing.timestamp)})
							{/if}
						</div>
						<div class="filing-actions">
							<a href={filing.url} target="_blank" rel="noopener noreferrer" class="filing-link">
								View Filing
							</a>
						</div>
					</div>
				{/each}
			</div>
		{:else}
			<p class="no-data">No SEC filings found.</p>
		{/if}
	{/if}
</div>

<style>
	.sec-filings-feed {
		display: flex;
		flex-direction: column;
		height: 100%;
		padding: 16px;
		background-color: #1a1a1a;
		color: #f0f0f0;
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

	.message {
		margin: 10px 0;
		color: #ddd;
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

	.filings-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
		overflow-y: auto;
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

	.filing-company {
		flex-grow: 1;
		margin: 0 10px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.filing-ticker {
		color: #2196f3;
	}

	.filing-date {
		color: #aaa;
		font-size: 0.9em;
		margin-bottom: 8px;
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
	}
</style> 