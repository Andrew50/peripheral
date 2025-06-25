<script lang="ts">
	import Chart from './chart.svelte';
	import { settings } from '$lib/utils/stores/stores';
	import { onMount, tick } from 'svelte';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryChart } from './interface';
	export let width: number;

	// Add focus management
	let containerRef: HTMLDivElement;

	onMount(() => {
		// Wait for DOM to be ready
		tick().then(() => {
			if (containerRef) {
				containerRef.focus();
			}
		});
	});
</script>

<div
	class="chart-container"
	bind:this={containerRef}
	role="application"
	aria-label="Chart Container"
>
	{#each Array.from({ length: $settings.chartRows }) as _, j}
		<div class="row" style="height: calc(100% / {$settings.chartRows})">
			{#each Array.from({ length: $settings.chartColumns }) as _, i}
				<Chart width={width / $settings.chartColumns} chartId={i + j * $settings.chartColumns} />
			{/each}
		</div>
	{/each}
</div>

<style>
	.chart-container {
		display: flex;
		flex-direction: column;
		width: 100%;
		height: 100%;
		position: relative; /* Changed from absolute to relative */
		outline: none; /* Remove focus outline but maintain accessibility */
	}

	.row {
		display: flex;
		width: 100%;
		justify-content: space-between;
		flex: 1;
		min-height: 0;
	}

	/* Add responsive focus indicator for accessibility */
	.chart-container:focus {
		box-shadow: inset 0 0 0 clamp(1px, 0.2vw, 2px) var(--ui-accent, rgba(59, 130, 246, 0.5));
	}

	@media (max-width: 768px) {
		/* Adjust for smaller screens if needed */
		.chart-container {
			overflow: hidden;
		}
	}
</style>
