<script lang="ts">
	import { onMount, createEventDispatcher } from 'svelte';
	import { get, writable } from 'svelte/store';
	import { strategies } from '$lib/utils/stores/stores';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import type { Strategy } from '$lib/utils/types/types';

	/***********************
	 *     ─ Types ─       *
	 ***********************/
	/** StrategyId follows the convention of the parent module */
	type StrategyId = number | 'new' | null;

	/***********************
	 *   ─ Public Props ─  *
	 ***********************/
	/** Currently‑selected strategy id (two‑way bindable) */
	export let selectedId: StrategyId = null;

	/** Show a “New strategy…” option (defaults to false) */
	export let allowNew: boolean = false;

	/** Optional placeholder to show when nothing is selected */
	export let placeholder: string = 'Select a strategy…';

	/***********************
	 *   ─ Internals  ─    *
	 ***********************/
	const loading = writable(false);
	const dispatch = createEventDispatcher<{ change: StrategyId }>();

	async function loadStrategies() {
		if (get(strategies)?.length) return; // already fetched

		loading.set(true);
		try {
			const data = await privateRequest<Strategy[]>('getStrategies', {});
			strategies.set(data);
		} finally {
			loading.set(false);
		}
	}

	onMount(loadStrategies);

	function handleChange(event: Event) {
		const value = (event.target as HTMLSelectElement).value;
		selectedId = value === '' ? null : value === 'new' ? 'new' : Number(value);
		dispatch('change', selectedId);
	}
</script>

<select
	class="strategy-dropdown"
	bind:value={selectedId}
	on:change={handleChange}
	disabled={$loading || !Array.isArray($strategies)}
>
	<option value="" disabled selected>{placeholder}</option>

	{#if allowNew}
		<option value="new"> ＋ New strategy… </option>
	{/if}

	{#if Array.isArray($strategies)}
		{#each $strategies as s (s.strategyId)}
			<option value={s.strategyId}>
				{s.name}
			</option>
		{/each}
	{/if}
</select>

<style>
	:global(:root) {
		/* fallback palette */
		--sd-border: #cbd5e1;
		--sd-bg: #f8fafc;
		--sd-bg-hover: #e2e8f0;
		--sd-text: #0f172a;
	}

	.strategy-dropdown {
		padding: 0.4rem 0.55rem;
		font-size: 0.9rem;
		color: var(--sd-text);
		background: var(--sd-bg);
		border: 1px solid var(--sd-border);
		border-radius: 4px;
		outline: none;
		transition: background 120ms ease;
		min-width: 14rem;
	}

	.strategy-dropdown:hover:not(:disabled) {
		background: var(--sd-bg-hover);
		cursor: pointer;
	}

	.strategy-dropdown:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}
</style>
