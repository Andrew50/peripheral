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
	import type { Trade } from '$lib/core/types';

	type StreamCellType = 'price' | 'change' | 'change %' | 'change % extended' | 'market cap';

	interface SimilarTrade {
		entry_time: number;
		ticker: string;
		direction: string;
		pnl: number;
		similarity_score: number;
	}

	interface ExtendedInstance extends Instance {
		trades?: Trade[];
		[key: string]: any; // Allow dynamic property access
	}

	interface ApiResponse {
		status: string;
		similar_trades?: SimilarTrade[];
		message?: string;
	}

	let longPressTimer: ReturnType<typeof setTimeout>;
	export let list: Writable<ExtendedInstance[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: ExtendedInstance) => {};
	export let formatters: { [key: string]: (value: any) => string } = {};
	export let expandable = false;
	export let expandedContent: (item: ExtendedInstance) => any = () => null;
	export let displayNames: { [key: string]: string } = {};

	let selectedRowIndex = -1;
	console.log('list', get(list));
	let expandedRows = new Set<number>();

	// Add these for similar trades handling
	let similarTradesMap = new Map<number, SimilarTrade[]>();
	let loadingMap = new Map<number, boolean>();
	let errorMap = new Map<number, string>();

	let isLoading = true;
	let loadError: string | null = null;

	function isFlagged(instance: ExtendedInstance, flagWatch: ExtendedInstance[]) {
		if (!Array.isArray(flagWatch)) return false;
		return flagWatch.some((item) => item.ticker === instance.ticker);
	}

	function deleteRow(event: MouseEvent, watch: ExtendedInstance) {
		event.stopPropagation();
		event.preventDefault();
		list.update((v: ExtendedInstance[]) => {
			return v.filter((s) => s !== watch);
		});
		parentDelete(watch);
	}

	function createListAlert() {
		const currentList = get(list);
		if (selectedRowIndex < 0 || selectedRowIndex >= currentList.length) return;

		const selectedItem = currentList[selectedRowIndex];
		const alert = {
			alertType: 'price',
			price: selectedItem.price,
			securityId: selectedItem.securityId,
			ticker: selectedItem.ticker
		};
		newAlert(alert);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'ArrowUp' || (event.key === ' ' && event.shiftKey)) {
			event.preventDefault();
			moveUp();
		} else if (event.key === 'ArrowDown' || event.key === ' ') {
			event.preventDefault();
			moveDown();
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
			const currentList = get(list);
			if (index >= 0 && index < currentList.length) {
				queryChart(currentList[index]);
			}
		}
	}

	onMount(() => {
		try {
			isLoading = true;
			window.addEventListener('keydown', handleKeydown);
			const preventContextMenu = (e: Event) => {
				e.preventDefault();
			};

			window.addEventListener('contextmenu', preventContextMenu);

			// Load icons if needed
			if (columns?.includes('Ticker') && $list?.length > 0) {
				loadIcons();
			}

			return () => {
				window.removeEventListener('contextmenu', preventContextMenu);
			};
		} catch (error) {
			loadError = error instanceof Error ? error.message : 'An unknown error occurred';
			console.error('Failed to load data:', error);
		} finally {
			isLoading = false;
		}
	});

	async function loadIcons() {
		try {
			const tickers = $list.map((item) => item.ticker).filter(Boolean);
			if (tickers.length === 0) return;

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
		}
	}

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown);
	});

	function clickHandler(
		event: MouseEvent,
		instance: ExtendedInstance,
		index: number,
		force: number | null = null
	) {
		console.log('selected instance: ', instance);
		const button = force !== null ? force : event.button;

		event.preventDefault();
		event.stopPropagation();

		if (button === 0) {
			selectedRowIndex = index;
			queryChart(instance);
		} else if (button === 1) {
			flagSecurity(instance);
		}
	}

	function handleTouchStart(
		event: TouchEvent & { currentTarget: EventTarget & HTMLTableRowElement },
		watch: ExtendedInstance,
		i: number
	) {
		event.preventDefault(); // Prevent default touch behavior
		const target = event.currentTarget as HTMLElement;
		longPressTimer = setTimeout(() => {
			clickHandler(new MouseEvent('mousedown'), watch, i, 2);
		}, 600);
	}

	function handleTouchEnd() {
		clearTimeout(longPressTimer);
	}

	function toggleRow(index: number) {
		if (expandedRows.has(index)) {
			expandedRows.delete(index);
		} else {
			expandedRows.add(index);
			const content = expandedContent($list[index]);
			if (content?.tradeId) {
				loadSimilarTrades(content.tradeId);
			}
		}
		expandedRows = expandedRows;
	}

	function formatValue(value: ExtendedInstance, column: string): string {
		const normalizedCol = column
			.replace(/ /g, '')
			.replace(/^[A-Z]/, (letter) => letter.toLowerCase());

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

		const rawValue = value[dataKey];

		if (formatters[column]) {
			return formatters[column](rawValue);
		}
		return rawValue?.toString() ?? 'N/A';
	}

	function getAllOrders(trade: ExtendedInstance): Trade[] {
		return trade.trades || [];
	}

	async function loadSimilarTrades(tradeId: number) {
		if (!tradeId) return;

		loadingMap.set(tradeId, true);
		errorMap.delete(tradeId);
		similarTradesMap = similarTradesMap;

		try {
			const result = await queueRequest<ApiResponse>('find_similar_trades', { trade_id: tradeId });

			if (result && typeof result === 'object' && 'status' in result) {
				if (result.status === 'success' && result.similar_trades) {
					similarTradesMap.set(tradeId, result.similar_trades);
				} else if (result.message) {
					errorMap.set(tradeId, result.message);
				}
			}
		} catch (e) {
			const errorMessage = e instanceof Error ? e.message : 'An unknown error occurred';
			console.error('Error loading similar trades:', errorMessage);
			errorMap.set(tradeId, `Error loading similar trades: ${errorMessage}`);
		} finally {
			loadingMap.delete(tradeId);
			similarTradesMap = similarTradesMap;
		}
	}

	// Watch for list changes to load icons
	$: if (columns?.includes('Ticker') && $list?.length > 0) {
		loadIcons();
	}

	// Watch for expanded rows changes
	$: if (expandedRows) {
		expandedRows.forEach((index) => {
			if ($list[index]) {
				const content = expandedContent($list[index]);
				if (content?.tradeId) {
					loadSimilarTrades(content.tradeId);
				}
			}
		});
	}

	function handleImageError(e: Event) {
		const img = e.currentTarget as HTMLImageElement;
		if (img) {
			img.style.display = 'none';
		}
	}

	function getStreamCellType(col: string): StreamCellType {
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
				return 'price'; // Fallback to price
		}
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
							on:touchstart={(event) => handleTouchStart(event, watch, i)}
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
												on:error={handleImageError}
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
											type={getStreamCellType(col)}
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

										{#if expandedContent}
											{@const content = expandedContent(watch)}
											{@const tradeId = content?.tradeId}
											{#if tradeId}
												<h4>Similar Trades</h4>
												{#if loadingMap.get(tradeId)}
													<div class="loading">Loading similar trades...</div>
												{:else if errorMap.get(tradeId)}
													<div class="error">{errorMap.get(tradeId)}</div>
												{:else if similarTradesMap.get(tradeId)}
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
															{#each similarTradesMap.get(tradeId) || [] as similarTrade}
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
