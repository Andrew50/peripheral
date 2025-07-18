<script lang="ts">
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryChart, activeChartInstance } from '$lib/features/chart/interface';
	import type { Instance } from '$lib/utils/types/types';
	import { streamInfo } from '$lib/utils/stores/stores';
	import { timeframeToSeconds } from '$lib/utils/helpers/timestamp';
	import { onMount, onDestroy } from 'svelte';
	import { writable } from 'svelte/store';


	
	export let instance: Instance;
	export let handleCalendar: () => void;
	const commonTimeframes = ['1', '1h', '1d', '1w'];
	let countdown = writable('--');
	let countdownInterval: ReturnType<typeof setInterval>;
	// Helper computed value to check if current timeframe is custom
	$: isCustomTimeframe = instance?.timeframe && !commonTimeframes.includes(instance.timeframe);

	// TopBar handler functions
	function handleTickerClick(event: MouseEvent | TouchEvent) {
	event.preventDefault();
	event.stopPropagation();
	queryInstanceInput([], ['ticker'], $activeChartInstance || {}, 'ticker', 'Symbol Search - TopBar')
		.then((v: Instance) => {
			if (v) queryChart(v, true);
		})
		.catch((error) => {
			if (error.message !== 'User cancelled input') {
				console.error('Error in ticker input:', error);
			}
		});
	}
	function handleTickerKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			event.stopPropagation();
			queryInstanceInput('any', ['ticker'], $activeChartInstance || {}, 'ticker', 'Symbol Search - TopBar')
				.then((v: Instance) => {
					if (v) queryChart(v, true);
				})
				.catch((error) => {
					if (error.message !== 'User cancelled input') {
						console.error('Error in ticker input:', error);
					}
				});
		}
	}
	function handleSessionClick(event: MouseEvent | TouchEvent) {
		event.preventDefault();
		event.stopPropagation();
		if ($activeChartInstance) {
			const updatedInstance = {
				...$activeChartInstance,
				extendedHours: !$activeChartInstance.extendedHours
			};
			queryChart(updatedInstance, true);
		}
	}
	// Function to handle clicking the "..." timeframe button
	function handleCustomTimeframeClick() {
		queryInstanceInput(['timeframe'], ['timeframe'], $activeChartInstance || undefined, 'timeframe')
			.then((v: Instance) => {
				if (v) queryChart(v, true);
			})
			.catch((error) => {
				if (error.message !== 'User cancelled input') {
					console.error('Error in timeframe input:', error);
				}
			});
	}

	function selectTimeframe(newTimeframe: string) {
		if ($activeChartInstance && $activeChartInstance.timeframe !== newTimeframe) {
			const updatedInstance = { ...$activeChartInstance, timeframe: newTimeframe };
			queryChart(updatedInstance, true);
		}
	}

	function formatTime(seconds: number): string {
		const years = Math.floor(seconds / (365 * 24 * 60 * 60));
		const months = Math.floor((seconds % (365 * 24 * 60 * 60)) / (30 * 24 * 60 * 60));
		const weeks = Math.floor((seconds % (30 * 24 * 60 * 60)) / (7 * 24 * 60 * 60));
		const days = Math.floor((seconds % (7 * 24 * 60 * 60)) / (24 * 60 * 60));
		const hours = Math.floor((seconds % (24 * 60 * 60)) / (60 * 60));
		const minutes = Math.floor((seconds % (60 * 60)) / 60);
		const secs = Math.floor(seconds % 60);

		if (years > 0) return `${years}y ${months}m`;
		if (months > 0) return `${months}m ${weeks}w`;
		if (weeks > 0) return `${weeks}w ${days}d`;
		if (days > 0) return `${days}d ${hours}h`;
		if (hours > 0) return `${hours}h ${minutes}m`;
		if (minutes > 0) return `${minutes}m ${secs < 10 ? '0' : ''}${secs}s`;
		return `${secs < 10 ? '0' : ''}${secs}s`;
	}

	function calculateCountdown() {
		if (!instance?.timeframe) {
			countdown.set('--');
			return;
		}

		const currentTimeInSeconds = Math.floor($streamInfo.timestamp / 1000);
		const chartTimeframeInSeconds = timeframeToSeconds(instance.timeframe);

		let nextBarClose =
			currentTimeInSeconds -
			(currentTimeInSeconds % chartTimeframeInSeconds) +
			chartTimeframeInSeconds;

		// For daily timeframes, adjust to market close (4:00 PM EST)
		if (instance.timeframe.includes('d')) {
			const currentDate = new Date(currentTimeInSeconds * 1000);
			const estOptions = { timeZone: 'America/New_York' };
			const formatter = new Intl.DateTimeFormat('en-US', {
				...estOptions,
				year: 'numeric',
				month: 'numeric',
				day: 'numeric'
			});

			const [month, day, year] = formatter.format(currentDate).split('/');

			const marketCloseDate = new Date(
				`${year}-${month.padStart(2, '0')}-${day.padStart(2, '0')}T16:00:00-04:00`
			);

			nextBarClose = Math.floor(marketCloseDate.getTime() / 1000);

			if (currentTimeInSeconds >= nextBarClose) {
				marketCloseDate.setDate(marketCloseDate.getDate() + 1);

				const dayOfWeek = marketCloseDate.getDay(); // 0 = Sunday, 6 = Saturday
				if (dayOfWeek === 0) {
					// Sunday
					marketCloseDate.setDate(marketCloseDate.getDate() + 1); // Move to Monday
				} else if (dayOfWeek === 6) {
					// Saturday
					marketCloseDate.setDate(marketCloseDate.getDate() + 2); // Move to Monday
				}

				nextBarClose = Math.floor(marketCloseDate.getTime() / 1000);
			}
		}

		const remainingTime = nextBarClose - currentTimeInSeconds;

		if (remainingTime > 0) {
			countdown.set(formatTime(remainingTime));
		} else {
			countdown.set('Bar Closed');
		}
	}

	onMount(() => {
		countdownInterval = setInterval(calculateCountdown, 1000);
		calculateCountdown(); // Initial calculation

	});

	onDestroy(() => {
		if (countdownInterval) {
			clearInterval(countdownInterval);
		}
	});
</script>

<div class="top-bar">
	<!-- Left side content -->
	<div class="top-bar-left">
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
			{$activeChartInstance?.ticker || 'NaN'}
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<!-- Add common timeframe buttons -->
		{#each commonTimeframes as tf}
			<button
				class="timeframe-preset-button metadata-button {$activeChartInstance?.timeframe ===
				tf
					? 'active'
					: ''}"
				on:click={() => selectTimeframe(tf)}
				aria-label="Set timeframe to {tf}"
				aria-pressed={$activeChartInstance?.timeframe === tf}
			>
				{tf}
			</button>
		{/each}
		<!-- Button to open custom timeframe input -->
		<button
			class="timeframe-custom-button metadata-button {isCustomTimeframe
				? 'active'
				: ''}"
			on:click={handleCustomTimeframeClick}
			aria-label="Select custom timeframe"
			aria-pressed={isCustomTimeframe ? 'true' : 'false'}
		>
			{#if isCustomTimeframe}
				{$activeChartInstance?.timeframe}
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
			{$activeChartInstance?.extendedHours ? 'Extended' : 'Regular'}
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<!-- Calendar button for timestamp selection -->
		<button
			class="calendar-button metadata-button"
			on:click={handleCalendar}
			title="Go to Date"
			aria-label="Go to Date"
		>
			<svg
				viewBox="0 0 24 24"
				width="16"
				height="16"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
			>
				<path
					d="M19 3H18V1H16V3H8V1H6V3H5C3.89 3 3 3.9 3 5V19C3 20.1 3.89 21 5 21H19C20.11 21 21 20.1 21 19V5C21 3.9 20.11 3 19 3ZM19 19H5V8H19V19ZM7 10H12V15H7V10Z"
					stroke="currentColor"
					stroke-width="1.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</button>

		<!-- Divider -->
		<div class="divider"></div>

		<!-- Countdown -->
		<div class="countdown-container">
			<span class="countdown-label">Next Bar Close:</span>
			<span class="countdown-value">{$countdown}</span>
		</div>
	</div>
</div>

<style>
	/* TopBar styles */
	.top-bar {
	height: 40px;
	min-height: 40px;
	background-color: #121212;
	display: flex;
	align-items: center;
	padding: 0 10px;
	flex-shrink: 0;
	width: 100%;
	z-index: 10;
	border-bottom: 4px solid var(--c1);
	}

	.top-bar-left {
		display: flex;
		align-items: center;
		gap: 4px;
		flex: 1;
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
		margin-left: -2px;
	}

	.timeframe-preset-button:first-of-type {
		margin-left: 0;
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
		margin-left: -2px;
	}

	.timeframe-custom-button.active {
		background: rgba(255, 255, 255, 0.2);
		border-color: transparent;
		color: #ffffff;
		font-weight: 600;
		box-shadow: 0 2px 8px rgba(255, 255, 255, 0.2);
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

	/* Countdown styles */
	.countdown-container {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		background: transparent;
		border-radius: 6px;
		border: none;
		color: rgba(255, 255, 255, 0.9);
		font-size: 13px;
		line-height: 18px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
		transition: none;
	}

	.countdown-container:hover {
		background: rgba(255, 255, 255, 0.15);
		color: #ffffff;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
	}

	.countdown-label {
		color: inherit;
		font-size: inherit;
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}

	.countdown-value {
		font-family: inherit;
		font-weight: 600;
		font-size: inherit;
		color: inherit;
		min-width: 45px;
		text-align: center;
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
