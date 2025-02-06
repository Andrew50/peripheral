<script lang="ts">
	export let hoveredCandleData;
	import type { Instance } from '$lib/core/types';
	import { queryChart } from './interface';
	import { writable } from 'svelte/store';
	export let instance: Instance;
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { settings } from '$lib/core/stores';
	function handleClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		queryInstanceInput('any', instance).then((v: Instance) => {
			queryChart(instance);
		});
	}
	function formatLargeNumber(volume: number, dolvol: boolean): string {
		if (volume === undefined) {
			return 'NA';
		}
		let vol;
		if (volume >= 1e12) {
			vol = (volume / 1e12).toFixed(2) + 'T';
		} else if (volume >= 1e9) {
			vol = (volume / 1e9).toFixed(2) + 'B';
		} else if (volume >= 1e6) {
			vol = (volume / 1e6).toFixed(2) + 'M';
		} else if (volume >= 1e3) {
			vol = (volume / 1e3).toFixed(2) + 'K';
		} else {
			vol = volume.toFixed(0);
		}
		if (dolvol) {
			vol += '$';
		}
		return vol;
	}
</script>

<div tabindex="-1" on:click={handleClick} on:touchstart={handleClick} class="legend">
	<div class="query">
		{instance?.ticker ?? 'NA'}
		{instance?.timeframe ?? 'NA'}
	</div>
	<div class="ohlcv" style="color: {$hoveredCandleData.chgprct < 0 ? 'red' : 'green'}">
		O: {$hoveredCandleData.open.toFixed(2)}
		H: {$hoveredCandleData.high.toFixed(2)}
		L: {$hoveredCandleData.low.toFixed(2)}
		C: {$hoveredCandleData.close.toFixed(2)}
		CHG: {$hoveredCandleData.chg.toFixed(2)}
		({$hoveredCandleData.chgprct.toFixed(2)}%) V: {formatLargeNumber(
			$hoveredCandleData.volume,
			$settings.dolvol
		)}
		AR: {$hoveredCandleData.adr?.toFixed(2)}
		RVOL: {$hoveredCandleData.rvol?.toFixed(2)}
	</div>
	<!--<div class="mcap">
		MCAP: {formatLargeNumber($hoveredCandleData.mcap, false)}
	</div>-->
</div>

<style>
	.legend {
		position: absolute;
		top: 10px;
		left: 10px;
		background-color: rgba(0, 0, 0, 0.5); /* Semi-transparent black background */
		padding: 5px;
		border-radius: 4px;
		font-family: Arial, sans-serif;
		color: white; /* White text */
		z-index: 900;
	}

	.query {
		font-size: 20px; /* Smaller font for stock name and timeframe */
		margin-bottom: 5px;
	}

	.ohlcv {
		display: grid;
		grid-template-columns: auto auto; /* Align labels and values */
		font-size: 16px; /* Smaller, cleaner font size */
		gap: 4px;
	}
	.mcap {
		display: grid;
		grid-template-columns: auto auto; /* Align labels and values */
		font-size: 16px; /* Smaller, cleaner font size */
		gap: 4px;
	}
</style>
