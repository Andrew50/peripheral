<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { subscribeSECFilings, unsubscribeSECFilings } from '$lib/utils/stream/socket';
	import { addGlobalSECFilingsStream } from '$lib/utils/stream/interface';
	import { globalFilings, formatTimestamp, type Filing } from '$lib/utils/stream/secfilings';
	import { queryChart } from '$lib/features/chart/interface';

	// Local component state
	let isLoadingGlobalFilings = true;
	let globalFilingsMessage = 'Loading SEC filings...';

	// Function to handle clicking on a filing
	function handleFilingClick(filing: Filing) {
		// Find the security by ticker and load its chart
		if (filing.ticker) {
			queryChart({ ticker: filing.ticker });
		}
	}

	function refreshFilings() {
		isLoadingGlobalFilings = true;
		globalFilingsMessage = 'Refreshing SEC filings...';
		privateRequest<Filing[]>('getLatestEdgarFilings', {})
			.then(filings => {
				globalFilings.set(filings);
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

	onMount(async () => {
		try {
			// Load initial data
			const initialFilings = await privateRequest<Filing[]>('getLatestEdgarFilings', {});
			globalFilings.set(initialFilings);
			isLoadingGlobalFilings = false;
			console.log(initialFilings);
			globalFilingsMessage = initialFilings.length > 0 ? 
				`Loaded ${initialFilings.length} recent SEC filings` : 
				'No recent SEC filings found';
			
			// Subscribe to real-time updates
			subscribeSECFilings();
		} catch (error) {
			console.error('Failed to load initial SEC filings:', error);
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
							<span class="filing-type">{filing.type}</span>
							<span class="filing-ticker">{filing.ticker || 'Unknown'}</span>
							<span class="filing-date">{formatTimestamp(filing.timestamp)}</span>
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
		padding: 20px;
		color: white;
		height: 100%;
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

	.filing-ticker {
		color: #2196f3;
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
	}
</style> 