<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { writable } from 'svelte/store';
	import List from '$lib/utils/modules/list.svelte';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { activeChartInstance } from '$lib/core/stores';
	import { addGlobalSECFilingsStream } from '$lib/utils/stream/interface';

	// Add tab state
	let activeTab = 'filings';

	// Store for SEC filings
	let filings = writable<Filing[]>([]);
	let isLoadingFilings = false;
	let message = '';

	// Store for global SEC filings
	let globalFilings = writable<GlobalFiling[]>([]);
	let isLoadingGlobalFilings = false;
	let globalFilingsMessage = '';

	// Store for news articles (for future implementation)
	let news = writable<NewsItem[]>([]);
	let isLoadingNews = false;

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

	// Interface for global SEC filings
	interface GlobalFiling {
		type: string;
		date: string;
		url: string;
		timestamp: number;
		ticker: string;
		channel: string;
	}

	// Interface for news items (for future implementation)
	interface NewsItem {
		title: string;
		source: string;
		summary: string;
		url: string;
		timestamp: number;
	}

	// Subscribe to active chart instance to get current ticker
	activeChartInstance.subscribe((chartInstance) => {
		if (chartInstance?.ticker && chartInstance?.securityId) {
			currentTicker = chartInstance.ticker;
			currentSecurityId = chartInstance.securityId;
			
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
		message = 'Loading SEC filings...';
		
		try {
			// Get current time for "to" parameter
			const now = Date.now();
			
			// Get filings from 2 years ago to now
			const twoYearsAgo = now - (2 * 365 * 24 * 60 * 60 * 1000);
			
			const result = await privateRequest<Filing[]>('getEdgarFilings', {
				securityId: currentSecurityId,
				from: twoYearsAgo,
				to: now,
				limit: 100
			});
			
			filings.set(result);
			message = result.length > 0 ? 
				`Loaded ${result.length} SEC filings for ${currentTicker}` : 
				`No SEC filings found for ${currentTicker}`;
		} catch (error) {
			console.error('Failed to load SEC filings:', error);
			message = `Error loading SEC filings: ${error}`;
			filings.set([]);
		} finally {
			isLoadingFilings = false;
		}
	}

	// Function to handle incoming global SEC filing messages
	function handleGlobalSECFilingMessage(message: any) {
		console.log("SEC Filing message received:", message);
		
		// Check if the message has a data property that is an array
		if (message.data && Array.isArray(message.data)) {
			console.log("Initial SEC filings data:", message.data);
			globalFilings.set(message.data);
			isLoadingGlobalFilings = false;
			globalFilingsMessage = message.data.length > 0 ? 
				`Loaded ${message.data.length} recent SEC filings` : 
				'No recent SEC filings found';
		} else if (message.data) {
			// Handle single filing update
			console.log("New SEC filing:", message.data);
			globalFilings.update(currentFilings => {
				// Add the new filing at the beginning of the array
				const updatedFilings = [message.data, ...currentFilings];
				// Keep only the most recent 100 filings
				if (updatedFilings.length > 100) {
					return updatedFilings.slice(0, 100);
				}
				console.log("Updated SEC filings:", updatedFilings);
				return updatedFilings;
			});
			globalFilingsMessage = `New SEC filing: ${message.data.type} for ${message.data.ticker || message.data.company_name}`;
		} else {
			console.error("Received unexpected message format:", message);
			globalFilingsMessage = "Received data in unexpected format";
		}
	}

	// Function to subscribe to global SEC filings
	function subscribeToGlobalFilings() {
		isLoadingGlobalFilings = true;
		globalFilingsMessage = 'Loading global SEC filings...';
		
		// Use the addGlobalSECFilingsStream function to subscribe
		unsubscribeGlobalFilings = addGlobalSECFilingsStream(handleGlobalSECFilingMessage);
	}

	// Function to load news (placeholder for future implementation)
	async function loadNews() {
		if (!currentTicker) return;
		
		isLoadingNews = true;
		message = 'Loading news...';
		
		try {
			// This would be replaced with an actual API call in the future
			// For now, just set an empty array
			news.set([]);
			message = 'News feed will be implemented in a future update';
		} catch (error) {
			console.error('Failed to load news:', error);
			message = `Error loading news: ${error}`;
			news.set([]);
		} finally {
			isLoadingNews = false;
		}
	}

	// Load appropriate data when tab changes
	function handleTabChange(tab: string) {
		activeTab = tab;
		
		if (tab === 'filings') {
			loadFilings();
		} else if (tab === 'news') {
			loadNews();
		} else if (tab === 'global-filings') {
			subscribeToGlobalFilings();
		}
	}

	// Variable to hold the unsubscribe function
	let unsubscribeGlobalFilings: Function | null = null;

	// Initial load
	onMount(() => {
		if (currentSecurityId && activeTab === 'filings') {
			loadFilings();
		}
	});

	// Clean up on destroy
	onDestroy(() => {
		// Unsubscribe from the global SEC filings channel
		if (unsubscribeGlobalFilings) {
			unsubscribeGlobalFilings();
		}
	});
</script>

<div class="newsfeed-container">
	<!-- Tab Navigation -->
	<div class="tab-navigation">
		<button class:active={activeTab === 'filings'} on:click={() => handleTabChange('filings')}>
			SEC Filings
		</button>
		<button class:active={activeTab === 'global-filings'} on:click={() => handleTabChange('global-filings')}>
			Global Filings
		</button>
		<button class:active={activeTab === 'news'} on:click={() => handleTabChange('news')}>
			News
		</button>
		<button class:active={activeTab === 'social'} on:click={() => handleTabChange('social')}>
			Social
		</button>
		<button class:active={activeTab === 'blogs'} on:click={() => handleTabChange('blogs')}>
			Blogs
		</button>
	</div>

	<!-- SEC Filings Tab -->
	{#if activeTab === 'filings'}
		<div class="tab-content">
			<div class="header-section">
				<h2>SEC Filings for {currentTicker || 'Current Ticker'}</h2>
				<button class="refresh-button" on:click={loadFilings} disabled={isLoadingFilings}>
					{isLoadingFilings ? 'Loading...' : 'Refresh'}
				</button>
			</div>

			{#if message}
				<p class="message">{message}</p>
			{/if}

			{#if isLoadingFilings}
				<div class="loading-container">
					<div class="loading-spinner"></div>
					<span>Loading SEC filings...</span>
				</div>
			{:else}
				<List
					list={filings}
					columns={['type', 'timestamp', 'url']}
					displayNames={{
						type: 'Filing Type',
						timestamp: 'Date',
						url: 'Link'
					}}
					formatters={{
						timestamp: (value) => UTCTimestampToESTString(value),
						url: (value) => 'View Filing'
					}}
					linkColumns={{
						url: (item) => item.url
					}}
				/>
			{/if}
		</div>
	{/if}

	<!-- Global SEC Filings Tab -->
	{#if activeTab === 'global-filings'}
		<div class="tab-content">
			<div class="header-section">
				<h2>Global SEC Filings Feed</h2>
				<button class="refresh-button" on:click={subscribeToGlobalFilings} disabled={isLoadingGlobalFilings}>
					{isLoadingGlobalFilings ? 'Loading...' : 'Refresh'}
				</button>
			</div>

			{#if globalFilingsMessage}
				<p class="message">{globalFilingsMessage}</p>
			{/if}

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
						type: 'Filing Type',
						timestamp: 'Date',
						url: 'Link'
					}}
					formatters={{
						timestamp: (value) => UTCTimestampToESTString(value),
						url: (value) => 'View Filing'
					}}
					linkColumns={{
						url: (item) => item.url
					}}
				/>
			{/if}
		</div>
	{/if}

	<!-- News Tab (placeholder) -->
	{#if activeTab === 'news'}
		<div class="tab-content">
			<h2>News Feed</h2>
			<p class="message">News feed will be implemented in a future update.</p>
		</div>
	{/if}

	<!-- Social Tab (placeholder) -->
	{#if activeTab === 'social'}
		<div class="tab-content">
			<h2>Social Media Feed</h2>
			<p class="message">Social media feed will be implemented in a future update.</p>
		</div>
	{/if}

	<!-- Blogs Tab (placeholder) -->
	{#if activeTab === 'blogs'}
		<div class="tab-content">
			<h2>Blog Posts</h2>
			<p class="message">Blog feed will be implemented in a future update.</p>
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
</style>