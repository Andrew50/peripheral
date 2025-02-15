<!-- screen.svelte -->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable, get } from 'svelte/store';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { queryInstanceRightClick } from '$lib/utils/popups/rightClick.svelte';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import { flagWatchlist } from '$lib/core/stores';
	import { flagSecurity } from '$lib/utils/flag';
	let longPressTimer: any;
	export let list: Writable<Instance[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: Instance) => {};
	export let formatters: {[key: string]: (value: any) => string} = {};
	export let expandable = false;
	export let expandedContent: (item: any) => any = () => null;

	let selectedRowIndex = -1;
	let expandedRows = new Set();

	function isFlagged(instance: Instance, flagWatch: Instance[]) {
		if (!Array.isArray(flagWatch)) return false;
		return flagWatch.some((item) => item.ticker === instance.ticker);
	}

	function rowRightClick(event: MouseEvent, watch: Instance) {
		event.preventDefault();
		queryInstanceRightClick(event, watch, 'list');
	}
	function deleteRow(event: MouseEvent, watch: Instance) {
		event.stopPropagation();
		event.preventDefault();
		list.update((v: Instance[]) => {
			return v.filter((s) => s !== watch);
		});
		parentDelete(watch);
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
			if('openQuantity' in instance) {
				queryChart(instance);
			} else {
				queryChart(instance);
			}
		} else if (even === 1) {
			flagSecurity(instance);
		} else if (even === 2) {
			rowRightClick(event, instance);
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
		if (expandedRows.has(index)) {
			expandedRows.delete(index);
		} else {
			expandedRows.add(index);
		}
		expandedRows = expandedRows; // Trigger reactivity
	}

	function formatValue(value: any, column: string): string {
		if (formatters[column]) {
			return formatters[column](value);
		}
		return value?.toString() ?? 'N/A';
	}

	function getAllOrders(trade) {
		const entries = trade.entries?.map(entry => ({
			...entry,
			type: 'Entry'
		})) || [];
		
		const exits = trade.exits?.map(exit => ({
			...exit,
			type: 'Exit'
		})) || [];

		return [...entries, ...exits].sort((a, b) => a.time - b.time);
	}
</script>

{#if Array.isArray($list) && $list.length > 0}
	<div class="table-container">
		<table>
			<thead>
				<tr>
					{#if expandable}
						<th></th>
					{/if}
					{#each columns as col}
						<th>{col}</th>
					{/each}
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each $list as watch, i}
					<tr
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
							<td class="expand-cell">
								<span class="expand-icon">{expandedRows.has(i) ? '−' : '+'}</span>
							</td>
						{/if}
						<td>
							{#if isFlagged(watch, $flagWatchlist)}
								<span class="flag-icon">⚑</span> <!-- Example flag icon -->
							{/if}
						</td>
						{#each columns as col}
							{#if col === 'price'}
								<StreamCell
									on:contextmenu={(event) => {
										event.preventDefault();
										event.stopPropagation();
									}}
									instance={watch}
									type="price"
								/>
							{:else if col === 'change'}
								<StreamCell
									on:contextmenu={(event) => {
										event.preventDefault();
										event.stopPropagation();
									}}
									instance={watch}
									type="change"
								/>
							{:else if col === 'change %'}
								<StreamCell
									on:contextmenu={(event) => {
										event.preventDefault();
										event.stopPropagation();
									}}
									instance={watch}
									type="change %"
								/>
							{:else if col === 'timestamp'}
								<td
									on:contextmenu={(event) => {
										event.preventDefault();
										event.stopPropagation();
									}}>{UTCTimestampToESTString(watch[col])}</td
								>
							{:else}
								<td
									on:contextmenu={(event) => {
										event.preventDefault();
										event.stopPropagation();
									}}>{formatValue(watch[col], col)}</td
								>
							{/if}
						{/each}
						<td>
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
							<td colspan={columns.length + 1}>
								<div class="trade-details">
									<h4>Trade Details</h4>
									<table>
										<thead>
											<tr>
												<th>Time</th>
												<th>Type</th>
												<th>Price</th>
												<th>Shares</th>
											</tr>
										</thead>
										<tbody>
											{#each getAllOrders(watch) as order}
												<tr>
													<td>{UTCTimestampToESTString(order.time)}</td>
													<td class={order.type.toLowerCase()}>{order.type}</td>
													<td>${order.price.toFixed(2)}</td>
													<td>{order.shares}</td>
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
		</table>
	</div>
{/if}

<style>
	.flag-icon {
		color: var(--c3);
		font-size: 16px;
		margin-right: 5px;
	}
	.delete-button {
		padding: 1px;
	}
	.selected {
		outline: 2px solid #4a9eff;
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
		color: white;
	}

	th, td {
		padding: 8px;
		text-align: left;
		border-bottom: 1px solid #444;
	}

	th {
		background-color: #333;
		font-weight: bold;
	}

	tr {
		background-color: #222;
		transition: background-color 0.2s;
	}

	tr:hover {
		background-color: #2a2a2a;
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
		display: inline-block;
		width: 16px;
		height: 16px;
		line-height: 16px;
		text-align: center;
		background-color: #444;
		border-radius: 3px;
		font-weight: bold;
		font-size: 12px;
	}

	.expanded-content {
		background-color: #2a2a2a;
	}

	.expanded-content td {
		padding: 8px;
	}

	.trade-details {
		background-color: #333;
		padding: 8px;
		border-radius: 4px;
	}

	.trade-details h4 {
		margin: 0 0 6px 0;
		color: #888;
		font-size: 0.9em;
	}

	.trade-details table {
		width: 100%;
		font-size: 0.85em;
	}

	.trade-details th {
		background-color: #444;
		padding: 6px 8px;
	}

	.trade-details tr {
		background-color: transparent;
	}

	.trade-details tr:hover {
		background-color: #3a3a3a;
	}

	.entry {
		color: #4caf50;
	}

	.exit {
		color: #f44336;
	}
</style>
