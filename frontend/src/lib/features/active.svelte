<script lang="ts">
	import { onMount } from 'svelte';
	import type { Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import { writable } from 'svelte/store';
	import List from '$lib/utils/modules/list.svelte';
	import { browser } from '$app/environment';

	interface ActiveResult {
		ticker?: string;
		securityId?: number;
		group?: string;
		market_cap?: number;
		dollar_volume?: number;
		constituents?: { 
			ticker: string; 
			securityId: number; 
			market_cap: number;
			dollar_volume: number;
		}[];
	}

	const list = writable<ActiveResult[]>([]);
	const constituentsList = writable<Instance[]>([]);
	let showConstituents = false;
	let selectedGroupName = '';
	let isLoading = writable(true);
	let loadError = writable<string | null>(null);
	type Timeframe = '1 day' | '1 week' | '1 month' | '6 month' | '1 year';
	type Group = 'stock' | 'sector' | 'industry';
	type Metric =
		| 'price leader'
		| 'price laggard'
		| 'volume leader'
		| 'volume laggard'
		| 'gap leader'
		| 'gap laggard';
	interface Params {
		timeframe: Timeframe;
		group: Group;
		metric: Metric;
		minMarketCap?: number;
		maxMarketCap?: number;
		minDollarVolume?: number;
		maxDollarVolume?: number;
	}

	const params = writable<Params>({
		timeframe: '1 day',
		group: 'stock',
		metric: 'price leader'
	});

	// Filter inputs
	let minMarketCap: string = '';
	let maxMarketCap: string = '';
	let minDollarVolume: string = '';
	let maxDollarVolume: string = '';
	let showFilters: boolean = false;

	let selectedRowIndex: number | null = null;

	function handleRowClick(item: ActiveResult) {
		if (!item) return;

		if (item.constituents) {
			// For group items (sectors/industries)
			selectedGroupName = item.group || '';
			const constituents = item.constituents
				.filter((c) => c.securityId && c.ticker) // Only include items with valid securityId AND ticker
				.map(
					(c): Instance => ({
						ticker: String(c.ticker).trim(), // Ensure ticker is a valid string
						securityId: c.securityId,
						timestamp: 0, // Set timestamp to 0
						price: 0,  // Initialize price to 0
						active: true,
						market_cap: c.market_cap,
						dollar_volume: c.dollar_volume
					})
				);
			constituentsList.set(constituents);

			// Toggle the selected row
			const index = $list.findIndex((r) => r.group === item.group);
			selectedRowIndex = selectedRowIndex === index ? null : index;
		}
	}

	function goBack() {
		showConstituents = false;
		selectedGroupName = '';
	}

	function toggleFilters() {
		showFilters = !showFilters;
	}

	function applyFilters() {
		// Parse filter values
		const requestParams: Params = {
			...currentParams
		};

		if (minMarketCap) {
			requestParams.minMarketCap = parseFloat(minMarketCap) * 1000000; // Convert from millions to actual value
		}
		if (maxMarketCap) {
			requestParams.maxMarketCap = parseFloat(maxMarketCap) * 1000000; // Convert from millions to actual value
		}
		if (minDollarVolume) {
			requestParams.minDollarVolume = parseFloat(minDollarVolume) * 1000000; // Convert from millions to actual value
		}
		if (maxDollarVolume) {
			requestParams.maxDollarVolume = parseFloat(maxDollarVolume) * 1000000; // Convert from millions to actual value
		}

		params.set(requestParams);
	}

	function clearFilters() {
		minMarketCap = '';
		maxMarketCap = '';
		minDollarVolume = '';
		maxDollarVolume = '';
		
		// Remove filter parameters
		const requestParams: Params = {
			timeframe: currentParams.timeframe,
			group: currentParams.group,
			metric: currentParams.metric
		};
		
		params.set(requestParams);
	}

	onMount(() => {
		if (!browser) return; // Only run in browser context

		const unsubscribe = params.subscribe((p: Params) => {
			isLoading.set(true);
			loadError.set(null);

			// Create a serializable object from the params
			const requestParams: any = {
				timeframe: p.timeframe,
				group: p.group,
				metric: p.metric
			};

			// Add filter parameters if they exist
			if (p.minMarketCap !== undefined) {
				requestParams.minMarketCap = p.minMarketCap;
			}
			if (p.maxMarketCap !== undefined) {
				requestParams.maxMarketCap = p.maxMarketCap;
			}
			if (p.minDollarVolume !== undefined) {
				requestParams.minDollarVolume = p.minDollarVolume;
			}
			if (p.maxDollarVolume !== undefined) {
				requestParams.maxDollarVolume = p.maxDollarVolume;
			}

			privateRequest<ActiveResult[]>('getActive', requestParams, true)
				.then((results: ActiveResult[]) => {
					if (!results || !Array.isArray(results)) {
						throw new Error('Invalid response format');
					}

					console.log('[Active] Raw response from getActive:', results);

					// Filter out any results without securityId
					const validResults = results.filter(
						(r) => r.securityId || (r.constituents && r.constituents.some((c) => c.securityId))
					);

					if (validResults.length === 0) {
						console.warn('No valid results found');
					} else {
						console.log('[Active] Valid results with securityIds:', 
							validResults.map(r => r.securityId ? 
								`${r.ticker}: ${r.securityId}` : 
								`${r.group}: ${r.constituents?.map(c => `${c.ticker}: ${c.securityId}`).join(', ')}`
							)
						);
					}

					list.set(validResults);
					// Reset constituents view when params change
					showConstituents = false;
					selectedGroupName = '';
					selectedRowIndex = null;
				})
				.catch((error) => {
					console.error('Error fetching active data:', error);
					loadError.set(error.message || 'Failed to load data');
					list.set([]);
				})
				.finally(() => {
					isLoading.set(false);
				});
		});

		return () => {
			unsubscribe();
		};
	});

	let currentParams: Params;
	params.subscribe((value) => {
		currentParams = value;
		// Force timeframe to '1 day' for gap metrics
		if (
			(value.metric === 'gap leader' || value.metric === 'gap laggard') &&
			value.timeframe !== '1 day'
		) {
			currentParams.timeframe = '1 day';
			params.set(currentParams);
		}
	});

	// Add this function to convert ActiveResult to Instance
	function convertToInstances(items: ActiveResult[]): Instance[] {
		console.log('[Active] Converting items to instances:', items);
		
		const instances = items
			.filter(item => item.ticker && item.securityId) // Only include items with securityId AND ticker
			.map((item): Instance => {
				return {
					ticker: String(item.ticker).trim(), // Ensure ticker is a valid string
					securityId: item.securityId,
					timestamp: 0, // Set timestamp to 0
					price: 0, // Initialize price to 0
					active: true,
					market_cap: item.market_cap,
					dollar_volume: item.dollar_volume
				};
			});
			
		console.log('[Active] Converted instances:', instances);
		return instances;
	}

	// Format market cap and dollar volume for display
	function formatCurrency(value: number | undefined): string {
		if (value === undefined || value === null) return 'N/A';
		
		// Format based on size
		if (value >= 1e12) {
			return `$${(value / 1e12).toFixed(2)}T`;
		} else if (value >= 1e9) {
			return `$${(value / 1e9).toFixed(2)}B`;
		} else if (value >= 1e6) {
			return `$${(value / 1e6).toFixed(2)}M`;
		} else if (value >= 1e3) {
			return `$${(value / 1e3).toFixed(2)}K`;
		}
		return `$${value.toFixed(2)}`;
	}
</script>

<div class="market-container">
	{#if $isLoading}
		<div class="loading">Loading...</div>
	{:else if $loadError}
		<div class="error">
			<p>{$loadError}</p>
			<button class="retry-button" on:click={() => params.set($params)}>Retry</button>
		</div>
	{:else if showConstituents}
		<div class="header">
			<button class="utility-button" on:click={goBack}>←</button>
			<h3>{selectedGroupName} Constituents</h3>
		</div>
		<List 
			list={constituentsList} 
			columns={['Ticker', 'Price', 'Chg', 'Chg%', 'Market Cap', 'Dollar Vol']}
			formatters={{
				'Market Cap': (value) => formatCurrency(value),
				'Dollar Vol': (value) => formatCurrency(value)
			}}
		/>
	{:else}
		<div class="controls">
			<div class="select-group">
				<label for="timeframe">Timeframe</label>
				<select
					class="default-select"
					id="timeframe"
					bind:value={currentParams.timeframe}
					on:change={() => params.set(currentParams)}
				>
					<option value="1 day">1 Day</option>
					<option value="1 week">1 Week</option>
					<option value="1 month">1 Month</option>
					<option value="6 month">6 Months</option>
					<option value="1 year">1 Year</option>
				</select>
			</div>

			<div class="select-group">
				<label for="group">Group</label>
				<select
					class="default-select"
					id="group"
					bind:value={currentParams.group}
					on:change={() => params.set(currentParams)}
				>
					<option value="stock">Stocks</option>
					<option value="sector">Sectors</option>
					<option value="industry">Industries</option>
				</select>
			</div>

			<div class="select-group">
				<label for="metric">Metric</label>
				<select
					class="default-select"
					id="metric"
					bind:value={currentParams.metric}
					on:change={() => params.set(currentParams)}
				>
					<option value="price leader">Price Leaders</option>
					<option value="price laggard">Price Laggards</option>
					<option value="volume leader">Volume Leaders</option>
					<option value="volume laggard">Volume Laggards</option>
					<option value="gap leader">Gap Leaders</option>
					<option value="gap laggard">Gap Laggards</option>
				</select>
			</div>

			<div class="filter-toggle">
				<button class="filter-button" on:click={toggleFilters}>
					{showFilters ? 'Hide Filters' : 'Show Filters'}
				</button>
			</div>
		</div>

		{#if showFilters}
			<div class="filter-container">
				<div class="filter-group">
					<label for="minMarketCap">Min Market Cap ($M)</label>
					<input 
						type="number"
						id="minMarketCap"
						bind:value={minMarketCap}
						placeholder="e.g. 100"
						min="0"
					/>
				</div>
				<div class="filter-group">
					<label for="maxMarketCap">Max Market Cap ($M)</label>
					<input 
						type="number"
						id="maxMarketCap"
						bind:value={maxMarketCap}
						placeholder="e.g. 10000"
						min="0"
					/>
				</div>
				<div class="filter-group">
					<label for="minDollarVolume">Min Dollar Volume ($M)</label>
					<input 
						type="number"
						id="minDollarVolume"
						bind:value={minDollarVolume}
						placeholder="e.g. 1"
						min="0"
					/>
				</div>
				<div class="filter-group">
					<label for="maxDollarVolume">Max Dollar Volume ($M)</label>
					<input 
						type="number"
						id="maxDollarVolume"
						bind:value={maxDollarVolume}
						placeholder="e.g. 100"
						min="0"
					/>
				</div>
				<div class="filter-actions">
					<button class="apply-button" on:click={applyFilters}>Apply Filters</button>
					<button class="clear-button" on:click={clearFilters}>Clear Filters</button>
				</div>
			</div>
		{/if}

		<div class="results">
			{#if currentParams.group === 'stock'}
				{#if $list.length > 0}
					{@const instances = convertToInstances($list)}
					{#if instances.length > 0}
						<List
							list={writable(instances)}
							columns={['Ticker', 'Price', 'Chg', 'Chg%', 'Market Cap', 'Dollar Vol']}
							formatters={{
								'Market Cap': (value) => formatCurrency(value),
								'Dollar Vol': (value) => formatCurrency(value)
							}}
						/>
					{:else}
						<div class="no-data">No valid stocks found with security IDs</div>
					{/if}
				{:else}
					<div class="no-data">No stocks available</div>
				{/if}
			{:else}
				<table>
					<thead>
						<tr class="defalt-tr">
							<th class="defalt-th">{currentParams.group}</th>
						</tr>
					</thead>
					<tbody>
						{#each $list as item, i}
							<tr class="group-row" on:click={() => handleRowClick(item)}>
								<td class="defalt-td">{item.group}</td>
							</tr>
							{#if i === selectedRowIndex}
								<tr class="defalt-tr">
									<td class="defalt-td">
										<List 
											list={constituentsList} 
											columns={['Ticker', 'Price', 'Chg', 'Chg%', 'Market Cap', 'Dollar Vol']}
											formatters={{
												'Market Cap': (value) => formatCurrency(value),
												'Dollar Vol': (value) => formatCurrency(value)
											}}
										/>
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</div>

<style>
	.market-container {
		display: flex;
		flex-direction: column;
		gap: 20px;
		padding: 20px;
		color: white;
	}

	.controls {
		display: flex;
		gap: 20px;
		flex-wrap: wrap;
	}

	.select-group {
		display: flex;
		flex-direction: column;
		gap: 5px;
	}

	.filter-toggle {
		display: flex;
		align-items: flex-end;
	}

	.filter-button, .apply-button, .clear-button {
		background-color: #1a1a1a;
		color: white;
		border: 1px solid #333;
		border-radius: 4px;
		padding: 8px 16px;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.filter-button:hover, .apply-button:hover, .clear-button:hover {
		background-color: #252525;
		border-color: #444;
	}

	.apply-button {
		background-color: #089981;
		border-color: #089981;
	}

	.apply-button:hover {
		background-color: #07806d;
		border-color: #07806d;
	}

	.clear-button {
		background-color: #333;
		border-color: #444;
	}

	.filter-container {
		display: flex;
		flex-wrap: wrap;
		gap: 15px;
		padding: 15px;
		background-color: #1a1a1a;
		border-radius: 4px;
		border: 1px solid #333;
		margin-top: -10px;
	}

	.filter-group {
		display: flex;
		flex-direction: column;
		gap: 5px;
		min-width: 160px;
	}

	.filter-actions {
		display: flex;
		gap: 10px;
		align-items: flex-end;
		margin-left: auto;
	}

	input[type="number"] {
		background-color: #252525;
		border: 1px solid #333;
		color: white;
		border-radius: 4px;
		padding: 8px;
		width: 100%;
	}

	label {
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: #888;
	}

	select {
		background-color: #1a1a1a;
		color: white;
		border: 1px solid #333;
		border-radius: 4px;
		padding: 8px 12px;
		font-size: 14px;
		min-width: 150px;
		cursor: pointer;
		outline: none;
	}

	select:hover {
		border-color: #444;
	}

	select:focus {
		border-color: #666;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		background-color: #1a1a1a;
		border-radius: 4px;
		overflow: hidden;
	}

	th,
	td {
		padding: 12px 16px;
		text-align: left;
		border-bottom: 1px solid #333;
	}

	th {
		background-color: #252525;
		font-weight: 500;
		text-transform: uppercase;
		font-size: 12px;
		letter-spacing: 0.05em;
	}

	tr:hover {
		background-color: #252525;
	}

	.loading {
		text-align: center;
		padding: 20px;
		color: #888;
	}

	@media (max-width: 600px) {
		.controls {
			flex-direction: column;
		}

		select {
			width: 100%;
		}
	}

	.header {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 16px;
	}

	.back-btn {
		background: #1a1a1a;
		border: 1px solid #333;
		color: white;
		padding: 4px 12px;
		border-radius: 4px;
		cursor: pointer;
		font-size: 16px;
	}

	.back-btn:hover {
		background: #252525;
		border-color: #444;
	}

	h3 {
		margin: 0;
		font-size: 16px;
		font-weight: 500;
	}

	.group-row {
		cursor: pointer;
	}

	.group-row:hover {
		background-color: #252525;
	}

	.error {
		color: var(--negative);
		text-align: center;
		padding: 20px;
	}

	.retry-button {
		background: var(--ui-bg-secondary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		padding: 8px 16px;
		margin-top: 10px;
		cursor: pointer;
	}

	.no-data {
		text-align: center;
		padding: 20px;
		color: #888;
	}
</style>
