<script lang="ts">
	import L1 from './l1.svelte';
	import TimeAndSales from './timeAndSales.svelte';
	import { get, writable, type Writable } from 'svelte/store';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import type { Instance } from '$lib/utils/types/types';
	import { activeChartInstance, queryChart } from '$lib/features/chart/interface';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { streamInfo, formatTimestamp } from '$lib/utils/stores/stores';
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import {
		UTCSecondstoESTSeconds,
		ESTSecondstoUTCSeconds,
		ESTSecondstoUTCMillis,
		getReferenceStartTimeForDateMilliseconds,
		timeframeToSeconds
	} from '$lib/utils/helpers/timestamp';
	import { getExchangeName } from '$lib/utils/helpers/exchanges';

	let instance: Writable<Instance> = writable({});
	let container: HTMLButtonElement;
	let showTimeAndSales = false;
	let currentDetails: Record<string, any> = {};
	let lastFetchedSecurityId: number | null = null;
	let countdown = writable('--');
	let countdownInterval: ReturnType<typeof setInterval>;
	let logoLoadError = false;

	// Sync instance with activeChartInstance and handle details fetching
	activeChartInstance.subscribe((chartInstance: Instance | null) => {
		if (chartInstance?.ticker) {
			instance.set(chartInstance);

			// Reset logo error state when instance changes
			logoLoadError = false;

			// Handle details fetching in the main subscription
			if (chartInstance.securityId && lastFetchedSecurityId !== chartInstance.securityId) {
				lastFetchedSecurityId = Number(chartInstance.securityId);
				privateRequest<Record<string, any>>(
					'getTickerMenuDetails',
					{ securityId: chartInstance.securityId },
					true
				)
					.then((details) => {
						if (lastFetchedSecurityId === Number(chartInstance.securityId)) {
							currentDetails = details;
							// Update the instance directly instead of activeChartInstance
							instance.update((inst) => ({
								...inst,
								...details
							}));
						}
					})
					.catch((error) => {
						console.error('Quote component: Error fetching details:', error);
						if (lastFetchedSecurityId === Number(chartInstance.securityId)) {
							currentDetails = {};
						}
					});
			}
		}
	});

	function handleKey(event: KeyboardEvent) {
		// Example: if user presses tab or alphanumeric, prompt ticker change
		if (event.key == 'Tab' || /^[a-zA-Z0-9]$/.test(event.key)) {
			const current = get(instance);
			queryInstanceInput(['ticker'], ['ticker'], current)
				.then((updated: Instance) => {
					instance.set(updated);
				})
				.catch(() => {});
		}
	}

	function toggleTimeAndSales() {
		showTimeAndSales = !showTimeAndSales;
	}

	function handleClick(event?: MouseEvent | TouchEvent) {
		if ($activeChartInstance) {
			queryChart($activeChartInstance);
		}
	}

	$: if (container) {
		container.addEventListener('keydown', handleKey);
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
		const currentInst = get(instance);
		if (!currentInst?.timeframe) {
			countdown.set('--');
			return;
		}

		const currentTimeInSeconds = Math.floor($streamInfo.timestamp / 1000);
		const chartTimeframeInSeconds = timeframeToSeconds(currentInst.timeframe);

		let nextBarClose =
			currentTimeInSeconds -
			(currentTimeInSeconds % chartTimeframeInSeconds) +
			chartTimeframeInSeconds;

		// For daily timeframes, adjust to market close (4:00 PM EST)
		if (currentInst.timeframe.includes('d')) {
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
		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);
		countdownInterval = setInterval(calculateCountdown, 1000);

		return () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
			clearInterval(countdownInterval);
		};
	});

	function handleMouseMove(e: MouseEvent | TouchEvent) {
		// This function is now empty as the height-related variables and functions are removed
	}

	function handleMouseUp() {
		// This function is now empty as the height-related variables and functions are removed
	}

	function handleLogoError() {
		logoLoadError = true;
		// Remove console.error for failed logo loading
	}
</script>

<button
	class="ticker-info-container"
	bind:this={container}
	aria-label="Ticker Information"
	on:click={handleClick}
	on:touchstart={handleClick}
>
	<div class="content">
		<!-- Header Section -->
		<div class="quote-header glass glass--rounded glass--medium">
			{#if ($instance?.logo || currentDetails?.logo) && !logoLoadError}
				<div class="logo-container">
					<img
						src={$instance?.logo || currentDetails?.logo}
						alt="{$instance?.name || currentDetails?.name || 'Company'} logo"
						class="company-logo"
						on:error={handleLogoError}
					/>
				</div>
			{:else if $instance?.ticker || currentDetails?.ticker}
				<div class="logo-container fallback-logo">
					<div class="ticker-logo">
						{($instance?.ticker || currentDetails?.ticker || '').charAt(0)}
					</div>
				</div>
			{/if}
			<div class="ticker-wrapper">
				<div class="ticker">{$instance.ticker || '--'}</div>
				{#if ($instance?.active === false || currentDetails?.active === false)}
					<div class="warning-triangle-container">
						<div class="warning-triangle"></div>
						<div class="tooltip">Delisted</div>
					</div>
				{/if}
			</div>
			<div class="company-info">
				<div class="name">{$instance?.name || currentDetails?.name || 'N/A'}</div>
			</div>
		</div>

		<!-- Key Metrics Section -->
		<div class="quote-key-metrics glass glass--rounded glass--medium">
			<div class="metric-item glass glass--small glass--light">
				<span class="label">Price</span>
				<StreamCell instance={$instance} type="price" />
			</div>
			<div class="metric-item glass glass--small glass--light">
				<span class="label">Change %</span>
				<StreamCell instance={$instance} type="change %" />
			</div>
			<div class="metric-item glass glass--small glass--light">
				<span class="label">Change</span>
				<StreamCell instance={$instance} type="change" />
			</div>
			<div class="metric-item glass glass--small glass--light">
				<span class="label">Ext %</span>
				<StreamCell instance={$instance} type="change % extended" />
			</div>
		</div>

		<!-- Market Data Section -->
		<div class="quote-market-data glass glass--rounded glass--medium">
			<L1 {instance} />
			<!--
			<button class="time-sales-button" on:click|stopPropagation={toggleTimeAndSales}>
				{showTimeAndSales ? 'Hide Time & Sales' : 'Show Time & Sales'}
			</button>
			{#if showTimeAndSales}
				<TimeAndSales {instance} />
			{/if} -->
		</div>

		<!-- Details Section -->
		<div class="quote-details glass glass--rounded glass--medium">
			<div class="detail-item">
				<span class="label">Market Cap:</span>
				<span class="value">
					{#if $instance?.totalShares || currentDetails?.totalShares}
						<StreamCell instance={$instance} type="market cap" />
					{:else}
						N/A
					{/if}
				</span>
			</div>
			<div class="detail-item">
				<span class="label">Sector:</span>
				<span class="value">{$instance?.sector || currentDetails?.sector || 'N/A'}</span>
			</div>
			<div class="detail-item">
				<span class="label">Industry:</span>
				<span class="value">{$instance?.industry || currentDetails?.industry || 'N/A'}</span>
			</div>
			<div class="detail-item">
				<span class="label">Exchange:</span>
				<span class="value"
					>{ getExchangeName($instance?.primary_exchange || currentDetails?.primary_exchange) }</span
				>
			</div>
			<div class="detail-item">
				<span class="label">Market:</span>
				<span class="value">{$instance?.market || currentDetails?.market || 'N/A'}</span>
			</div>
			<div class="detail-item">
				<span class="label">Shares Out:</span>
				<span class="value">
					{#if $instance?.share_class_shares_outstanding || currentDetails?.share_class_shares_outstanding}
						{(
							($instance?.share_class_shares_outstanding ||
								currentDetails?.share_class_shares_outstanding) / 1e6
						).toFixed(2)}M
					{:else}
						N/A
					{/if}
				</span>
			</div>
		</div>

		<!-- Countdown Section -->
		<div class="countdown-section glass glass--rounded glass--medium">
				<div class="countdown-container glass glass--small glass--light">
					<span class="countdown-label">Next Bar Close:</span>
					<span class="countdown-value">{$countdown}</span>
				</div>
			</div>

		<!-- Description Section -->
		{#if $activeChartInstance?.description}
			<div class="description glass glass--rounded glass--medium">
				<span class="label">Description:</span>
				<p class="value description-text">{$activeChartInstance?.description}</p>
			</div>
		{/if}
	</div>
</button>

<style>
	.ticker-info-container {
		background: var(--ui-bg-primary);
		border-top: 1px solid var(--ui-border);
		font-family: var(--font-primary);
		height: 100%;
		width: 100%;
		padding: 0;
		margin: 0;
		text-align: left;
		border: none;
		cursor: pointer;
		display: flex;
		flex-direction: column;
	}

	.content {
		padding: 12px;
		overflow-y: auto;
		scrollbar-width: thin;
		scrollbar-color: var(--ui-border) transparent;
		-ms-overflow-style: none;
		flex-grow: 1;
		color: var(--text-primary);
	}

	.content::-webkit-scrollbar {
		width: 4px;
	}
	.content::-webkit-scrollbar-thumb {
		background-color: var(--ui-border);
		border-radius: 2px;
	}
	.content::-webkit-scrollbar-track {
		background: transparent;
	}

	/* Header */
	.quote-header {
		display: flex;
		align-items: center;
		justify-content: flex-start;
		gap: 10px;
		margin-bottom: 16px;
		padding: 12px;
	}

	.logo-container {
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: white;
		padding: 4px;
		border-radius: 4px;
		width: 32px;
		height: 32px;
	}

	.company-logo {
		max-height: 100%;
		max-width: 100%;
		object-fit: contain;
		display: block;
	}

	.fallback-logo {
		background: var(--ui-bg-secondary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
	}

	.ticker-logo {
		width: 100%;
		height: 100%;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 14px;
		font-weight: bold;
		text-transform: uppercase;
		background: var(--ui-bg-primary);
		color: var(--text-primary);
	}

	.ticker-wrapper {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.ticker {
		font-size: 1.4em;
		font-weight: 700;
		color: var(--text-primary);
		text-transform: uppercase;
		line-height: 1.1;
	}

	.warning-triangle-container {
		position: relative;
		display: flex;
		align-items: center;
	}

	.warning-triangle {
		width: 0;
		height: 0;
		border-left: 10px solid transparent;
		border-right: 10px solid transparent;
		border-bottom: 16px solid #ff4444;
		cursor: pointer;
		transition: transform 0.15s ease;
		position: relative;
	}

	.warning-triangle::after {
		content: '';
		position: absolute;
		top: 3px;
		left: -7px;
		width: 0;
		height: 0;
		border-left: 7px solid transparent;
		border-right: 7px solid transparent;
		border-bottom: 11px solid var(--ui-bg-primary);
	}

	.tooltip {
		position: absolute;
		bottom: 100%;
		left: 50%;
		transform: translateX(-50%);
		background: var(--ui-bg-secondary);
		color: var(--text-primary);
		padding: 6px 8px;
		border-radius: 4px;
		font-size: 0.75em;
		font-weight: 500;
		white-space: nowrap;
		border: 1px solid var(--ui-border);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
		opacity: 0;
		visibility: hidden;
		transition: opacity 0.2s ease, visibility 0.2s ease;
		margin-bottom: 4px;
		z-index: 1000;
	}

	.warning-triangle-container:hover .warning-triangle {
		transform: scale(1.1);
	}

	.warning-triangle-container:hover .tooltip {
		opacity: 1;
		visibility: visible;
	}

	.company-info {
		display: flex;
		flex-direction: column;
		min-width: 0;
		flex-grow: 1;
	}

	.name {
		font-size: 0.85em;
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		line-height: 1.2;
		font-weight: 500;
	}

	/* Key Metrics */
	.quote-key-metrics {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(75px, 1fr));
		gap: 8px;
		margin-bottom: 16px;
		padding: 12px;
	}

	.metric-item {
		padding: 8px 6px;
		text-align: center;
	}

	.metric-item .label {
		font-size: 0.7em;
		color: var(--text-secondary);
		display: block;
		margin-bottom: 4px;
		text-transform: uppercase;
		font-weight: 500;
	}

	.metric-item :global(.value) {
		font-size: 0.95em;
		font-weight: 600;
		display: block;
		line-height: 1.1;
	}

	/* Market Data */
	.quote-market-data {
		margin-bottom: 16px;
		padding: 12px;
	}

	/*.time-sales-button {
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		padding: 6px 10px;
		font-size: 0.8em;
		cursor: pointer;
		transition: background-color 0.15s ease;
		margin: 8px 0;
		width: 100%;
		font-weight: 500;
		display: flex;
		align-items: center;
		justify-content: center;
	}*/

	/* Details */
	.quote-details {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 8px 12px;
		margin-bottom: 16px;
		padding: 12px;
	}

	.detail-item {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		font-size: 0.8em;
		padding: 4px 0;
	}

	.detail-item .label {
		color: var(--text-secondary);
		margin-right: 8px;
		white-space: nowrap;
		font-weight: 500;
	}

	.detail-item .value {
		color: var(--text-primary);
		text-align: right;
		font-weight: 500;
	}

	/* Countdown */
	.countdown-section {
		margin-top: 12px;
		padding: 12px;
	}

	.countdown-container {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 8px 12px;
	}

	.countdown-label {
		color: var(--text-secondary);
		font-size: 0.75em;
		font-weight: 500;
		text-transform: uppercase;
	}

	.countdown-value {
		font-family: var(--font-primary);
		font-weight: 600;
		font-size: 0.8em;
		color: var(--text-primary);
		padding: 4px 8px;
		background: var(--ui-bg-secondary);
		border-radius: 3px;
		min-width: 60px;
		text-align: center;
		border: 1px solid var(--ui-border);
	}

	/* Description */
	.description {
		margin-top: 16px;
		padding: 12px;
	}

	.description .label {
		display: block;
		color: var(--text-secondary);
		font-size: 0.8em;
		margin-bottom: 6px;
		font-weight: 500;
		text-transform: uppercase;
	}

	.description-text {
		font-size: 0.8em;
		line-height: 1.4;
		color: var(--text-secondary);
	}

	/* Responsive adjustments */
	@media (max-width: 400px) {
		.quote-key-metrics {
			grid-template-columns: repeat(2, 1fr);
		}
		
		.quote-details {
			grid-template-columns: 1fr;
		}
	}
</style>
