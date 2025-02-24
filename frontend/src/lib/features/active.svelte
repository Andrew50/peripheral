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
		constituents?: { ticker: string; securityId: number }[];
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
	}

	const params = writable<Params>({
		timeframe: '1 day',
		group: 'stock',
		metric: 'price leader'
	});

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
						price: 0,
						active: true
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

	onMount(() => {
		if (!browser) return; // Only run in browser context

		const unsubscribe = params.subscribe((p: Params) => {
			isLoading.set(true);
			loadError.set(null);

			privateRequest<ActiveResult[]>('getActive', p, true)
				.then((results: ActiveResult[]) => {
					if (!results || !Array.isArray(results)) {
						throw new Error('Invalid response format');
					}

					// Filter out any results without securityId
					const validResults = results.filter(
						(r) => r.securityId || (r.constituents && r.constituents.some((c) => c.securityId))
					);

					if (validResults.length === 0) {
						console.warn('No valid results found');
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
		return items
			.map((item): Instance | null => {
				// Ensure we have a valid securityId and ticker
				if (!item.securityId) {
					console.warn('Missing securityId for ticker:', item.ticker);
					return null;
				}

				if (!item.ticker) {
					console.warn('Missing ticker for securityId:', item.securityId);
					return null;
				}

				return {
					ticker: String(item.ticker).trim(), // Ensure ticker is a valid string
					securityId: item.securityId,
					// Set timestamp to 0 to let the stream system handle it
					timestamp: 0,
					price: 0,
					active: true
				};
			})
			.filter(Boolean) as Instance[]; // Remove any null items
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
		<List list={constituentsList} columns={['Ticker', 'Price', 'Chg', 'Chg%']} />
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
		</div>

		<div class="results">
			{#if currentParams.group === 'stock'}
				<List
					list={writable(convertToInstances($list))}
					columns={['Ticker', 'Price', 'Chg', 'Chg%']}
				/>
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
										<List list={constituentsList} columns={['Ticker', 'Price', 'Chg', 'Chg%']} />
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

	.retry-button:hover {
		background: var(--ui-bg-hover);
	}
</style>
