<script lang="ts">
	import Chart from './chart.svelte';
	import { settings } from '$lib/utils/stores/stores';
	import { onMount, tick } from 'svelte';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryChart } from './interface';
	export let defaultChartData: any;

	// Add focus management
	let containerRef: HTMLDivElement;
	let containerWidth = 0;
	let chartWidth = 0;
	onMount(() => {
		// Wait for DOM to be ready
		tick().then(() => {
			if (containerRef) {
				containerRef.focus();
			}
		});
		const ro = new ResizeObserver((entries) => {
			containerWidth = entries[0].contentRect.width;
			chartWidth = Math.floor(containerWidth / $settings.chartColumns);
		});

		ro.observe(containerRef);
		return () => ro.disconnect();
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
				<Chart
					width={chartWidth}
					chartId={i + j * $settings.chartColumns}
					defaultChartData={i + j * $settings.chartColumns === 0 ? defaultChartData : null}
				/>
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
		min-width: 0;
		min-height: 0;
	}

	/* Add responsive focus indicator for accessibility */
	.chart-container:focus {
		box-shadow: inset 0 0 0 clamp(1px, 0.2vw, 2px) var(--ui-accent, rgb(59 130 246 / 50%));
	}

	@media (width <= 768px) {
		/* Adjust for smaller screens if needed */
		.chart-container {
			overflow: hidden;
		}
	}
</style>
