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
	import { fade,  } from 'svelte/transition';

	type StreamCellType = 'price' | 'change' | 'change %' | 'change % extended' | 'market cap';

	interface ExtendedInstance extends Instance {
		trades?: Trade[];
		[key: string]: any; // Allow dynamic property access
	}

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

	// Add sorting state variables
	let sortColumn: string | null = null;
	let sortDirection: 'asc' | 'desc' = 'asc';
	let isSorting = false;
	export let linkColumns: string[] = [];
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

	// Add function to handle sorting when a column header is clicked
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
	function getDataKey(column: string): string {
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
			default:
				return normalizedCol;
		}
	}

	// Function to sort the list based on current sort column and direction
	function sortList() {
		if (!sortColumn) return;

		list.update((items: ExtendedInstance[]) => {
			const sorted = [...items].sort((a, b) => {
				// Handle special column cases first based on column name directly
				if (sortColumn === 'Price') {
					const priceA = typeof a.price === 'number' ? a.price : 0;
					const priceB = typeof b.price === 'number' ? b.price : 0;
					return sortDirection === 'asc' ? priceA - priceB : priceB - priceA;
				}

				if (sortColumn === 'Chg') {
					const changeA = typeof a.change === 'number' ? a.change : 0;
					const changeB = typeof b.change === 'number' ? b.change : 0;
					return sortDirection === 'asc' ? changeA - changeB : changeB - changeA;
				}

				if (sortColumn === 'Chg%') {
					const pctA = typeof a['change%'] === 'number' ? a['change%'] : 0;
					const pctB = typeof b['change%'] === 'number' ? b['change%'] : 0;
					return sortDirection === 'asc' ? pctA - pctB : pctB - pctA;
				}

				if (sortColumn === 'Ext') {
					const extA = typeof a['change%extended'] === 'number' ? a['change%extended'] : 0;
					const extB = typeof b['change%extended'] === 'number' ? b['change%extended'] : 0;
					return sortDirection === 'asc' ? extA - extB : extB - extA;
				}

				// For other columns, use data key
				const dataKey = getDataKey(sortColumn!);

				// Get the values to compare
				let valueA = a[dataKey];
				let valueB = b[dataKey];

				// Handle timestamps
				if (dataKey === 'timestamp') {
					const timeA = typeof valueA === 'number' ? valueA : 0;
					const timeB = typeof valueB === 'number' ? valueB : 0;
					return sortDirection === 'asc' ? timeA - timeB : timeB - timeA;
				}

				// Generic number handling
				if (typeof valueA === 'number' && typeof valueB === 'number') {
					return sortDirection === 'asc' ? valueA - valueB : valueB - valueA;
				}

				// For strings or other types, convert to string and compare
				const strA = String(valueA || '').toLowerCase();
				const strB = String(valueB || '').toLowerCase();

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

	// Whenever the Ticker column is active and there are rows, refresh icons (using cache)
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
			// This ensures the {#if} condition evaluates correctly
			list.update((items: ExtendedInstance[]) =>
				items.map((item: ExtendedInstance) => (item.ticker === ticker ? { ...item, icon: BLACK_PIXEL } : item))
			);
			// Optionally, set the current img src directly for immediate visual feedback
			// although the list update above should handle it reactively.
			// img.src = BLACK_PIXEL; 
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

	// Fix for linter error: Define formatDuration
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
								<span>{displayNames[col] || col}</span>
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
					{#each $list as watch, i (`${watch.securityId || i}-${i}`)}
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
							transition:fade={{ duration: 150 }}
						>
							{#if expandable}
								<td class="default-td expand-cell">
									<span class="expand-icon">{expandedRows.has(i) ? '−' : '+'}</span>
								</td>
							{/if}
							<td class="default-td">
								{#if isFlagged(watch, $flagWatchlist)}
									<span class="flag-icon">
										<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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
												on:error={(e) => handleImageError(e, watch.ticker)}
											/>
										{:else if watch.ticker}
											<span class="default-ticker-icon">
												{watch.ticker.charAt(0).toUpperCase()}
											</span>
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
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.05);
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
		margin: 20px 0 10px 0;
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
</style>
