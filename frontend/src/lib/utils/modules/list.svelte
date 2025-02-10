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

	function isFlagged(instance: Instance, flagWatch: Instance[]) {
		if (!Array.isArray(flagWatch)) return false;
		return flagWatch.some((item) => item.ticker === instance.ticker);
	}

	let selectedRowIndex = -1;

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
			queryChart(instance);
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
</script>

{#if Array.isArray($list) && $list.length > 0}
	<div class="table-container">
		<table>
			<thead>
				<tr>
					<th></th>
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
					>
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
									}}>{watch[col]}</td
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
</style>
