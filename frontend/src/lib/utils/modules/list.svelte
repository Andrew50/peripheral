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
	import { queueRequest } from '$lib/core/backend';
	let longPressTimer: any;
	export let list: Writable<Instance[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: Instance) => {};
	export let formatters: { [key: string]: (value: any) => string } = {};
	export let expandable = false;
	export let expandedContent: (item: any) => any = () => null;
	export let displayNames: {[key: string]: string} = {};

	let selectedRowIndex = -1;
	let expandedRows = new Set();

	// Add these for similar trades handling
	let similarTradesMap = new Map();
	let loadingMap = new Map();
	let errorMap = new Map();

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
	onMount(() => {
		window.addEventListener('keydown', handleKeydown);
		const preventContextMenu = (event) => {
			event.preventDefault();
		};

		window.addEventListener('contextmenu', preventContextMenu);

		return () => {
			window.removeEventListener('contextmenu', preventContextMenu);
		};
	});
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
		console.log(event);
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
		console.log("Toggling row:", index);
		if (expandedRows.has(index)) {
			expandedRows.delete(index);
		} else {
			expandedRows.add(index);
			// Debug log for expanded content
			const content = expandedContent($list[index]);
			console.log("Expanded content:", content);
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
			console.log("No tradeId provided");
			return;
		}
		
		console.log("Loading similar trades for trade:", tradeId);
		loadingMap.set(tradeId, true);
		errorMap.delete(tradeId);
		similarTradesMap = similarTradesMap;
		
		try {
			console.log("Making request for trade:", tradeId);
			const result = await queueRequest('find_similar_trades', { trade_id: tradeId });
			console.log("Similar trades result:", result);
			
			if (result.status === 'success') {
				similarTradesMap.set(tradeId, result.similar_trades);
				console.log("Updated similarTradesMap:", similarTradesMap);
			} else {
				errorMap.set(tradeId, result.message);
				console.log("Error from server:", result.message);
			}
		} catch (e) {
			console.error("Error loading similar trades:", e);
			errorMap.set(tradeId, `Error loading similar trades: ${e}`);
		} finally {
			loadingMap.delete(tradeId);
			similarTradesMap = similarTradesMap;
		}
	}

	// Modified reactive statement with more logging
	$: {
		if (expandedRows) {
			console.log("Expanded rows changed:", expandedRows);
			expandedRows.forEach((isExpanded, index) => {
				console.log(`Checking row ${index}, expanded: ${isExpanded}`);
				if (isExpanded && $list[index]) {
					const content = expandedContent($list[index]);
					console.log(`Content for row ${index}:`, content);
					if (content?.tradeId) {
						console.log(`Loading similar trades for row ${index}, tradeId: ${content.tradeId}`);
						loadSimilarTrades(content.tradeId);
					}
				}
			});
		}
	}
</script>

<div class="table-container">
	<table class="default-table">
		<thead>
			<tr class="default-tr">
				{#if expandable}
					<th class="default-th expand-column" />
				{/if}
				<th class="default-th"></th>
				{#each columns as col}
					<th class="default-th">{displayNames[col] || col}</th>
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
							{#if ['price', 'Chg', 'Chg%', 'Ext'].includes(col)}
								<td class="default-td">
									<StreamCell
										on:contextmenu={(event) => {
											event.preventDefault();
											event.stopPropagation();
										}}
										instance={watch}
										type={col.toLowerCase().replace(/ /g, '')}
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
													<td class={order.type.toLowerCase().replace(/\s+/g, '-')}>{order.type}</td
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

	.loading, .error, .no-results {
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
</style>
