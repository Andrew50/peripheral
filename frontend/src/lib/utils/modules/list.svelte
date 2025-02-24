<!-- screen.svelte -->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable, get } from 'svelte/store';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import { flagWatchlist } from '$lib/core/stores';
	import { flagSecurity } from '$lib/utils/flag';
	import { newAlert } from '$lib/features/alerts/interface';
	import { queueRequest, privateRequest } from '$lib/core/backend';
	let longPressTimer: any;
	export let list: Writable<Instance[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: Instance) => {};
	export let formatters: { [key: string]: (value: any) => string } = {};
	export let expandable = false;
	export let expandedContent: (item: any) => any = () => null;
	export let displayNames: { [key: string]: string } = {};

	let selectedRowIndex = -1;
	console.log('list', get(list));
	let expandedRows = new Set();

	// Add these for similar trades handling
	let similarTradesMap = new Map();
	let loadingMap = new Map();
	let errorMap = new Map();

	let isLoading = true;
	let loadError = null;

	function isFlagged(instance: Instance, flagWatch: Instance[]) {
		if (!Array.isArray(flagWatch)) return false;
		return flagWatch.some((item) => item.ticker === instance.ticker);
	}

	function deleteRow(event: MouseEvent, watch: Instance) {
		event.stopPropagation();
		event.preventDefault();
		list.update((v: Instance[]) => {
			return v.filter((s) => s !== watch);
		});
		parentDelete(watch);
	}
	function createListAlert() {
		const alert = {
			price: get(list)[selectedRowIndex].price
		};
		for (let i = 0; i < get(list).length; i++) {
			alert.securityId = get(list)[i].securityId;
			alert.ticker = get(list)[i].ticker;
			newAlert(alert);
		}
	}
	function handleKeydown(event: KeyboardEvent, watch: Instance) {
		if (event.key === 'ArrowUp' || (event.key === ' ' && event.shiftKey)) {
			event.preventDefault();
			moveUp();
		} else if (event.key === 'ArrowDown' || event.key === ' ') {
			event.preventDefault();
			moveDown();
		} else {
			return;
		}
	}
	function moveDown() {
		if (selectedRowIndex < $list.length - 1) {
			selectedRowIndex++;
		} else {
			selectedRowIndex = 0;
		}
		scrollToRow(selectedRowIndex);
	}
	function moveUp() {
		if (selectedRowIndex > 0) {
			selectedRowIndex--;
		} else {
			selectedRowIndex = $list.length - 1;
		}
		scrollToRow(selectedRowIndex);
	}

	function scrollToRow(index: number) {
		const row = document.getElementById(`row-${index}`);
		if (row) {
			row.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
			queryChart(get(list)[selectedRowIndex]);
		}
	}
	onMount(async () => {
		try {
			isLoading = true;
			window.addEventListener('keydown', handleKeydown);
			const preventContextMenu = (event) => {
				event.preventDefault();
			};

			window.addEventListener('contextmenu', preventContextMenu);

			// Updated icon handling - update list items directly
			if (columns.includes('Ticker')) {
				const tickers = get(list).map((item) => item.ticker);
				const iconsResponse = await privateRequest('getIcons', { tickers });
				if (iconsResponse && Array.isArray(iconsResponse)) {
					list.update((items) => {
						return items.map((item) => {
							const iconData = iconsResponse.find((i) => i.ticker === item.ticker);
							if (iconData && iconData.icon) {
								const iconUrl = iconData.icon.startsWith('/9j/')
									? `data:image/jpeg;base64,${iconData.icon}`
									: `data:image/png;base64,${iconData.icon}`;
								return { ...item, icon: iconUrl };
							}
							return item;
						});
					});
				}
			}

			return () => {
				window.removeEventListener('contextmenu', preventContextMenu);
			};
		} catch (error) {
			loadError = error.message;
			console.error('Failed to load data:', error);
		} finally {
			isLoading = false;
		}
	});

	// Add this reactive statement after the onMount block
	$: if (columns?.includes('Ticker') && $list?.length > 0) {
		console.log('List changed, loading icons for', $list.length, 'items');
		const tickers = $list.map((item) => item.ticker).filter(Boolean);
		console.log('Requesting icons for tickers:', tickers);

		privateRequest('getIcons', { tickers }).then((iconsResponse) => {
			console.log('Received icon response:', iconsResponse);

			if (iconsResponse && Array.isArray(iconsResponse)) {
				list.update((items) => {
					console.log('Updating list with icons');
					const updatedItems = items.map((item) => {
						if (!item.ticker) return item;

						const iconData = iconsResponse.find((i) => i.ticker === item.ticker);
						console.log(`Processing icon for ${item.ticker}:`, iconData);

						if (iconData && iconData.icon) {
							const iconUrl = iconData.icon.startsWith('/9j/')
								? `data:image/jpeg;base64,${iconData.icon}`
								: `data:image/png;base64,${iconData.icon}`;
							return { ...item, icon: iconUrl };
						}
						return item;
					});
					console.log('Updated list items:', updatedItems);
					return updatedItems;
				});
			}
		});
	}

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown);
	});
	function clickHandler(
		event: MouseEvent,
		instance: Instance,
		index: number,
		force: number | null = null
	) {
		let even;
		if (force !== null) {
			even = force;
		} else {
			even = event.button;
		}
		event;
		event.preventDefault();
		event.stopPropagation();
		if (even === 0) {
			selectedRowIndex = index;
			if ('openQuantity' in instance) {
				queryChart(instance);
			} else {
				queryChart(instance);
			}
		} else if (even === 1) {
			flagSecurity(instance);
		}
	}
	function handleTouchStart(event, watch, i) {
		longPressTimer = setTimeout(() => {
			clickHandler(event, watch, i, 2); // The action you want to trigger
		}, 600); // Time in milliseconds to consider a long press
	}

	function handleTouchEnd() {
		clearTimeout(longPressTimer); // Clear if it's a short tap
	}

	function toggleRow(index: number) {
		'Toggling row:', index;
		if (expandedRows.has(index)) {
			expandedRows.delete(index);
		} else {
			expandedRows.add(index);
			// Debug log for expanded content
			const content = expandedContent($list[index]);
			'Expanded content:', content;
		}
		expandedRows = expandedRows; // Trigger reactivity
	}

	function formatValue(value: any, column: string): string {
		// Convert column name to camelCase
		const normalizedCol = column
			.replace(/ /g, '') // Remove spaces
			.replace(/^[A-Z]/, (letter) => letter.toLowerCase()); // Decapitalize first letter

		// Get the actual data property name based on the display column name
		let dataKey = normalizedCol;
		switch (normalizedCol) {
			case 'chg':
				dataKey = 'change';
				break;
			case 'chg%':
				dataKey = 'change%';
				break;
			case 'ext':
				dataKey = 'change%extended';
				break;
			default:
				dataKey = normalizedCol;
		}

		// Get value using the normalized data key
		const rawValue = value[dataKey];

		if (formatters[column]) {
			return formatters[column](rawValue);
		}
		return rawValue?.toString() ?? 'N/A';
	}

	function getAllOrders(trade) {
		return trade.trades || [];
	}

	async function loadSimilarTrades(tradeId: number) {
		if (!tradeId) {
			('No tradeId provided');
			return;
		}

		'Loading similar trades for trade:', tradeId;
		loadingMap.set(tradeId, true);
		errorMap.delete(tradeId);
		similarTradesMap = similarTradesMap;

		try {
			'Making request for trade:', tradeId;
			const result = await queueRequest('find_similar_trades', { trade_id: tradeId });
			'Similar trades result:', result;

			if (result.status === 'success') {
				similarTradesMap.set(tradeId, result.similar_trades);
				'Updated similarTradesMap:', similarTradesMap;
			} else {
				errorMap.set(tradeId, result.message);
				'Error from server:', result.message;
			}
		} catch (e) {
			console.error('Error loading similar trades:', e);
			errorMap.set(tradeId, `Error loading similar trades: ${e}`);
		} finally {
			loadingMap.delete(tradeId);
			similarTradesMap = similarTradesMap;
		}
	}

	// Modified reactive statement with more logging
	$: {
		if (expandedRows) {
			'Expanded rows changed:', expandedRows;
			expandedRows.forEach((isExpanded, index) => {
				`Checking row ${index}, expanded: ${isExpanded}`;
				if (isExpanded && $list[index]) {
					const content = expandedContent($list[index]);
					`Content for row ${index}:`, content;
					if (content?.tradeId) {
						`Loading similar trades for row ${index}, tradeId: ${content.tradeId}`;
						loadSimilarTrades(content.tradeId);
					}
				}
			});
		}
	}

	$: if (columns?.includes('Ticker') && $list?.length > 0) {
		(async () => {
			try {
				isLoading = true;
				const tickers = $list.map((item) => item.ticker).filter(Boolean);

				if (tickers.length === 0) {
					isLoading = false;
					return;
				}

				const iconsResponse = await privateRequest('getIcons', { tickers });
				if (iconsResponse && Array.isArray(iconsResponse)) {
					list.update((items) => {
						return items.map((item) => {
							if (!item.ticker) return item;

							const iconData = iconsResponse.find((i) => i.ticker === item.ticker);
							if (iconData && iconData.icon) {
								const iconUrl = iconData.icon.startsWith('/9j/')
									? `data:image/jpeg;base64,${iconData.icon}`
									: `data:image/png;base64,${iconData.icon}`;
								return { ...item, icon: iconUrl };
							}
							return item;
						});
					});
				}
			} catch (error) {
				console.error('Failed to load icons:', error);
				// Don't set error state here as it's not critical functionality
			} finally {
				isLoading = false;
			}
		})();
	}
</script>

<div class="table-container">
	{#if isLoading}
		<div class="loading">Loading...</div>
	{:else if loadError}
		<div class="error">
			<p>Failed to load data: {loadError}</p>
			<button on:click={() => window.location.reload()}>Retry</button>
		</div>
	{:else}
		<table class="default-table">
			<thead>
				<tr class="default-tr">
					{#if expandable}
						<th class="default-th expand-column" />
					{/if}
					<th class="default-th"></th>
					{#each columns as col}
						<th class="default-th" data-type={col.toLowerCase().replace(/\s+/g, '-')}>
							{displayNames[col] || col}
						</th>
					{/each}
					<th class="default-th"></th>
				</tr>
			</thead>
			{#if Array.isArray($list) && $list.length > 0}
				<tbody>
					{#each $list as watch, i}
						<tr
							class="default-tr"
							on:mousedown={(event) => clickHandler(event, watch, i)}
							on:touchstart={handleTouchStart}
							on:touchend={handleTouchEnd}
							id="row-{i}"
							class:selected={i === selectedRowIndex}
							on:contextmenu={(event) => {
								event.preventDefault();
							}}
							class:expandable
							class:expanded={expandedRows.has(i)}
							on:click={() => expandable && toggleRow(i)}
						>
							{#if expandable}
								<td class="default-td expand-cell">
									<span class="expand-icon">{expandedRows.has(i) ? '−' : '+'}</span>
								</td>
							{/if}
							<td class="default-td">
								{#if isFlagged(watch, $flagWatchlist)}
									<span class="flag-icon">⚑</span>
								{/if}
							</td>
							{#each columns as col}
								{#if col === 'Ticker'}
									<td class="default-td">
										{#if watch.icon}
											<img
												src={watch.icon}
												alt={`${watch.ticker} icon`}
												class="ticker-icon"
												on:error={(e) => {
													console.error(`Failed to load icon for ${watch.ticker}:`, e);
													e.currentTarget.style.display = 'none';
												}}
											/>
										{/if}
										{watch.ticker}
									</td>
								{:else if ['Price', 'Chg', 'Chg%', 'Ext'].includes(col)}
									<td class="default-td">
										<StreamCell
											on:contextmenu={(event) => {
												event.preventDefault();
												event.stopPropagation();
											}}
											instance={watch}
											type={(() => {
												switch (col) {
													case 'Price':
														return 'price';
													case 'Chg':
														return 'change';
													case 'Chg%':
														return 'change %';
													case 'Ext':
														return 'change % extended';
													default:
														return col.toLowerCase();
												}
											})()}
										/>
									</td>
								{:else if col === 'Timestamp'}
									<td
										class="default-td"
										on:contextmenu={(event) => {
											event.preventDefault();
											event.stopPropagation();
										}}>{UTCTimestampToESTString(watch[col.toLowerCase()])}</td
									>
								{:else}
									<td
										class="default-td"
										on:contextmenu={(event) => {
											event.preventDefault();
											event.stopPropagation();
										}}>{formatValue(watch, col)}</td
									>
								{/if}
							{/each}
							<td class="default-td">
								<button
									class="delete-button"
									on:click={(event) => {
										deleteRow(event, watch);
									}}
								>
									✕
								</button>
							</td>
						</tr>
						{#if expandable && expandedRows.has(i)}
							<tr class="expanded-content">
								<td colspan={columns.length + (expandable ? 2 : 1)}>
									<div class="trade-details">
										<h4>Trade Details</h4>
										<table>
											<thead>
												<tr class="defalt-tr">
													<th class="defalt-th">Time</th>
													<th class="defalt-th">Type</th>
													<th class="defalt-th">Price</th>
													<th class="defalt-th">Shares</th>
												</tr>
											</thead>
											<tbody>
												{#each getAllOrders(watch) as order}
													<tr class="defalt-tr">
														<td class="defalt-td">{UTCTimestampToESTString(order.time)}</td>
														<td class={order.type.toLowerCase().replace(/\s+/g, '-')}
															>{order.type}</td
														>
														<td class="defalt-td">{order.price}</td>
														<td class="defalt-td">{order.shares}</td>
													</tr>
												{/each}
											</tbody>
										</table>

										<!-- Add Similar Trades section -->
										{#if expandedContent}
											{@const content = expandedContent($list[i])}
											{@const tradeId = content.tradeId}
											{#if tradeId}
												<h4>Similar Trades</h4>
												{#if loadingMap.get(tradeId)}
													<div class="loading">Loading similar trades...</div>
												{:else if errorMap.get(tradeId)}
													<div class="error">{errorMap.get(tradeId)}</div>
												{:else if similarTradesMap.get(tradeId)?.length}
													<table>
														<thead>
															<tr class="defalt-tr">
																<th class="defalt-th">Date</th>
																<th class="defalt-th">Ticker</th>
																<th class="defalt-th">Direction</th>
																<th class="defalt-th">P/L</th>
																<th class="defalt-th">Similarity</th>
															</tr>
														</thead>
														<tbody>
															{#each similarTradesMap.get(tradeId) as similarTrade}
																<tr class="defalt-tr">
																	<td class="defalt-td">
																		{UTCTimestampToESTString(similarTrade.entry_time)}
																	</td>
																	<td class="defalt-td">{similarTrade.ticker}</td>
																	<td class="defalt-td">{similarTrade.direction}</td>
																	<td class={similarTrade.pnl >= 0 ? 'positive' : 'negative'}>
																		${similarTrade.pnl.toFixed(2)}
																	</td>
																	<td class="defalt-td">
																		{(similarTrade.similarity_score * 100).toFixed(1)}%
																	</td>
																</tr>
															{/each}
														</tbody>
													</table>
												{:else}
													<div class="no-results">No similar trades found</div>
												{/if}
											{/if}
										{/if}
									</div>
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			{/if}
		</table>
	{/if}
</div>

<style>
	.selected {
		outline: 2px solid var(--ui-accent);
		outline-offset: -2px;
	}

	tr {
		transition: outline 0.2s ease;
	}

	.list-container {
		width: 100%;
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		margin: 0;
		padding: 0;
		color: var(--text-primary);
		background: var(--ui-bg-primary);
		table-layout: fixed;
	}

	th,
	td {
		padding: 8px;
		text-align: left;
		border-bottom: 1px solid var(--ui-border);
	}

	th {
		background-color: var(--ui-bg-element);
		font-weight: bold;
		color: var(--text-secondary);
	}

	tr {
		background-color: var(--ui-bg-primary);
		transition: background-color 0.2s;
	}

	tr:hover {
		background-color: var(--ui-bg-hover);
	}

	.expandable {
		cursor: pointer;
	}

	.expand-cell {
		width: 30px;
		text-align: center;
		padding: 4px;
	}

	.expand-icon {
		color: var(--text-secondary);
	}

	.expanded-content {
		background-color: var(--ui-bg-element);
	}

	.expanded-content td {
		padding: 8px;
	}

	.trade-details {
		background-color: var(--ui-bg-element);
		padding: 8px;
		border-radius: 4px;
	}

	.trade-details h4 {
		margin: 0 0 6px 0;
		color: var(--text-secondary);
		font-size: 0.9em;
	}

	.trade-details table {
		width: 100%;
		font-size: 0.85em;
	}

	.trade-details th {
		background-color: var(--ui-bg-element);
		padding: 6px 8px;
	}

	.trade-details tr {
		background-color: transparent;
	}

	.trade-details tr:hover {
		background-color: var(--ui-bg-hover);
	}

	.entry,
	.buy {
		color: var(--positive);
	}

	.exit,
	.sell {
		color: var(--negative);
	}

	.short {
		color: var(--negative);
	}

	.buy-to-cover {
		color: var(--positive);
	}

	.table-container {
		width: 100%;
		overflow: hidden;
		max-width: 100%;
		padding-bottom: 2px;
		padding-right: 8px;
	}

	td:last-child {
		position: sticky;
		right: 8px;
		width: 24px;
		max-width: 24px;
		padding: 0;
		text-align: center;
	}

	th:last-child {
		position: sticky;
		right: 8px;
		width: 24px;
		max-width: 24px;
		padding: 0;
		background-color: var(--ui-bg-element);
	}

	.delete-button {
		opacity: 0;
		transition: opacity 0.2s ease;
	}

	tr:hover .delete-button {
		opacity: 1;
	}

	tr:hover td {
		background-color: var(--ui-bg-hover);
	}

	tr:hover td:last-child {
		background-color: var(--ui-bg-hover);
	}

	.loading,
	.error,
	.no-results {
		padding: 10px;
		text-align: center;
		color: var(--text-secondary);
	}

	.error {
		color: var(--negative);
	}

	.positive {
		color: var(--positive);
	}

	.negative {
		color: var(--negative);
	}

	h4 {
		margin: 20px 0 10px 0;
		color: var(--text-secondary);
	}

	.ticker-icon {
		width: 20px;
		height: 20px;
		margin-right: 5px;
		vertical-align: middle;
	}

	.loading,
	.error {
		text-align: center;
		padding: 2rem;
		color: var(--text-primary);
	}

	.error {
		color: var(--c5);
	}
</style>
