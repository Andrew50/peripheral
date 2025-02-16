<script lang="ts">
	import L1 from './l1.svelte';
	import TimeAndSales from './timeAndSales.svelte';
	import { get, writable, type Writable } from 'svelte/store';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import type { Instance } from '$lib/core/types';

	let instance: Writable<Instance> = writable({});
	let container: HTMLDivElement;

	function handleKey(event: KeyboardEvent) {
		// Example: if user presses tab or alphanumeric, prompt ticker change
		if (event.key == 'Tab' || /^[a-zA-Z0-9]$/.test(event.key)) {
			const current = get(instance);
			queryInstanceInput(['ticker'], ['ticker'], current)
				.then((updated: Instance) => {
					instance.set(updated);
				})
				.catch(() => {});
		}
	}

	$: if (container) {
		container.addEventListener('keydown', handleKey);
	}
</script>

<div bind:this={container} tabindex="-1" class="quotes-container">
	<div class="ticker-display">
		<span>{$instance.ticker || '--'}</span>
	</div>

	<div class="content-wrapper">
		<L1 {instance} />
		<TimeAndSales {instance} />
	</div>
</div>

<style>
	.quotes-container {
		width: 100%;
		outline: none; /* so we can focus and catch keyboard events */
	}
	.ticker-display {
		font-family: Arial, sans-serif;
		font-size: 20px;
		color: white;
		background-color: black;
		border: 1px solid #333;
		border-radius: 5px;
		width: 100%;
		text-align: center;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		margin-bottom: 8px;
	}
	.ticker-display span {
		display: inline-block;
	}
	.content-wrapper {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
</style>
