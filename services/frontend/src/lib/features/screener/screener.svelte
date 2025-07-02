<!-- screen.svelte -->
<script lang="ts">
	import { writable, get } from 'svelte/store';
	import List from '$lib/components/list.svelte';
	import '$lib/styles/global.css';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import { queueRequest } from '$lib/utils/helpers/backend';
	import { strategies } from '$lib/utils/stores/stores';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';

	/* ─────────────── Local state ─────────────── */
	let screens: Writable<Screen[]> = writable([]);
	let selectedDate: Writable<number> = writable(0);

	/** Which strategies should be screened? */
	const selectedStrategyIds: Writable<Set<number>> = writable(new Set());

	interface Screen extends Instance {
		strategyType: string;
		score: number;
		flagged: boolean;
	}

	/* ─────────────── Helpers ─────────────── */
	function toggleStrategy(id: number) {
		selectedStrategyIds.update((set) => {
			const n = new Set(set);
			n.has(id) ? n.delete(id) : n.add(id);
			return n;
		});
	}

	function runScreen() {
		const strategyIds = Array.from(get(selectedStrategyIds));
		const dateToScreen = get(selectedDate);

		queueRequest<Screen[]>('screen', {
			strategyIds,
			timestamp: dateToScreen / 1000
		}).then((resp) => {
			screens.set(resp);
		});
	}

	/** Prompt user for a new date */
	function changeDate() {
		queryInstanceInput(['timestamp'], ['timestamp'], { timestamp: $selectedDate }).then(
			(v: Instance) => {
				if (v.timestamp !== undefined) selectedDate.set(v.timestamp);
			}
		);
	}
</script>

<!-- ─────────────── Strategy toggle buttons ─────────────── -->
<div class="controls-container">
	{#if Array.isArray($strategies)}
		{#each $strategies as strategy (strategy.strategyId)}
			<button
				class="toggle-button {$selectedStrategyIds.has(strategy.strategyId) ? 'active' : ''}"
				on:click={() => toggleStrategy(strategy.strategyId)}
			>
				{strategy.name}
			</button>
		{/each}
	{:else}
		<p1> No strategies Found </p1>
	{/if}
</div>

<!-- ─────────────── Run / Date buttons ─────────────── -->
<div class="button-row">
	<!--<button on:click={changeDate}>Change Date</button>-->
	<button on:click={runScreen}>
		Screen {UTCTimestampToESTString($selectedDate)}
	</button>
</div>

<!-- ─────────────── Results list ─────────────── -->
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={screens}
	columns={['Ticker', 'Chg%', 'Setup', 'Score', 'Ext.']}
/>

<style>
	/* ── unchanged styles omitted for brevity ── */
</style>
