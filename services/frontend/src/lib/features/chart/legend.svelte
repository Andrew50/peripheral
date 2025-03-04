<script lang="ts">
	export let hoveredCandleData;
	import type { Instance } from '$lib/core/types';
	import { queryChart } from './interface';
	import { writable } from 'svelte/store';
	export let instance: Instance;
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { settings } from '$lib/core/stores';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { onMount, onDestroy } from 'svelte';

	// Add new props for chart dimensions
	export let width: number;

	let legendElement: HTMLDivElement;
	let isOverflowing = false;
	let showPriceGrid = true;
	let showMetricsGrid = true;
	let resizeObserver: ResizeObserver;
	let checkOverflowTimeout: number;

	let isCollapsed = false;
	let isUpdating = false;

	function toggleCollapse() {
		isCollapsed = !isCollapsed;
	}

	function handleClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		queryInstanceInput([], ['ticker', 'timeframe', 'timestamp', 'extendedHours'], instance).then(
			(v: Instance) => {
				queryChart(v, true);
			}
		);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			queryInstanceInput(
				'any',
				['ticker', 'timeframe', 'timestamp', 'extendedHours'],
				instance
			).then((v: Instance) => {
				queryChart(v, true);
			});
		}
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

	// Modify the debounced check overflow function
	function debouncedCheckOverflow() {
		if (checkOverflowTimeout) {
			clearTimeout(checkOverflowTimeout);
		}
		checkOverflowTimeout = setTimeout(() => {
			if (!isUpdating) {
				checkOverflow();
			}
		}, 100);
	}

	// Function to check and handle overflow
	function checkOverflow() {
		if (!legendElement || isUpdating) return;

		isUpdating = true;

		const legendRect = legendElement.getBoundingClientRect();
		const parentRect = legendElement.parentElement?.getBoundingClientRect();

		if (!parentRect) {
			isUpdating = false;
			return;
		}

		// Store previous state
		const prevShowMetrics = showMetricsGrid;
		const prevShowPrice = showPriceGrid;
		const prevOverflow = isOverflowing;

		// Reset visibility
		showPriceGrid = true;
		showMetricsGrid = true;
		isOverflowing = false;

		// Check if legend is too wide
		if (legendRect.right > parentRect.right - 10) {
			isOverflowing = true;
			showMetricsGrid = false;

			// If still overflowing after a brief delay, hide price grid
			if (legendRect.right > parentRect.right - 10) {
				showPriceGrid = false;
			}
		}

		// Only trigger update if state actually changed
		if (
			prevShowMetrics !== showMetricsGrid ||
			prevShowPrice !== showPriceGrid ||
			prevOverflow !== isOverflowing
		) {
			legendElement.style.width = 'auto';
		}

		// Reset the updating flag after a short delay
		setTimeout(() => {
			isUpdating = false;
		}, 50);
	}

	// Add reactive statements to log changes
	$: {
		console.log('Instance updated:', {
			ticker: instance?.ticker,
			timeframe: instance?.timeframe,
			timestamp: instance?.timestamp,
			extendedHours: instance?.extendedHours,
			fullInstance: instance
		});
	}

	// Add reactive statement specifically for ticker changes
	$: {
		if (instance?.ticker) {
			console.log('Ticker changed to:', instance.ticker);
		}
	}

	// Watch for content changes that might affect size
	$: if (hoveredCandleData || instance || width) {
		debouncedCheckOverflow();
	}

	onMount(() => {
		instance;
		// Initialize ResizeObserver with a more conservative callback
		resizeObserver = new ResizeObserver((entries) => {
			if (!isUpdating) {
				debouncedCheckOverflow();
			}
		});

		// Observe both the legend and its parent
		if (legendElement) {
			resizeObserver.observe(legendElement);
			if (legendElement.parentElement) {
				resizeObserver.observe(legendElement.parentElement);
			}
		}
	});

	onDestroy(() => {
		// Clean up
		if (checkOverflowTimeout) {
			clearTimeout(checkOverflowTimeout);
		}
		if (resizeObserver) {
			resizeObserver.disconnect();
		}
	});

	// Watch for content changes that might affect size
	$: if (hoveredCandleData || instance || width) {
		debouncedCheckOverflow();
	}
</script>

<div
	bind:this={legendElement}
	tabindex="-1"
	role="button"
	on:click={handleClick}
	on:keydown={handleKeydown}
	on:touchstart={handleClick}
	class="legend {isCollapsed ? 'collapsed' : ''} {isOverflowing ? 'compact' : ''}"
>
	<div class="header">
		{#if instance?.icon}
			<img
				src={instance.icon.startsWith('data:')
					? instance.icon
					: `data:image/jpeg;base64,${instance.icon}`}
				alt="{instance.name} logo"
				class="company-logo"
				on:error={() => {}}
			/>
		{/if}
		<span class="symbol">{instance?.ticker || 'NaN'}</span>
		<span class="metadata">
			<span class="timeframe">{instance?.timeframe || '1d'}</span>
			{#if !isOverflowing}
				<span class="timestamp">{UTCTimestampToESTString(instance?.timestamp ?? 0)}</span>
				<span class="session-type">{instance?.extendedHours ? 'Extended' : 'Regular'}</span>
			{/if}
		</span>
	</div>

	{#if !isCollapsed}
		{#if showPriceGrid}
			<div
				class="price-grid"
				style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}"
			>
				<div class="price-row">
					<span class="label">O</span>
					<span class="value">{$hoveredCandleData.open.toFixed(2)}</span>
					<span class="label">H</span>
					<span class="value">{$hoveredCandleData.high.toFixed(2)}</span>
					<span class="label">L</span>
					<span class="value">{$hoveredCandleData.low.toFixed(2)}</span>
					<span class="label">C</span>
					<span class="value">{$hoveredCandleData.close.toFixed(2)}</span>
				</div>
			</div>
		{/if}

		{#if showMetricsGrid}
			<div class="metrics-grid">
				<div class="metric">
					<span class="label">CHG</span>
					<span
						class="value"
						style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}"
					>
						{$hoveredCandleData.chg.toFixed(2)} ({$hoveredCandleData.chgprct.toFixed(2)}%)
					</span>
				</div>
				<div class="metric">
					<span class="label">VOL</span>
					<span class="value">{formatLargeNumber($hoveredCandleData.volume, $settings.dolvol)}</span
					>
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
		max-width: calc(100% - 20px);
		width: fit-content;
		min-width: min-content;
		backdrop-filter: var(--backdrop-blur);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
		user-select: none;
		cursor: pointer;
	}

	/* Update hover styles to not affect width/layout */
	.legend:hover {
		background-color: var(--ui-bg-primary);
		border-color: var(--ui-border);
	}

	/* Make compact styles more specific to override hover */
	.legend.compact {
		width: auto !important; /* Use !important to ensure it overrides hover */
	}

	.legend.compact .metadata {
		flex-wrap: nowrap;
		max-width: 100px !important;
	}

	.legend.compact .timeframe {
		max-width: 100% !important;
	}

	/* Ensure compact state is maintained even on hover */
	.legend.compact:hover .metadata {
		flex-wrap: nowrap;
		max-width: 100px !important;
	}

	.legend.compact:hover {
		width: auto !important;
	}

	/* Hide elements in compact mode even on hover */
	.legend.compact:hover .timestamp,
	.legend.compact:hover .session-type {
		display: none;
	}

	.header {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 8px;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--ui-border);
		min-width: 0;
		flex-wrap: wrap;
	}

	.symbol {
		font-size: 14px;
		line-height: 20px;
		font-weight: 600;
		color: var(--text-primary);
		white-space: nowrap;
		padding: 4px 10px;
		background: var(--ui-bg-element);
		border-radius: 4px;
		border: 1px solid var(--ui-border);
	}

	.metadata {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		overflow: hidden;
		min-width: 0;
	}

	.timeframe,
	.timestamp,
	.session-type {
		font-size: 13px;
		line-height: 18px;
		color: var(--text-secondary);
		padding: 4px 8px;
		background: var(--ui-bg-element);
		border-radius: 4px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		border: 1px solid var(--ui-border);
	}

	.timestamp {
		font-family: monospace;
	}

	.price-grid {
		display: flex;
		flex-direction: column;
		margin-bottom: 8px;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--ui-border);
	}

	.price-row {
		display: grid;
		grid-template-columns: 15px 45px 15px 45px 15px 45px 15px 45px;
		gap: 4px;
		align-items: center;
		height: 20px;
	}

	.metrics-grid {
		display: grid;
		grid-template-columns: 60% 40%;
		gap: 6px;
		width: 100%;
	}

	.metric {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		overflow: hidden;
		height: 20px;
	}

	.label {
		font-size: 12px;
		line-height: 16px;
		color: var(--text-secondary);
		font-weight: 500;
		min-width: 35px;
		flex-shrink: 0;
	}

	.value {
		font-size: 12px;
		line-height: 16px;
		font-weight: 500;
		font-feature-settings: 'tnum';
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		flex: 1;
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
