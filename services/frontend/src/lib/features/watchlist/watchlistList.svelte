<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable, get } from 'svelte/store';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';
	import { queryChart } from '$lib/features/chart/interface';
	import { flagWatchlist } from '$lib/utils/stores/stores';
	import { flagSecurity } from '$lib/utils/stores/flag';
	import { newAlert } from '$lib/features/alerts/interface';
	import { queueRequest, privateRequest } from '$lib/utils/helpers/backend';
	import StreamCellV2 from '$lib/utils/stream/streamCellV2.svelte';
	import { getColumnStore } from '$lib/utils/stream/streamHub';

	type StreamCellType = 'price' | 'change' | 'change %' | 'change % extended' | 'market cap';

	// Define WatchlistItem to match what's used in watchlist.svelte
	interface WatchlistItem extends Instance {
		watchlistItemId?: number;
		change?: number;
		'change%'?: number;
		'change%extended'?: number;
		[key: string]: any; // Allow dynamic property access for sorting
	}

	// Response shape from getIcons API
	interface IconResponse {
		ticker: string;
		icon?: string;
	}

	let longPressTimer: ReturnType<typeof setTimeout>;
	export let list: Writable<WatchlistItem[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: WatchlistItem) => {};
	export let displayNames: { [key: string]: string } = {};
	export let rowClass: (item: WatchlistItem) => string = () => '';
	export const defaultSortColumn: string | null = null;

	// Add sorting state variables
	let sortColumn: string | null = null;
	let sortDirection: 'asc' | 'desc' = 'asc';
	let isSorting = false;
	let selectedRowIndex = -1;

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
			.map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase()) // Capitalize each word
			.join(' '); // Join back with spaces
	}

	function isFlagged(instance: WatchlistItem, flagWatch: WatchlistItem[]) {
		if (!Array.isArray(flagWatch)) return false;
		return flagWatch.some((item) => item.ticker === instance.ticker);
	}

	function deleteRow(event: MouseEvent, watch: WatchlistItem) {
		event.stopPropagation();
		event.preventDefault();
		list.update((v: WatchlistItem[]) => {
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
	// Function to sort the list based on current sort column and direction
	// NOTE: This function relies on the *original* `sortColumn` value.
	function sortList() {
		if (!sortColumn) return;

		list.update((items: WatchlistItem[]) => {
			const sorted = [...items].sort((a, b) => {
				// Helper function to get value from StreamHub store
				const getStreamValue = (item: WatchlistItem, columnType: string) => {
					if (!item.securityId) return 0;
					const store = getColumnStore(Number(item.securityId), columnType as any);
					const storeValue = get(store);
					return storeValue;
				};

				// Handle special column cases first based on the *original* column name directly
				// Keep these checks using the original column keys passed to handleSort
				if (sortColumn === 'Price') {
					const priceDataA = getStreamValue(a, 'price');
					const priceDataB = getStreamValue(b, 'price');
					const priceA = typeof priceDataA?.price === 'number' ? priceDataA.price : 0;
					const priceB = typeof priceDataB?.price === 'number' ? priceDataB.price : 0;
					return sortDirection === 'asc' ? priceA - priceB : priceB - priceA;
				}
				if (sortColumn === 'Chg') {
					const changeDataA = getStreamValue(a, 'change');
					const changeDataB = getStreamValue(b, 'change');
					const changeA = typeof changeDataA?.change === 'number' ? changeDataA.change : 0;
					const changeB = typeof changeDataB?.change === 'number' ? changeDataB.change : 0;
					return sortDirection === 'asc' ? changeA - changeB : changeB - changeA;
				}
				if (sortColumn === 'Chg%') {
					const pctDataA = getStreamValue(a, 'changePct');
					const pctDataB = getStreamValue(b, 'changePct');
					const pctA = typeof pctDataA?.pct === 'number' ? pctDataA.pct : 0;
					const pctB = typeof pctDataB?.pct === 'number' ? pctDataB.pct : 0;
					return sortDirection === 'asc' ? pctA - pctB : pctB - pctA;
				}
				if (sortColumn === 'Ext') {
					const extDataA = getStreamValue(a, 'chgExt');
					const extDataB = getStreamValue(b, 'chgExt');
					const extA = typeof extDataA?.chgExt === 'number' ? extDataA.chgExt : 0;
					const extB = typeof extDataB?.chgExt === 'number' ? extDataB.chgExt : 0;
					return sortDirection === 'asc' ? extA - extB : extB - extA;
				}

				// For other columns, handle ticker specially then fall back to generic logic
				if (sortColumn === 'Ticker') {
					const tickerA = String(a.ticker ?? '').toLowerCase();
					const tickerB = String(b.ticker ?? '').toLowerCase();
					return sortDirection === 'asc' ? tickerA.localeCompare(tickerB) : tickerB.localeCompare(tickerA);
				}

				// For other columns, use the *original* sortColumn as the data key directly
				if (!sortColumn) return 0;
				const dataKey = sortColumn;
				let valueA = a[dataKey];
				let valueB = b[dataKey];

				// If both values are numbers, sort numerically
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
		list.update((items: WatchlistItem[]) =>
			items.map((item: WatchlistItem) => {
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
		instance: WatchlistItem,
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
		watch: WatchlistItem,
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




	// Whenever the Ticker column is active and there are rows, refresh icons (using cache)
	// Use original column name 'Ticker'
	$: if (columns?.includes('Ticker') && $list?.length > 0) {
		loadIcons();
	}

	function handleImageError(e: Event, ticker: string) {
		const img = e.currentTarget as HTMLImageElement;
		if (img && ticker) {
			// Update the cache so we don't retry loading the bad icon
			iconCache.set(ticker, BLACK_PIXEL);
					// Force the specific item in the list to use the black pixel
		list.update((items: WatchlistItem[]) =>
			items.map((item: WatchlistItem) => (item.ticker === ticker ? { ...item, icon: BLACK_PIXEL } : item))
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
					<th class="default-th"></th> {#each columns as col} 
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
						>
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
										<StreamCellV2
											on:contextmenu={(event) => {
												event.preventDefault();
												event.stopPropagation();
											}}
											instance={watch}
											type={getStreamCellType(col)} 
										/>
									</td>
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

	/* ---- START DELETE BUTTON / STICKY COLUMN STYLES ---- */
	/* Sticky Last column (Delete Button) */
	th:last-child, td:last-child {
		position: sticky;
		right: 0px; /* Stick to the very edge */
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
    tr:hover th:last-child, tr:hover td:last-child {
        background-color: var(--ui-bg-hover);
    }
</style>