<script lang="ts">
	export let hoveredCandleData;
	import type { Instance } from '$lib/utils/types/types';
	import { queryChart } from './interface';
	import { writable } from 'svelte/store';
	export let instance: Instance;
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { settings } from '$lib/utils/stores/stores';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import { onMount, onDestroy } from 'svelte';

	// Add new props for chart dimensions
	export let width: number;

	let legendElement: HTMLDivElement;
	let isOverflowing = false;
	let showPriceGrid = true;
	let showMetricsGrid = true;
	let resizeObserver: ResizeObserver;
	let checkOverflowTimeout: ReturnType<typeof setTimeout>;

	let isCollapsed = false;
	let isUpdating = false;

	// Common preset timeframes to display as buttons
	const commonTimeframes = ['1', '1h', '1d', '1w'];

	// Helper computed value to check if current timeframe is custom
	$: isCustomTimeframe = instance?.timeframe && !commonTimeframes.includes(instance.timeframe);

	function toggleCollapse() {
		isCollapsed = !isCollapsed;
	}

	// --- New Handlers for Buttons ---
	function handleTickerClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		queryInstanceInput([], ['ticker'], instance).then((v: Instance) => {
			if (v) queryChart(v, true);
		});
	}

	function handleTickerKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			event.stopPropagation(); // Prevent legend collapse toggle
			queryInstanceInput('any', ['ticker'], instance).then((v: Instance) => {
				if (v) queryChart(v, true);
			});
		}
	}

	function handleTimestampClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		queryInstanceInput([], ['timestamp'], instance).then((v: Instance) => {
			if (v) queryChart(v, true);
		});
	}

	function handleSessionClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		if (instance) {
			const updatedInstance = { ...instance, extendedHours: !instance.extendedHours };
			queryChart(updatedInstance, true);
		}
	}
	// --- End New Handlers ---

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

	// Function to handle selecting a preset timeframe button
	function selectTimeframe(newTimeframe: string) {
		if (instance && instance.timeframe !== newTimeframe) {
			const updatedInstance = { ...instance, timeframe: newTimeframe };
			queryChart(updatedInstance, true);
		}
		// No need to hide selector anymore
	}

	// Function to handle clicking the "..." timeframe button
	function handleCustomTimeframeClick() {
		// No selector to hide
		queryInstanceInput([], ['timeframe'], instance).then((v: Instance) => {
			if (v) queryChart(v, true);
		});
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

<div
	bind:this={legendElement}
	tabindex="-1"
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
		<button
			class="symbol metadata-button"
			on:click={handleTickerClick}
			on:keydown={handleTickerKeydown}
			aria-label="Change ticker"
		>
			<svg class="search-icon" viewBox="0 0 24 24" width="12" height="12" fill="currentColor">
				<path d="M9.5,3A6.5,6.5 0 0,1 16,9.5C16,11.11 15.41,12.59 14.44,13.73L14.71,14H15.5L21.5,20L20,21.5L14,15.5V14.71L13.73,14.44C12.59,15.41 11.11,16 9.5,16A6.5,6.5 0 0,1 3,9.5A6.5,6.5 0 0,1 9.5,3M9.5,5C7,5 5,7 5,9.5C5,12 7,14 9.5,14C12,14 14,12 14,9.5C14,7 12,5 9.5,5Z" />
			</svg>
			{instance?.ticker || 'NaN'}
		</button>
		<span class="metadata">
			<!-- Add common timeframe buttons -->
			{#each commonTimeframes as tf}
				<button
					class="timeframe-preset-button metadata-button {instance?.timeframe === tf ? 'active' : ''}"
					on:click={() => selectTimeframe(tf)}
					aria-label="Set timeframe to {tf}"
					aria-pressed={instance?.timeframe === tf}
				>
					{tf}
				</button>
			{/each}
			<!-- Button to open custom timeframe input -->
			<button
				class="timeframe-custom-button metadata-button {isCustomTimeframe ? 'active' : ''}"
				on:click={handleCustomTimeframeClick}
				aria-label="Select custom timeframe"
				aria-pressed={isCustomTimeframe ? 'true' : 'false'}
			>
				{#if isCustomTimeframe}
					{instance.timeframe}
				{:else}
					...
				{/if}
			</button>

			{#if !isOverflowing}
				<button
					class="timestamp metadata-button"
					on:click={handleTimestampClick}
					aria-label="Change timestamp"
				>
					{UTCTimestampToESTString(instance?.timestamp ?? 0)}
				</button>
				<button
					class="session-type metadata-button"
					on:click={handleSessionClick}
					aria-label="Toggle session type"
				>
					{instance?.extendedHours ? 'Extended' : 'Regular'}
				</button>
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
				<!-- <div class="metric">
					<span class="label">RVOL</span>
					<span class="value">{$hoveredCandleData.rvol?.toFixed(2) ?? 'NA'}</span>
				</div> -->
			</div>
		{/if}
	{/if}

	<div 
		class="collapse-row" 
		on:click|stopPropagation={toggleCollapse}
		role="button"
		tabindex="0"
		on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { toggleCollapse(); e.preventDefault(); } }}
		aria-expanded={!isCollapsed}
	>
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
		background-color: rgba(255, 255, 255, 0.1);
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
	}

	/* Update hover styles to not affect width/layout */
	.legend:hover {
		background-color: rgba(255, 255, 255, 0.1);
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
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.metadata {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		overflow: hidden;
		min-width: 0;
	}

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

	.search-icon {
		opacity: 0.6;
		transition: opacity 0.2s ease;
	}

	/* Adjust hover for search icon on button */
	.symbol.metadata-button:hover .search-icon {
		opacity: 1;
	}

	/* Base styles for new metadata buttons */
	.metadata-button {
		font-family: inherit; /* Inherit font from legend */
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
		cursor: pointer;
		transition: background-color 0.2s ease, border-color 0.2s ease;
		text-align: left; /* Ensure text alignment */
		display: inline-flex; /* For alignment with potential icons */
		align-items: center; /* Align text/icons vertically */
		gap: 4px; /* Gap for icons if added */
	}

	/* Remove default button appearance */
	.metadata-button:focus {
		outline: none; /* Remove default focus outline */
		box-shadow: 0 0 0 2px var(--accent-color); /* Add custom focus ring */
	}

	.metadata-button:hover {
		background-color: var(--ui-bg-element-hover);
		border-color: var(--ui-border-hover);
		color: var(--text-primary); /* Slightly brighten text on hover */
	}

	/* Specific style adjustments for symbol button */
	.symbol.metadata-button {
		font-size: 14px;
		line-height: 20px;
		font-weight: 600;
		color: var(--text-primary);
		padding: 4px 10px; /* Keep original padding */
		gap: 4px; /* Ensure gap for icon */
	}

	/* Adjust original span styles to target buttons */
	.timestamp.metadata-button,
	.session-type.metadata-button {
		/* Styles previously applied to .timeframe, .timestamp, .session-type spans now applied via .metadata-button base class */
	}

	.timestamp.metadata-button {
		font-family: monospace;
	}

	/* Styles for preset timeframe buttons */
	.timeframe-preset-button {
		min-width: 30px; /* Ensure buttons have some width */
		text-align: center;
		padding: 4px 6px; /* Adjust padding */
		display: inline-flex;
		justify-content: center;
		align-items: center;
	}

	.timeframe-preset-button.active {
		background-color: var(--accent-color-translucent);
		border-color: var(--accent-color);
		color: var(--accent-color);
		font-weight: 500;
	}

	/* Styles for the custom timeframe '...' button */
	.timeframe-custom-button {
		padding: 4px 6px;
		min-width: 30px; /* Give it similar min-width */
		text-align: center;
		/* Add flex properties for robust centering */
		display: inline-flex;
		justify-content: center;
		align-items: center;
	}

	/* Apply active styles also to the custom button when it's showing a custom value */
	.timeframe-custom-button.active {
		background-color: var(--accent-color-translucent);
		border-color: var(--accent-color);
		color: var(--accent-color);
		font-weight: 500;
	}
</style>
