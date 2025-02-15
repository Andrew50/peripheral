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
		queryInstanceInput('any', ['ticker', 'timeframe', 'timestamp', 'extendedHours'], instance).then(
			(v: Instance) => {
				queryChart(instance);
			}
		);
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

	function debugInstance() {
		console.log('Current instance:', JSON.parse(JSON.stringify(instance)));
	}
</script>

<div tabindex="-1" on:click={handleClick} on:touchstart={handleClick} class="legend">
	{#if instance.detailsUpdateStore?.subscribe}
		<div class="header">
			{#if instance?.icon}
				<img
					src="data:image/svg+xml;base64,{instance.icon}"
					alt="{instance.name} logo"
					class="company-logo"
				/>
			{/if}
			<span class="symbol">{instance?.ticker ?? 'NA'}</span>
			<span class="timeframe">{instance?.timeframe ?? 'NA'}</span>
		</div>

		<div class="price-grid" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}">
			<div class="price-row">
				<span class="label">O</span>
				<span class="value">{$hoveredCandleData.open.toFixed(2)}</span>
				<span class="label">H</span>
				<span class="value">{$hoveredCandleData.high.toFixed(2)}</span>
			</div>
			<div class="price-row">
				<span class="label">L</span>
				<span class="value">{$hoveredCandleData.low.toFixed(2)}</span>
				<span class="label">C</span>
				<span class="value">{$hoveredCandleData.close.toFixed(2)}</span>
			</div>
		</div>

		<div class="metrics-grid">
			<div class="metric">
				<span class="label">CHG</span>
				<span class="value" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}">
					{$hoveredCandleData.chg.toFixed(2)} ({$hoveredCandleData.chgprct.toFixed(2)}%)
				</span>
			</div>
			<div class="metric">
				<span class="label">VOL</span>
				<span class="value">{formatLargeNumber($hoveredCandleData.volume, $settings.dolvol)}</span>
			</div>
			<div class="metric">
				<span class="label">ADR</span>
				<span class="value">{$hoveredCandleData.adr?.toFixed(2) ?? 'NA'}</span>
			</div>
			<div class="metric">
				<span class="label">RVOL</span>
				<span class="value">{$hoveredCandleData.rvol?.toFixed(2) ?? 'NA'}</span>
			</div>
		</div>

		<button class="debug-button" on:click|stopPropagation={debugInstance}> Debug Instance </button>
	{/if}
</div>

<style>
	.legend {
		position: absolute;
		top: 10px;
		left: 10px;
		background-color: rgba(0, 0, 0, 0.85);
		border: 1px solid rgba(255, 255, 255, 0.1);
		padding: 8px;
		border-radius: 4px;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		color: #e0e0e0;
		z-index: 900;
		width: 300px;
		min-width: 300px;
		backdrop-filter: blur(4px);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
		user-select: none;
		cursor: pointer;
	}

	.header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 6px;
		padding-bottom: 4px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
	}

	.symbol {
		font-size: 16px;
		font-weight: 600;
		color: #fff;
	}

	.timeframe {
		font-size: 12px;
		color: #999;
		padding: 2px 6px;
		background: rgba(255, 255, 255, 0.1);
		border-radius: 3px;
	}

	.price-grid {
		display: flex;
		flex-direction: column;
		gap: 2px;
		margin-bottom: 6px;
		padding-bottom: 4px;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
	}

	.price-row {
		display: grid;
		grid-template-columns: 15px 70px 15px 70px;
		gap: 8px;
		align-items: center;
	}

	.metrics-grid {
		display: grid;
		grid-template-columns: 60% 40%;
		gap: 4px;
		width: 100%;
	}

	.metric {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		overflow: hidden;
	}

	.label {
		font-size: 12px;
		color: #999;
		font-weight: 500;
		min-width: 35px;
		flex-shrink: 0;
	}

	.value {
		font-size: 12px;
		font-weight: 500;
		font-feature-settings: 'tnum';
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		flex: 1;
	}

	/* Hover effect */
	.legend:hover {
		background-color: rgba(0, 0, 0, 0.9);
		border-color: rgba(255, 255, 255, 0.2);
	}

	/* Ensure legend stays within chart bounds */
	@media (max-width: 400px) {
		.legend {
			width: calc(100% - 20px);
			min-width: 260px;
		}
	}

	.debug-button {
		margin-top: 8px;
		width: 100%;
		padding: 4px;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		color: #999;
		border-radius: 3px;
		cursor: pointer;
		font-size: 12px;
	}

	.debug-button:hover {
		background: rgba(255, 255, 255, 0.15);
	}
</style>
