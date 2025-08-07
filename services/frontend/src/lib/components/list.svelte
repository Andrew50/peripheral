<script lang="ts" context="module">
	import type { Instance, Trade } from '$lib/utils/types/types';
	export interface ExtendedInstance extends Instance {
		trades?: Trade[];
		[key: string]: any;
	}
	export interface ActionButton {
		label: string;
		action: (item: ExtendedInstance) => void;
		class?: string;
	}
</script>

<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable, get } from 'svelte/store';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import type { Writable } from 'svelte/store';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import { flagWatchlist } from '$lib/utils/stores/stores';
	import { flagSecurity } from '$lib/utils/stores/flag';
	import { newAlert } from '$lib/features/alerts/interface';
	import { queueRequest, privateRequest } from '$lib/utils/helpers/backend';
	import { fade } from 'svelte/transition';

	type StreamCellType = 'price' | 'change' | 'change %' | 'change % extended' | 'market cap';

	// Response shape from getIcons API
	interface IconResponse {
		ticker: string;
		icon?: string;
	}

	let longPressTimer: ReturnType<typeof setTimeout>;
	export let list: Writable<ExtendedInstance[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: ExtendedInstance) => {};
	export let formatters: { [key: string]: (value: any) => string } = {};
	export let expandable = false;
	export let expandedContent: (item: ExtendedInstance) => any = () => null;
	export let displayNames: { [key: string]: string } = {};
	export let rowClass: (item: ExtendedInstance) => string = () => '';
	export const actionButtons: ActionButton[] = [];
	// export let linkColumns: LinkColumn[] = []; // Commented out - unused export
	export const defaultSortColumn: string | null = null;

	// Add sorting state variables
	let sortColumn: string | null = null;
	let sortDirection: 'asc' | 'desc' = 'asc';
	let isSorting = false;
	let selectedRowIndex = -1;
	let expandedRows = new Set<number>();

	let isLoading = true;
	let loadError: string | null = null;
	// Add a flag to track icon loading state
	let iconsLoadedForTickers = new Set<string>();
	let iconCache = new Map<string, string>();
	let isLoadingIcons = false;

	// Replace all instances of the current base64 placeholder with a pure black pixel
	const BLACK_PIXEL =
		'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=';

	function formatColumnHeader(columnName: string): string {
		if (!columnName) return ''; // Handle empty column names if they occur
		return columnName
			.replace(/_/g, ' ') // Replace underscores with spaces
			.split(' ') // Split into words
			.map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase()) // Capitalize each word
			.join(' '); // Join back with spaces
	}

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
		// Ignore keydown events if the target is an input, textarea, or select element
		const targetElement = event.target as HTMLElement;
		if (
			targetElement &&
			(targetElement.tagName === 'INPUT' ||
				targetElement.tagName === 'TEXTAREA' ||
				targetElement.tagName === 'SELECT')
		) {
			return; // Do nothing if the event originated from an input-like element
		}

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

	// Add function to handle sorting when a column header is clicked
	// NOTE: This function uses the *original* column key (`column`), not the formatted one.
	function handleSort(column: string) {
		// Prevent sorting on the empty columns
		if (!column) return;

		if (sortColumn === column) {
			// Toggle direction if already sorting by this column
			sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
		} else {
			// Set new sort column and default to ascending
			sortColumn = column;
			sortDirection = 'asc';
		}

		// Apply the sorting with visual feedback
		isSorting = true;
		setTimeout(() => {
			sortList();
			// Give some time for the animation to be visible
			setTimeout(() => {
				isSorting = false;
			}, 300);
		}, 50);
	}

	// Function to get data key from column name
	// NOTE: This function expects the *original* column key (`column`), not the formatted one.
	function getDataKey(column: string): string {
		// This function should continue to work with the original column names
		// For example, if a column is 'change_percent', it should use 'change_percent' here.
		// The formatting change is only for the display in the header.
		// However, your original code already normalized keys like 'Chg %' -> 'change%'
		// so we keep that logic if it was intended. Let's stick to the original normalization logic.

		const normalizedCol = column
			.replace(/ /g, '')
			.replace(/^[A-Z]/, (letter) => letter.toLowerCase());

		switch (normalizedCol) {
			case 'chg':
				return 'change';
			case 'chg%':
				return 'change%';
			case 'ext':
				return 'change%extended';
			// Add other specific key mappings if necessary based on your *original* column names
			default:
				// Return the original column name if no specific mapping exists
				// Or keep the existing normalization if that's correct for your data keys
				return column.includes('_') ? column : normalizedCol; // Prioritize original if underscore exists
		}
	}

	// Function to sort the list based on current sort column and direction
	// NOTE: This function relies on the *original* `sortColumn` value.
	function sortList() {
		if (!sortColumn) return;
		const currentSortColumn = sortColumn;

		list.update((items: ExtendedInstance[]) => {
			const sorted = [...items].sort((a, b) => {
				// Handle special column cases first based on the *original* column name directly
				// Keep these checks using the original column keys passed to handleSort
				if (currentSortColumn === 'Price') {
					const priceA = typeof a.price === 'number' ? a.price : 0;
					const priceB = typeof b.price === 'number' ? b.price : 0;
					return sortDirection === 'asc' ? priceA - priceB : priceB - priceA;
				}
				if (currentSortColumn === 'Chg') {
					const changeA = typeof a.change === 'number' ? a.change : 0;
					const changeB = typeof b.change === 'number' ? b.change : 0;
					return sortDirection === 'asc' ? changeA - changeB : changeB - changeA;
				}
				if (currentSortColumn === 'Chg%') {
					const pctA = typeof a['change%'] === 'number' ? a['change%'] : 0;
					const pctB = typeof b['change%'] === 'number' ? b['change%'] : 0;
					return sortDirection === 'asc' ? pctA - pctB : pctB - pctA;
				}
				if (currentSortColumn === 'Ext') {
					const extA = typeof a['change%extended'] === 'number' ? a['change%extended'] : 0;
					const extB = typeof b['change%extended'] === 'number' ? b['change%extended'] : 0;
					return sortDirection === 'asc' ? extA - extB : extB - extA;
				}

				// For other columns, use the *original* sortColumn as the data key directly
				// Or use getDataKey if that transformation is required for your data structure
				const dataKey = currentSortColumn; // Assuming direct key access works for underscore columns like 'market_cap'
				// const dataKey = getDataKey(sortColumn); // Use this if you need the transformation from getDataKey

				let valueA = a[dataKey];
				let valueB = b[dataKey];

				// Keep timestamp handling as is
				if (dataKey === 'timestamp' || dataKey === 'Timestamp') {
					// Check both cases just in case
					const timeA = typeof valueA === 'number' ? valueA : 0;
					const timeB = typeof valueB === 'number' ? valueB : 0;
					return sortDirection === 'asc' ? timeA - timeB : timeB - timeA;
				}
				// Handle trade duration
				if (dataKey === 'trade_duration' || dataKey === 'Trade Duration') {
					// Use original or potentially display name
					const durationA = typeof a.tradeDurationMillis === 'number' ? a.tradeDurationMillis : -1;
					const durationB = typeof b.tradeDurationMillis === 'number' ? b.tradeDurationMillis : -1;
					return sortDirection === 'asc' ? durationA - durationB : durationB - durationA;
				}

				// Generic number handling
				if (typeof valueA === 'number' && typeof valueB === 'number') {
					return sortDirection === 'asc' ? valueA - valueB : valueB - valueA;
				}

				// For strings or other types, convert to string and compare
				const strA = String(valueA ?? '').toLowerCase();
				const strB = String(valueB ?? '').toLowerCase();

				return sortDirection === 'asc' ? strA.localeCompare(strB) : strB.localeCompare(strA);
			});

			return sorted;
		});
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
			// Use original column name 'Ticker' if that's the key
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
		if (isLoadingIcons) return;
		isLoadingIcons = true;

		// Gather all tickers in the current list
		const tickers = [...new Set($list.map((item) => item?.ticker).filter(Boolean))];
		if (tickers.length === 0) {
			isLoadingIcons = false;
			return;
		}

		// Figure out which tickers we haven't cached yet
		const toFetch = tickers.filter((t) => t && !iconCache.has(t));
		if (toFetch.length > 0) {
			try {
				// Fetch icon data for tickers not yet cached
				const resp = await privateRequest<IconResponse[]>('getIcons', { tickers: toFetch });
				if (Array.isArray(resp)) {
					resp.forEach((ir: IconResponse) => {
						if (ir.ticker) {
							const raw = ir.icon || '';
							const url = raw.startsWith('data:')
								? raw
								: raw.startsWith('/9j/')
									? `data:image/jpeg;base64,${raw}`
									: `data:image/png;base64,${raw}`;
							iconCache.set(ir.ticker, url);
						}
					});
				}
			} catch (e) {
				console.error('Error fetching icons:', e);
			}
		}

		// Apply any cached icons to all list items
		list.update((items: ExtendedInstance[]) =>
			items.map((item: ExtendedInstance) => {
				const ticker = item.ticker;
				if (typeof ticker !== 'string') return item;
				const url = iconCache.get(ticker);
				return url ? { ...item, icon: url } : item;
			})
		);
		isLoadingIcons = false;
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
		}
		expandedRows = expandedRows;
	}

	// NOTE: This function expects the *original* column key (`column`)
	function formatValue(value: ExtendedInstance, column: string): string {
		let dataKey = column; // Default to using the original column name as the key

		// Apply specific transformations based on the original column name if needed
		switch (column) {
			case 'Chg':
				dataKey = 'change';
				break;
			case 'Chg%':
				dataKey = 'change%';
				break;
			case 'Ext':
				dataKey = 'change%extended';
				break;
			// Add other cases if the column name used in `columns` array
			// differs from the actual key in the `ExtendedInstance` object.
			// Example: if columns has 'Market Cap' but data key is 'market_cap'
			// case 'Market Cap':
			//  dataKey = 'market_cap';
			//  break;
			case 'Timestamp': // Handle potential case difference
				dataKey = 'timestamp';
				break;
			case 'Trade Duration': // Handle potential case difference
				dataKey = 'tradeDurationMillis'; // Use the actual data key
				break;
		}

		const rawValue = value[dataKey];

		// Apply custom formatters using the *original* column name as the key
		if (formatters[column]) {
			return formatters[column](rawValue);
		}

		// Special handling for timestamp and duration if no formatter provided
		if (column === 'Timestamp' && typeof rawValue === 'number') {
			return UTCTimestampToESTString(rawValue);
		}
		if (column === 'Trade Duration' && dataKey === 'tradeDurationMillis') {
			return formatDuration(rawValue); // rawValue here is tradeDurationMillis
		}

		return rawValue?.toString() ?? 'N/A';
	}

	function getAllOrders(trade: ExtendedInstance): Trade[] {
		return trade.trades || [];
	}

	// Whenever the Ticker column is active and there are rows, refresh icons (using cache)
	// Use original column name 'Ticker'
	$: if (columns?.includes('Ticker') && $list?.length > 0) {
		loadIcons();
	}

	// Watch for expanded rows changes
	$: if (expandedRows) {
		expandedRows.forEach((index: number) => {
			if ($list[index]) {
				const content = expandedContent($list[index]);
			}
		});
	}

	function handleImageError(e: Event, ticker: string) {
		const img = e.currentTarget as HTMLImageElement;
		if (img && ticker) {
			// Update the cache so we don't retry loading the bad icon
			iconCache.set(ticker, BLACK_PIXEL);
			// Force the specific item in the list to use the black pixel
			list.update((items: ExtendedInstance[]) =>
				items.map((item: ExtendedInstance) =>
					item.ticker === ticker ? { ...item, icon: BLACK_PIXEL } : item
				)
			);
		}
	}

	// NOTE: This function expects the *original* column key (`col`)
	function getStreamCellType(col: string): StreamCellType {
		// Use original column keys for matching
		switch (col) {
			case 'Price':
				return 'price';
			case 'Chg':
				return 'change';
			case 'Chg%':
				return 'change %';
			case 'Ext':
				return 'change % extended';
			// Add cases for underscore versions if they appear in `columns`
			// case 'market_cap': return 'market cap';
			default:
				// Check if it maps to a known data key used by StreamCell
				if (col === 'price') return 'price';
				if (col === 'change') return 'change';
				if (col === 'change%') return 'change %';
				if (col === 'change%extended') return 'change % extended';
				if (col === 'market_cap') return 'market cap';
				return 'price'; // Sensible fallback
		}
	}

	// Format duration helper
	function formatDuration(millis: number | null | undefined): string {
		if (millis === null || millis === undefined || isNaN(millis) || millis < 0) {
			return 'N/A';
		}
		let seconds = Math.floor(millis / 1000);
		let minutes = Math.floor(seconds / 60);
		let hours = Math.floor(minutes / 60);

		seconds = seconds % 60;
		minutes = minutes % 60;

		let parts: string[] = [];
		if (hours > 0) parts.push(`${hours}h`);
		if (minutes > 0) parts.push(`${minutes}m`);
		if (seconds > 0 || parts.length === 0) parts.push(`${seconds}s`); // Show 0s if duration is less than 1s

		return parts.join(' ');
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
		<table class="default-table" class:sorting={isSorting}>
			<thead>
				<tr class="default-tr">
					{#if expandable}
						<th class="default-th expand-column" />
					{/if}
					<th class="default-th"></th>
					{#each columns as col}
						<th
							class="default-th"
							data-type={col.toLowerCase().replace(/\s+/g, '-')}
							class:sortable={col !== ''}
							class:sorting={sortColumn === col}
							class:sort-asc={sortColumn === col && sortDirection === 'asc'}
							class:sort-desc={sortColumn === col && sortDirection === 'desc'}
							on:click={() => handleSort(col)}
						>
							<div class="th-content">
								<span>{displayNames[col] || formatColumnHeader(col)}</span>
								{#if sortColumn === col}
									<span class="sort-icon">{sortDirection === 'asc' ? '↑' : '↓'}</span>
								{/if}
							</div>
						</th>
					{/each}
					<th class="default-th"></th>
				</tr>
			</thead>
			{#if Array.isArray($list) && $list.length > 0}
				<tbody>
					{#each $list as watch, i (`${watch.watchlistItemId}-${i}`)}
						<tr
							class="default-tr {rowClass(watch)}"
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
									<span class="flag-icon">
										<svg
											xmlns="http://www.w3.org/2000/svg"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											stroke-width="2"
											stroke-linecap="round"
											stroke-linejoin="round"
										>
											<path d="M5 5v14"></path>
											<path d="M19 5l-6 4 6 4-6 4"></path>
										</svg>
									</span>
								{/if}
							</td>
							{#each columns as col}
								{#if col === 'Ticker'}
									<td class="default-td">
										{#if watch.icon && watch.icon !== BLACK_PIXEL}
											<img
												src={watch.icon}
												alt={`${watch.ticker} icon`}
												class="ticker-icon"
												on:error={(e) => handleImageError(e, watch.ticker ?? '')}
											/>
										{:else if watch.ticker}
											<span class="default-ticker-icon">
												{watch.ticker.charAt(0).toUpperCase()}
											</span>
										{/if}
										<span class="ticker-name">{watch.ticker}</span>
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
										}}>{watch.timestamp ? UTCTimestampToESTString(watch.timestamp) : 'N/A'}</td
									>
								{:else if col === 'Trade Duration'}
									<td
										class="default-td"
										on:contextmenu={(event) => {
											event.preventDefault();
											event.stopPropagation();
										}}>{formatDuration(watch.tradeDurationMillis)}</td
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
									on:mousedown|stopPropagation
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
								<td colspan={columns.length + 1 + (expandable ? 1 : 0)}>
									<div class="trade-details">
										{#if typeof expandedContent === 'function'}
											<!-- eslint-disable-next-line svelte/no-at-html-tags -->
											{@html expandedContent(watch)}
										{:else}
											<h4>Trade Details</h4>
											{#if getAllOrders(watch).length > 0}
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
														{#each getAllOrders(watch) as order, orderIndex (orderIndex)}
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
											{:else}
												<p>No trade details available.</p>
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
		position: sticky;
		top: 0;
		z-index: 1;
	}

	/* Sorting styles */
	.sortable {
		cursor: pointer;
		user-select: none;
		position: relative;
	}

	.th-content {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.sort-icon {
		margin-left: 4px;
		font-size: 0.8em;
		opacity: 0.7;
	}

	.sorting {
		background-color: var(--ui-bg-hover);
	}

	table.sorting tbody tr {
		opacity: 0.7;
		transition: opacity 0.3s ease;
	}

	table.sorting {
		position: relative;
	}

	table.sorting::after {
		content: '';
		position: absolute;
		inset: 0;
		background: rgb(0 0 0 / 5%);
		pointer-events: none;
	}

	.sort-asc .sort-icon,
	.sort-desc .sort-icon {
		opacity: 1;
		color: var(--ui-accent);
	}

	th.sortable:hover {
		background-color: var(--ui-bg-hover);
	}

	tr {
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
		margin: 0 0 6px;
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
		background-color: var(--ui-bg-primary);
		vertical-align: middle;
	}

	th:last-child {
		position: sticky;
		right: 8px;
		width: 24px;
		max-width: 24px;
		padding: 0;
		transition: opacity 0.2s ease;
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
		margin: 20px 0 10px;
		color: var(--text-secondary);
	}

	.ticker-icon {
		width: 24px;
		height: 24px;
		border-radius: 50%; /* Make icons circular */
		object-fit: cover; /* Ensure icon covers the area nicely */
		background-color: var(--ui-bg-element); /* BG for unloaded images */
		vertical-align: middle; /* Align with text */
		margin-right: 5px; /* Space between icon and text */
	}

	.default-ticker-icon {
		display: inline-flex; /* Use inline-flex for alignment */
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background-color: var(--ui-border); /* Use border color for background */
		color: var(--text-primary); /* Use primary text color */
		font-size: 12px;
		font-weight: 500;
		user-select: none; /* Prevent text selection */
		vertical-align: middle; /* Align with text */
		margin-right: 5px; /* Space between icon and text */
	}

	.ticker-name {
		flex-grow: 1; /* Allow ticker name to take remaining space */
		overflow: hidden; /* Prevent long names from breaking layout */
		white-space: nowrap;
	}

	/* Style for different trade types */
	.long {
		color: var(--color-positive);
	}

	/* Professional flag icon styling */
	.flag-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}

	.flag-icon svg {
		width: 16px;
		height: 16px;
		color: var(--accent-color);
	}

	/* ---- START DELETE BUTTON / STICKY COLUMN STYLES ---- */

	/* Sticky Last column (Delete Button) */
	th:last-child,
	td:last-child {
		position: sticky;
		right: 0; /* Stick to the very edge */
		z-index: 1; /* Above non-sticky cells */
		background-color: inherit; /* Inherit row/header background */
		width: 30px; /* Minimal width for button */
		max-width: 30px;
		padding: 0;
		text-align: center;
		vertical-align: middle;
	}

	th:last-child {
		z-index: 3; /* Above tbody cells and sort overlay */
		background-color: var(--ui-bg-element); /* Ensure header BG */
	}

	.delete-button {
		opacity: 0;
		transition: opacity 0.2s ease;
		cursor: pointer;
		border: none;
		background: none;
		color: var(--negative);
		font-size: 1.2em;
		padding: 4px;
		line-height: 1;
		display: inline-flex; /* Helps center */
		align-items: center;
		justify-content: center;
	}

	.delete-button:hover {
		color: var(--negative-hover, red); /* Darker red on hover */
	}

	tr:hover .delete-button {
		opacity: 1;
	}

	/* Adjust background for sticky columns on hover/select */

	/* Assuming .selected class is used for row selection */
	tr:hover th:last-child,
	tr:hover td:last-child {
		background-color: var(--ui-bg-hover);
	}
</style>
