<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable, get } from 'svelte/store';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';
	import { queryChart, activeChartInstance } from '$lib/features/chart/interface';
	import { flagWatchlist } from '$lib/utils/stores/stores';
	import { flagSecurity } from '$lib/utils/stores/flag';
	import { newAlert } from '$lib/features/alerts/interface';
	import { queueRequest, privateRequest } from '$lib/utils/helpers/backend';
	import StreamCellV2 from '$lib/utils/stream/streamCellV2.svelte';
	import { getColumnStore } from '$lib/utils/stream/streamHub';
	import { isMobileDevice } from '$lib/utils/stores/device';
	import '$lib/styles/glass.css';
	import { switchMobileTab } from '$lib/stores/mobileStore';
	// privateRequest already imported above in this file
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

	export let list: Writable<WatchlistItem[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: WatchlistItem) => {};
	export let parentReorder: (args: {
		movedId: number;
		prevId?: number;
		nextId?: number;
		newIndex: number;
	}) => void = () => {};
	export let currentWatchlistId: number | undefined;
	export let displayNames: { [key: string]: string } = {};
	export let rowClass: (item: WatchlistItem) => string = () => '';
	export const defaultSortColumn: string | null = null;

	// Add sorting state variables
	let sortColumn: string | null = null;
	let sortDirection: 'asc' | 'desc' = 'asc';
	let isSorting = false;
	let selectedRowIndex = -1;
	// Drag state
	let isDragging = false;
	let dragIndex: number = -1;
	let insertionIndex: number = -1;
	let tableBodyEl: HTMLDivElement | null = null;
	let dropLineTop = 0;
	let dragStartY = 0;

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

	// Scroll the watchlist to the row at `index`; optionally trigger a chart query for that row.
	// Pass shouldQueryChart=false when programmatically syncing selection (to avoid redundant queries).
	function scrollToRow(index: number, shouldQueryChart: boolean = true) {
		const row = document.getElementById(`row-${index}`);
		if (row) {
			row.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
			if (shouldQueryChart) {
				const currentList = get(list);
				if (index >= 0 && index < currentList.length) {
					queryChart(currentList[index]);
				}
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
			// Persist order after sorting
			persistOrderAfterSort();
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
				const getStreamValue = (item: WatchlistItem, columnType: string): any => {
					if (!item.securityId) return {} as any;
					const store = getColumnStore(Number(item.securityId), columnType as any);
					const storeValue = get(store) as any;
					return storeValue || ({} as any);
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
					return sortDirection === 'asc'
						? tickerA.localeCompare(tickerB)
						: tickerB.localeCompare(tickerA);
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

	function clickHandler(
		event: MouseEvent,
		instance: WatchlistItem,
		index: number,
		force: number | null = null
	) {
		const button = force !== null ? force : event.button;
		if ($isMobileDevice) {
			switchMobileTab('chart');
		}
		event.preventDefault();
		event.stopPropagation();
		if (button === 0) {
			// Only query chart if clicking on a different row
			if (selectedRowIndex !== index) {
				selectedRowIndex = index;
				queryChart(instance);
			}
		} else if (button === 1) {
			flagSecurity(instance);
		}
	}

	// Utilities for DnD
	function getRowMidpoints(): number[] {
		const mids: number[] = [];
		const rows = document.querySelectorAll<HTMLTableRowElement>('.body-table tbody tr');
		rows.forEach((row) => {
			const rect = row.getBoundingClientRect();
			mids.push((rect.top + rect.bottom) / 2);
		});
		return mids;
	}

	function computeInsertionIndexFromY(y: number): number {
		const mids = getRowMidpoints();
		let idx = -1;
		for (let k = 0; k < mids.length; k++) {
			if (y < mids[k]) {
				idx = k;
				break;
			}
		}
		return idx === -1 ? mids.length : idx;
	}

	function updateDropLineTopForInsertion(index: number) {
		const bodyRect = tableBodyEl?.getBoundingClientRect();
		if (!bodyRect) {
			dropLineTop = 0;
			return;
		}
		const items = get(list) || [];
		if (index <= 0) {
			dropLineTop = 0;
			return;
		}
		if (index >= items.length && items.length > 0) {
			const last = document.getElementById(`row-${items.length - 1}`) as HTMLTableRowElement | null;
			if (last) {
				const lastRect = last.getBoundingClientRect();
				dropLineTop = Math.max(0, lastRect.bottom - bodyRect.top);
			} else {
				dropLineTop = 0;
			}
			return;
		}
		const target = document.getElementById(`row-${index}`) as HTMLTableRowElement | null;
		if (target) {
			const tRect = target.getBoundingClientRect();
			dropLineTop = Math.max(0, tRect.top - bodyRect.top);
		} else {
			dropLineTop = 0;
		}
	}

	function onRowPointerDown(e: PointerEvent, i: number) {
		// Only left button
		if (e.button !== 0) return;
		e.preventDefault();
		e.stopPropagation();
		const y = e.clientY;
		const idx = computeInsertionIndexFromY(y);
		insertionIndex = idx;
		updateDropLineTopForInsertion(idx);
		isDragging = true;
		dragIndex = i;
		dragStartY = e.clientY;
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
		document.body.style.cursor = 'grabbing';
	}

	function onRowPointerMove(e: PointerEvent) {
		if (!isDragging) return;
		const y = e.clientY;
		insertionIndex = computeInsertionIndexFromY(y);

		// Auto-scroll near edges of the scroll container
		if (tableBodyEl) {
			const rect = tableBodyEl.getBoundingClientRect();
			const edge = 30;
			if (y < rect.top + edge) tableBodyEl.scrollTop -= 10;
			else if (y > rect.bottom - edge) tableBodyEl.scrollTop += 10;
		}

		// Compute drop line position relative to tableBodyEl
		updateDropLineTopForInsertion(insertionIndex);
	}

	function onRowPointerUp(_e: PointerEvent, watch: WatchlistItem) {
		if (!isDragging) return;
		const from = dragIndex;
		let to = insertionIndex;
		isDragging = false;
		dragIndex = -1;
		insertionIndex = -1;
		document.body.style.cursor = '';

		if (from === -1 || to === -1) return;
		// Adjust when moving downwards (removal shifts indices)
		if (to > from) to = to - 1;
		if (to === from) return;

		// Perform local reorder
		let movedId = watch.watchlistItemId!;
		list.update((items: WatchlistItem[]) => {
			const a = [...items];
			const [moved] = a.splice(from, 1);
			a.splice(to, 0, moved);
			return a;
		});

		// Identify neighbors for minimal backend update
		const current = get(list);
		const prev = to > 0 ? current[to - 1] : undefined;
		const next = to < current.length - 1 ? current[to + 1] : undefined;
		parentReorder({
			movedId,
			prevId: prev?.watchlistItemId,
			nextId: next?.watchlistItemId,
			newIndex: to
		});
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
				items.map((item: WatchlistItem) =>
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
	function syncWatchlistWithActiveChart(activeTicker: string) {
		if (!activeTicker || !$list?.length) return;

		// Find the ticker in the current watchlist (case-insensitive)
		const matchIndex = $list.findIndex(
			(item) => item.ticker?.toLowerCase() === activeTicker.toLowerCase()
		);

		if (matchIndex !== -1 && matchIndex !== selectedRowIndex) {
			// Update selection and scroll to the matched ticker (without querying chart)
			selectedRowIndex = matchIndex;
			// Use setTimeout to ensure DOM is updated before scrolling
			setTimeout(() => {
				scrollToRow(selectedRowIndex, false);
			}, 0);
		} else if (matchIndex === -1) {
			// No match found, clear selection
			selectedRowIndex = -1;
		}
	}
	onMount(() => {
		try {
			isLoading = true;
			// Use capture so upstream handlers cannot swallow Arrow/Space before we see them
			window.addEventListener('keydown', handleKeydown, true);
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
	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown, true);
	});

	// Add reactive synchronization with activeChartInstance
	$: if ($activeChartInstance?.ticker && $list?.length > 0) {
		syncWatchlistWithActiveChart($activeChartInstance.ticker);
	}

	// Persist order when user sorts by header: debounce and send bulk order
	let persistTimer: ReturnType<typeof setTimeout> | null = null;
	function persistOrderAfterSort() {
		if (persistTimer) clearTimeout(persistTimer);
		persistTimer = setTimeout(() => {
			const ids = (get(list) || []).map((it) => it.watchlistItemId).filter(Boolean) as number[];
			const wlId = currentWatchlistId;
			if (!wlId || ids.length === 0) return;
			// Fire-and-forget; UI already reflects the new order
			privateRequest('setWatchlistOrder', {
				watchlistId: wlId,
				orderedItemIds: ids
			}).catch((e) => {
				console.error('Failed to persist sorted order', e);
			});
		}, 400);
	}
</script>

<div class="table-container" class:mobile={$isMobileDevice}>
	{#if isLoading}
		<div class="loading">Loading...</div>
	{:else if loadError}
		<div class="error">
			<p>Failed to load data: {loadError}</p>
			<button on:click={() => window.location.reload()}>Retry</button>
		</div>
	{:else}
		<!-- Fixed Header -->
		<div class="table-header">
			<table class="header-table" class:sorting={isSorting}>
				<thead>
					<tr class="default-tr">
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
			</table>
		</div>

		<!-- Scrollable Body -->
		<div class="table-body" bind:this={tableBodyEl}>
			{#if isDragging}
				<div class="drop-line" style={`top:${dropLineTop}px`}></div>
			{/if}
			{#if Array.isArray($list) && $list.length > 0}
				<table class="body-table">
					<tbody>
						{#each $list as watch, i (watch.watchlistItemId)}
							<tr
								class="default-tr {rowClass(watch)}"
								on:click={(event) => clickHandler(event, watch, i)}
								on:pointerdown={(e) => onRowPointerDown(e, i)}
								on:pointermove={(e) => onRowPointerMove(e)}
								on:pointerup={(e) => onRowPointerUp(e, watch)}
								id="row-{i}"
								class:selected={i === selectedRowIndex}
								on:contextmenu={(event) => {
									event.preventDefault();
								}}
								on:selectstart={(e) => {
									if (isDragging) {
										e.preventDefault();
										e.stopPropagation();
									}
								}}
							>
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
										on:mousedown={(event) => {
											event.stopPropagation();
											event.preventDefault();
										}}
										title="Remove from watchlist"
									>
										<svg
											xmlns="http://www.w3.org/2000/svg"
											viewBox="0 0 20 20"
											fill="currentColor"
											width="12"
											height="12"
										>
											<path
												d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z"
											/>
										</svg>
									</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</div>

<style>
	.table-container {
		width: 100%;
		overflow: hidden;
		max-width: 100%;
		padding: 0;
		margin: 0;
		height: 100%;
		max-height: 100%;
		display: flex;
		flex-direction: column;
	}

	.table-header {
		flex-shrink: 0;
		width: 100%;
		background: transparent;
	}

	.table-body {
		flex-grow: 1;
		overflow: hidden auto;
		width: 100%;
		position: relative;
		user-select: none;
	}

	.header-table,
	.body-table {
		width: 100%;
		border-collapse: separate;
		border-spacing: 0 2px;
		margin: 0;
		padding: 0;
		color: var(--text-primary);
		table-layout: fixed;
		background: transparent;
	}

	.body-table {
		border-spacing: 0 1.5px;
	}

	.header-table {
		border-spacing: 0;
	}

	th,
	td {
		padding: clamp(3px, 0.35vw, 5px) clamp(2px, 0.35vw, 3px);
		text-align: right;
		background: transparent;
		overflow: hidden;
		font-size: clamp(0.73rem, 0.82rem, 0.95rem);
		vertical-align: middle;
	}

	/* Ticker column - reduce left padding and define width/alignment */
	th:nth-child(2),
	td:nth-child(2) {
		padding-left: clamp(1px, 0.2vw, 2px);
		width: 25%;
		min-width: 45px;
		text-align: left;
		padding-right: clamp(1px, 0.2vw, 2px);
	}

	/* Second-to-last column (Extended hours) - reduce right padding */
	th:nth-last-child(2),
	td:nth-last-child(2) {
		padding-right: clamp(1px, 0.2vw, 2px);
	}

	/* Header cells */
	th {
		font-weight: normal;
		color: var(--text-secondary);
		position: static;
		z-index: 1;
		background: transparent;
		text-align: right;
	}

	/* Header divider - removed */
	thead tr {
		background: transparent;
		position: relative;
	}

	/* Custom scrollbar for the table body */
	.table-body::-webkit-scrollbar {
		width: 6px;
	}

	.table-body::-webkit-scrollbar-track {
		background: transparent;
		border-radius: 3px;
	}

	.table-body::-webkit-scrollbar-thumb {
		background-color: rgb(255 255 255 / 20%);
		border-radius: 3px;
		border: 1px solid transparent;
		background-clip: content-box;
	}

	.table-body::-webkit-scrollbar-thumb:hover {
		background-color: rgb(255 255 255 / 40%);
	}

	/* Body cells */
	tbody td {
		border: none;
		vertical-align: middle;
	}

	tbody td:first-child {
		border-top-left-radius: 3px;
		border-bottom-left-radius: 3px;
	}

	tbody td:last-child {
		border-top-right-radius: 3px;
		border-bottom-right-radius: 3px;
	}

	/* Sorting styles */
	.sortable {
		cursor: pointer;
		user-select: none;
		position: relative;
		transition: color 0.2s ease;
	}

	.sortable:hover {
		color: #cfd0d2;
		border-bottom: 1px solid #aeafb0;
	}

	.th-content {
		align-items: right;
		justify-content: space-between;
	}

	.sort-icon {
		margin-left: 4px;
		font-size: 0.8em;
		opacity: 0.7;
	}

	.sorting {
		background-color: transparent;
		position: relative;
	}

	.sorting tbody tr {
		opacity: 0.7;
		transition: opacity 0.3s ease;
	}

	.sorting::after {
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

	tr:hover .delete-button {
		opacity: 1;
	}

	.loading,
	.error,
	.no-results {
		padding: 20px;
		text-align: center;
		color: var(--text-secondary);
		margin: 16px;
		border-radius: 8px;
	}

	.error button {
		margin-top: 12px;
		padding: 8px 16px;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		color: #fff;
		font-weight: 500;
		transition: all 0.2s ease;
	}

	.error button:hover {
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
	}

	.ticker-icon {
		width: clamp(15px, 2.3vw, 20px);
		height: clamp(15px, 2.3vw, 20px);
		border-radius: 50%; /* Make icons circular */
		object-fit: cover; /* Ensure icon covers the area nicely */
		background-color: var(--ui-bg-element); /* BG for unloaded images */
		margin-right: clamp(2px, 0.4vw, 3px); /* Space between icon and text */
		vertical-align: middle;
	}

	.default-ticker-icon {
		display: inline-flex; /* Use inline-flex for alignment */
		align-items: center;
		justify-content: center;
		width: clamp(15px, 2.3vw, 20px);
		height: clamp(15px, 2.3vw, 20px);
		border-radius: 50%;
		background-color: var(--ui-border); /* Use border color for background */
		color: var(--text-primary); /* Use primary text color */
		font-size: clamp(0.48rem, 0.3rem + 0.3vw, 0.6rem);
		font-weight: 500;
		user-select: none; /* Prevent text selection */
		margin-right: clamp(2px, 0.4vw, 3px); /* Space between icon and text */
		vertical-align: middle;
	}

	.ticker-name {
		vertical-align: middle;
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

	/* First column (Flag column) - minimize left space */
	th:first-child,
	td:first-child {
		width: 20px;
		min-width: 20px;
		max-width: 20px;
		padding: clamp(1px, 0.15vw, 2px) 0 clamp(1px, 0.15vw, 2px) clamp(1px, 0.2vw, 2px);
		text-align: center;
		vertical-align: middle;
	}

	/* Regular Last column (Delete Button) */
	th:last-child,
	td:last-child {
		width: 30px;
		min-width: 30px;
		max-width: 30px;
		padding: clamp(1px, 0.15vw, 2px) clamp(1px, 0.2vw, 2px) clamp(1px, 0.15vw, 2px) 0;
		text-align: center;
		vertical-align: middle;
	}

	/* Define widths for main content columns */
	th:nth-child(3),
	td:nth-child(3) {
		width: 18%;
		min-width: 45px;
		padding-right: clamp(0px, 0.1vw, 1px);
	}

	th:nth-child(4),
	td:nth-child(4) {
		width: 18%;
		min-width: 45px;
		padding-right: clamp(0px, 0.1vw, 1px);
	}

	th:nth-child(5),
	td:nth-child(5) {
		width: 20%;
		min-width: 55px;
		padding-right: clamp(2px, 0.3vw, 4px);
	}

	th:nth-child(6),
	td:nth-child(6) {
		width: 13%;
		min-width: 40px;
	}

	.delete-button {
		opacity: 0;
		transition: none;
		cursor: pointer;
		border: none;
		background: none;
		color: var(--text-secondary);
		font-size: 0.75em;
		padding: 0;
		line-height: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		margin: auto;
	}

	.delete-button:hover {
		background: rgb(255 255 255 / 10%);
		color: var(--text-secondary);
	}

	/* Table rows */
	tbody tr {
		background: transparent;
		border-radius: 6px;
		position: relative;
	}

	/* Placeholder row indicating drop position */
	.drop-placeholder td {
		padding: 0;
	}

	.drop-placeholder {
		height: 2px;
	}

	.drop-placeholder .default-td {
		background: var(--ui-accent, rgb(255 255 255 / 70%));
		border-radius: 1px;
	}

	/* Absolutely positioned drop-line overlay that does not affect table layout */

	.drop-line {
		position: absolute;
		left: 0;
		right: 0;
		height: 2px;
		background: var(--ui-accent, rgb(255 255 255 / 70%));
		border-radius: 1px;
		pointer-events: none;
	}

	/* Visual drop indicator using outline on hovered target row */
	tbody tr::before {
		content: '';
		position: absolute;
		left: 0;
		right: 0;
		height: 0;
		top: 0;
	}

	/* We cannot bind :hoverIndex directly in CSS, but we can show subtle feedback by cursor change above. */

	.dragging {
		cursor: grabbing;
	}

	tbody tr::after {
		content: '';
		position: absolute;
		bottom: -1px;
		left: 0;
		right: 0;
		height: 1px;
		background: rgb(255 255 255 / 8%);
		border-radius: 0.5px;
	}

	tbody tr:hover {
		background: rgb(255 255 255 / 5%);
		border-radius: 6px;
	}

	tbody tr:hover::after {
		opacity: 0;
	}

	/* Selected row enhancement */
	tbody .selected {
		outline: 2px solid #cfd0d2;
		border-radius: 6px;
	}

	tbody .selected::after {
		opacity: 0;
	}

	/* Mobile-specific styles - make rows taller and wider for better touch targets */
	.table-container.mobile th,
	.table-container.mobile td {
		padding: clamp(8px, 1.2vw, 12px) clamp(6px, 1vw, 10px);
		font-size: clamp(0.8rem, 0.9rem, 1rem);
		line-height: 1.4;
	}

	/* Mobile ticker icons - make them bigger for better visibility */
	.table-container.mobile .ticker-icon,
	.table-container.mobile .default-ticker-icon {
		width: clamp(22px, 3vw, 28px);
		height: clamp(22px, 3vw, 28px);
		margin-right: clamp(4px, 0.6vw, 6px);
	}

	.table-container.mobile .default-ticker-icon {
		font-size: clamp(0.6rem, 0.4rem + 0.4vw, 0.8rem);
		font-weight: 600;
	}

	/* Mobile delete button - make it more visible and easier to tap */
	.table-container.mobile .delete-button {
		opacity: 0.6;
		width: 22px;
		height: 22px;
	}

	.table-container.mobile .delete-button svg {
		width: 13px;
		height: 13px;
	}
</style>
