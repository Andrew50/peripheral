<script lang="ts">
	export let hoveredCandleData;
	import type { Instance } from '$lib/utils/types/types';
	export let instance: Instance;
	import { settings } from '$lib/utils/stores/stores';
	import { onMount, onDestroy } from 'svelte';

	// Add new props for chart dimensions
	export let width: number;

	let legendElement: HTMLDivElement;
	let isOverflowing = false;
	let showPriceGrid = true;
	let showMetricsGrid = true;
	let resizeObserver: ResizeObserver;
	let checkOverflowTimeout: ReturnType<typeof setTimeout>;

	let isUpdating = false;

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
</script>

<div bind:this={legendElement} tabindex="-1" class="legend {isOverflowing ? 'compact' : ''}">
	{#if showPriceGrid}
		<!-- OHLC Row -->
		<div class="ohlc-row">
			<span class="label">O</span>
			<span class="value" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}"
				>{$hoveredCandleData.open.toFixed(2)}</span
			>
			<span class="label">H</span>
			<span class="value" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}"
				>{$hoveredCandleData.high.toFixed(2)}</span
			>
			<span class="label">L</span>
			<span class="value" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}"
				>{$hoveredCandleData.low.toFixed(2)}</span
			>
			<span class="label">C</span>
			<span class="value" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}"
				>{$hoveredCandleData.close.toFixed(2)}</span
			>
		</div>
	{/if}

	{#if showMetricsGrid}
		<!-- Metrics Row -->
		<div class="metrics-row">
			<span class="label">CHG</span>
			<span class="value" style="color: {$hoveredCandleData.chgprct < 0 ? '#ef5350' : '#089981'}">
				{$hoveredCandleData.chg >= 0 ? '+' : ''}{$hoveredCandleData.chg.toFixed(2)} ({$hoveredCandleData.chgprct >=
				0
					? '+'
					: ''}{$hoveredCandleData.chgprct.toFixed(2)}%)
			</span>
			<span class="label">VOL</span>
			<span class="value">{formatLargeNumber($hoveredCandleData.volume, $settings.dolvol)}</span>
			<span class="label">ADR</span>
			<span class="value">{$hoveredCandleData.adr?.toFixed(2) ?? 'NA'}</span>
		</div>
	{/if}
</div>

<style>
	.legend {
		position: absolute;
		top: 5px;
		left: 10px;
		padding: 8px;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		color: #fff;
		z-index: 5;
		max-width: calc(100% - 20px);
		width: fit-content;
		min-width: min-content;
		user-select: none;
		text-shadow: 0 1px 2px rgb(0 0 0 / 80%);
		background: transparent;
		border: none;
	}

	/* Compact styles for small screens */
	.legend.compact {
		width: auto;
	}

	.ohlc-row {
		display: grid;
		grid-template-columns: 15px 50px 15px 50px 15px 50px 15px 50px;
		gap: 4px;
		align-items: center;
		margin-bottom: 4px;
		height: 20px;
	}

	.metrics-row {
		display: grid;
		grid-template-columns: 30px 100px 30px 60px 30px 40px;
		gap: 4px;
		align-items: center;
		height: 20px;
	}

	.label {
		font-size: 12px;
		line-height: 16px;
		color: rgb(255 255 255 / 70%);
		font-weight: 500;
		min-width: 35px;
		flex-shrink: 0;
		text-shadow: 0 1px 2px rgb(0 0 0 / 60%);
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
		color: #fff;
		text-shadow: 0 1px 2px rgb(0 0 0 / 80%);
	}

	/* Ensure legend stays within chart bounds */
	@media (width <= 400px) {
		.legend {
			width: calc(100% - 20px);
			min-width: 260px;
		}
	}
</style>
