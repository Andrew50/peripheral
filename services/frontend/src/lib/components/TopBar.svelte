<script lang="ts">
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import type { Instance } from '$lib/utils/types/types';
	import { streamInfo } from '$lib/utils/stores/stores';
	import { timeframeToSeconds } from '$lib/utils/helpers/timestamp';
	import { onMount, onDestroy } from 'svelte';
	import { writable } from 'svelte/store';

	export let instance: Instance;

	const commonTimeframes = ['1', '1h', '1d', '1w'];
	// Helper computed value to check if current timeframe is custom
	$: isCustomTimeframe = instance?.timeframe && !commonTimeframes.includes(instance.timeframe);

	// --- New Handlers for Buttons ---
	function handleTickerClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		queryInstanceInput([], ['ticker'], instance, 'ticker')
			.then((v: Instance) => {
				if (v) queryChart(v, true);
			})
			.catch((error) => {
				// Handle cancellation silently
				if (error.message !== 'User cancelled input') {
					console.error('Error in ticker input:', error);
				}
			});
	}
	function handleTickerKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			event.stopPropagation(); // Prevent legend collapse toggle
			queryInstanceInput('any', ['ticker'], instance, 'ticker')
				.then((v: Instance) => {
					if (v) queryChart(v, true);
				})
				.catch((error) => {
					// Handle cancellation silently
					if (error.message !== 'User cancelled input') {
						console.error('Error in ticker input:', error);
					}
				});
		}
	}
	function handleSessionClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation(); // Prevent legend collapse toggle
		if (instance) {
			const updatedInstance = { ...instance, extendedHours: !instance.extendedHours };
			queryChart(updatedInstance, true);
		}
	}
	// Function to handle clicking the "..." timeframe button
	function handleCustomTimeframeClick() {
		// Start with empty input but force timeframe type
		queryInstanceInput(['timeframe'], ['timeframe'], instance, 'timeframe')
			.then((v: Instance) => {
				if (v) queryChart(v, true);
			})
			.catch((error) => {
				// Handle cancellation silently
				if (error.message !== 'User cancelled input') {
					console.error('Error in timeframe input:', error);
				}
			});
	}

	// Function to handle selecting a preset timeframe button
	function selectTimeframe(newTimeframe: string) {
		if (instance && instance.timeframe !== newTimeframe) {
			const updatedInstance = { ...instance, timeframe: newTimeframe };
			queryChart(updatedInstance, true);
		}
	}

	// Function to handle calendar button click
	function handleCalendarClick() {
		// Dispatch a custom event that the parent can listen to
		const event = new CustomEvent('calendar-click');
		document.dispatchEvent(event);
	}
</script>

<div class="top-bar">
	<!-- Company Logo -->
	{#if instance?.logo}
		<div class="logo-container">
			<img
				src={instance.logo}
				alt="{instance?.name || 'Company'} logo"
				class="company-logo-topbar"
			/>
		</div>
	{/if}

	<button
		class="symbol metadata-button"
		on:click={handleTickerClick}
		on:keydown={handleTickerKeydown}
		aria-label="Change ticker"
	>
		<svg class="search-icon" viewBox="0 0 24 24" width="18" height="18" fill="none">
			<path
				d="M21 21L16.514 16.506L21 21ZM19 10.5C19 15.194 15.194 19 10.5 19C5.806 19 2 15.194 2 10.5C2 5.806 5.806 2 10.5 2C15.194 2 19 5.806 19 10.5Z"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
				stroke-linejoin="round"
			/>
		</svg>
		{instance?.ticker || 'NaN'}
	</button>

	<!-- Divider -->
	<div class="divider"></div>

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

	<!-- Divider -->
	<div class="divider"></div>

	<button
		class="session-type metadata-button"
		on:click={handleSessionClick}
		aria-label="Toggle session type"
	>
		{instance?.extendedHours ? 'Extended' : 'Regular'}
	</button>

	<!-- Divider -->
	<div class="divider"></div>

	<!-- Calendar button for timestamp selection -->
	<button
		class="calendar-button metadata-button"
		on:click={handleCalendarClick}
		title="Go to Date"
		aria-label="Go to Date"
	>
		<svg viewBox="0 0 24 24" width="16" height="16" fill="none" xmlns="http://www.w3.org/2000/svg">
			<path
				d="M19 3H18V1H16V3H8V1H6V3H5C3.89 3 3 3.9 3 5V19C3 20.1 3.89 21 5 21H19C20.11 21 21 20.1 21 19V5C21 3.9 20.11 3 19 3ZM19 19H5V8H19V19ZM7 10H12V15H7V10Z"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linecap="round"
				stroke-linejoin="round"
			/>
		</svg>
	</button>
</div>

<style>
	.top-bar {
		height: 40px;
		min-height: 40px;
		background-color: #0f0f0f;
		display: flex;
		justify-content: flex-start;
		align-items: center;
		padding: 0 10px;
		gap: 4px;
		flex-shrink: 0;
		width: 100%;
		z-index: 10;
		border-bottom: 4px solid var(--c1);
		position: absolute; /* Position absolutely */
		top: 0;
		left: 0;
		right: 0;
	}

	/* Base styles for metadata buttons */
	.metadata-button {
		font-family: inherit;
		font-size: 13px;
		line-height: 18px;
		color: rgba(255, 255, 255, 0.9);
		padding: 6px 10px;
		background: transparent;
		border-radius: 6px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		border: 1px solid transparent;
		cursor: pointer;
		transition: none;
		text-align: left;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.metadata-button:focus {
		outline: none;
		box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.4);
	}

	.metadata-button:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: transparent;
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	/* Specific style adjustments for symbol button */
	.symbol.metadata-button {
		font-size: 14px;
		line-height: 20px;
		color: #ffffff;
		padding: 6px 12px;
		gap: 4px;
	}

	.top-bar .search-icon {
		opacity: 0.8;
		transition: opacity 0.2s ease;
		position: static;
		padding: 0;
		left: auto;
	}

	.top-bar .symbol.metadata-button:hover .search-icon {
		opacity: 1;
	}

	/* Styles for preset timeframe buttons */
	.timeframe-preset-button {
		min-width: 24px;
		text-align: center;
		padding: 6px 4px;
		display: inline-flex;
		justify-content: center;
		align-items: center;
		margin-left: -2px; /* Reduce spacing between timeframe buttons */
	}

	.timeframe-preset-button:first-of-type {
		margin-left: 0; /* Don't apply negative margin to first timeframe button */
	}

	.timeframe-preset-button.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Styles for the custom timeframe '...' button */
	.timeframe-custom-button {
		padding: 6px 4px;
		min-width: 24px;
		text-align: center;
		display: inline-flex;
		justify-content: center;
		align-items: center;
		margin-left: -2px; /* Reduce spacing with preceding timeframe buttons */
	}

	.timeframe-custom-button.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
	}

	/* Logo styles */
	.logo-container {
		height: 24px;
		max-width: 80px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		margin-right: 8px;
		overflow: hidden;
	}

	.company-logo-topbar {
		height: 100%;
		max-width: 100%;
		object-fit: contain;
		filter: brightness(0.9);
		transition: filter 0.2s ease;
	}

	.company-logo-topbar:hover {
		filter: brightness(1);
	}

	/* Calendar button styles */
	.calendar-button {
		padding: 6px 8px;
		min-width: auto;
		display: inline-flex;
		justify-content: center;
		align-items: center;
	}

	.calendar-button svg {
		opacity: 0.8;
		transition: opacity 0.2s ease;
	}

	.calendar-button:hover svg {
		opacity: 1;
	}

	/* Divider styles */
	.divider {
		width: 1px;
		height: 28px;
		background: rgba(255, 255, 255, 0.15);
		margin: 0 6px;
		flex-shrink: 0;
	}
</style>
