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
	import { newAlert } from '$lib/features/alerts/interface';
	let longPressTimer: any;
	export let list: Writable<Instance[]> = writable([]);
	export let columns: Array<string>;
	export let parentDelete = (v: Instance) => {};
	export let formatters: { [key: string]: (value: any) => string } = {};
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
		return trade.trades || [];
	}
</script>

<div class="table-container">
	<table>
		<thead>
			<tr class="defalt-tr">
				{#if expandable}
					<th class="defalt-th"></th>
				{/if}
				<th class="defalt-th"></th>
				{#each columns as col}
					<th data-type={col.toLowerCase().replace(/ /g, '-')}>{col}</th>
				{/each}
				<th class="defalt-th"></th>
			</tr>
		</thead>
		{#if Array.isArray($list) && $list.length > 0}
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
						<td class="defalt-td">
							{#if isFlagged(watch, $flagWatchlist)}
								<span class="flag-icon">⚑</span> <!-- Example flag icon -->
							{/if}
						</td>
						{#each columns as col}
							{#if ['price', 'change', 'change %', 'change % extended'].includes(col)}
								<td class="defalt-td">
									<StreamCell
										on:contextmenu={(event) => {
											event.preventDefault();
											event.stopPropagation();
										}}
										instance={watch}
										type={col}
									/>
								</td>
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
						<td class="defalt-td">
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
</style>
