<script lang="ts">
	import Chart from './chart.svelte';
	import { settings } from '$lib/core/stores';
	export let width: number;
</script>

<div class="chart-container">
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
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
	}

	.row {
		display: flex;
		width: 100%;
		justify-content: space-between;
		flex: 1;
		min-height: 0;
	}
</style>
