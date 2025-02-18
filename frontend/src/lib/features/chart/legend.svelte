<script lang="ts">
	export let hoveredCandleData;
	import type { Instance } from '$lib/core/types';
	import { queryChart } from './interface';
	import { writable } from 'svelte/store';
	export let instance: Instance;
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { settings } from '$lib/core/stores';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';

	let isCollapsed = false;

	function toggleCollapse() {
		isCollapsed = !isCollapsed;
	}

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
</script>

<div
	tabindex="-1"
	on:click={handleClick}
	on:touchstart={handleClick}
	class="legend {isCollapsed ? 'collapsed' : ''}"
>
	<div class="header">
		{#if instance?.icon}
			<img
				src="data:image/jpeg;base64,{instance.icon}"
				alt="{instance.name} logo"
				class="company-logo"
			/>
		{/if}
		<span class="symbol">{instance?.ticker || 'NaN'}</span>
		<span class="metadata">
			<span class="timeframe">{instance?.timeframe ?? 'NA'}</span>
			<span class="timestamp">{UTCTimestampToESTString(instance?.timestamp ?? 0)}</span>
			<span class="session-type">{instance?.extendedHours ? 'Extended' : 'Regular'}</span>
		</span>
	</div>

	{#if !isCollapsed}
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
	{/if}

	<div class="collapse-row" on:click|stopPropagation={toggleCollapse}>
		<div class="divider"></div>
		<button class="utility-button" aria-label="Toggle legend">
			<svg
				class="arrow-icon"
				viewBox="0 0 24 24"
				width="16"
				height="16"
				stroke="currentColor"
				stroke-width="2"
				fill="none"
			>
				<path d="M18 15l-6-6-6 6" />
			</svg>
		</button>
	</div>
</div>

<style>
	.legend {
		position: absolute;
		top: 10px;
		left: 10px;
		background-color: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		padding: 8px;
		border-radius: 4px;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		color: var(--text-primary);
		z-index: 900;
		width: 300px;
		min-width: 300px;
		backdrop-filter: var(--backdrop-blur);
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
		border-bottom: 1px solid var(--ui-border);
		min-width: 0;
		flex-wrap: wrap;
	}

	.symbol {
		font-size: 16px;
		font-weight: 600;
		color: var(--text-primary);
		white-space: nowrap;
	}

	.metadata {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
		overflow: hidden;
		flex: 1;
		min-width: 0;
		max-width: calc(100% - 16px);
	}

	.timeframe,
	.timestamp,
	.session-type {
		font-size: 12px;
		color: var(--text-secondary);
		padding: 2px 6px;
		background: var(--ui-bg-element);
		border-radius: 3px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: calc(100% - 12px);
	}

	.timestamp {
		font-family: monospace;
	}

	.price-grid {
		display: flex;
		flex-direction: column;
		gap: 2px;
		margin-bottom: 6px;
		padding-bottom: 4px;
		border-bottom: 1px solid var(--ui-border);
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
		color: var(--text-secondary);
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
		background-color: var(--ui-bg-primary);
		border-color: var(--ui-border);
	}

	/* Ensure legend stays within chart bounds */
	@media (max-width: 400px) {
		.legend {
			width: calc(100% - 20px);
			min-width: 260px;
		}
	}

	.company-logo {
		width: 24px;
		height: 24px;
		object-fit: contain;
		border-radius: 4px;
	}

	.legend.collapsed {
		width: auto;
		min-width: auto;
		max-width: 100%;
	}

	.legend.collapsed .header {
		margin-bottom: 0;
		border-bottom: none;
		padding-bottom: 0;
	}

	.collapse-row {
		display: none;
		flex-direction: column;
		align-items: center;
		margin-top: 2px;
		cursor: pointer;
		height: 16px;
	}

	/* Show collapse row on hover for expanded state */
	.legend:hover .collapse-row {
		display: flex;
	}

	/* For collapsed state, only show on hover */
	.legend.collapsed .collapse-row {
		display: none;
	}

	.legend.collapsed:hover .collapse-row {
		display: flex;
		margin-top: 0;
	}

	.divider {
		height: 1px;
		background-color: var(--ui-border);
		width: 100%;
		margin: 2px 0;
	}

	.arrow-icon {
		transform: rotate(0deg);
		transition: transform 0.2s ease;
	}

	.collapsed .arrow-icon {
		transform: rotate(180deg);
	}
</style>
