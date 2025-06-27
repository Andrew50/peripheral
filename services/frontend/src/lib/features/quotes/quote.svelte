<script lang="ts">
	import L1 from './l1.svelte';
	import TimeAndSales from './timeAndSales.svelte';
	import { get, writable, type Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';
	import { activeChartInstance } from '$lib/features/chart/interface';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { streamInfo, formatTimestamp } from '$lib/utils/stores/stores';
	import { onMount, onDestroy } from 'svelte';
	import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
	import {
		UTCSecondstoESTSeconds,
		ESTSecondstoUTCSeconds,
		ESTSecondstoUTCMillis,
		getReferenceStartTimeForDateMilliseconds,
		timeframeToSeconds
	} from '$lib/utils/helpers/timestamp';
	import { getExchangeName } from '$lib/utils/helpers/exchanges';
	import { showAuthModal } from '$lib/stores/authModal';
	import {
		isPublicViewing,
		watchlists,
		flagWatchlistId,
		flagWatchlist,
		currentWatchlistId,
		currentWatchlistItems
	} from '$lib/utils/stores/stores';

	let instance: Writable<Instance> = writable({});
	let container: HTMLDivElement;
	let showTimeAndSales = false;
	let currentDetails: Record<string, any> = {};
	let lastFetchedSecurityId: number | null = null;
	let countdown = writable('--');
	let countdownInterval: ReturnType<typeof setInterval>;
	// Sync instance with activeChartInstance and handle details fetching
	activeChartInstance.subscribe((chartInstance: Instance | null) => {
		if (chartInstance?.ticker) {
			instance.set(chartInstance);

			// Handle details fetching in the main subscription
			if (chartInstance.securityId && lastFetchedSecurityId !== chartInstance.securityId) {
				lastFetchedSecurityId = Number(chartInstance.securityId);
				publicRequest<Record<string, any>>('getTickerMenuDetails', {
					securityId: chartInstance.securityId
				})
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
		countdownInterval = setInterval(calculateCountdown, 1000);

		// Initialize with flagWatchlistId if available
		if (flagWatchlistId !== undefined) {
			currentWatchlistId.set(flagWatchlistId);
		}

		// Subscribe to watchlists to set initial watchlist if none selected
		const unsubscribeWatchlists = watchlists.subscribe((list) => {
			const currentValue = get(currentWatchlistId);
			if (
				Array.isArray(list) &&
				list.length > 0 &&
				(currentValue === undefined || isNaN(currentValue))
			) {
				currentWatchlistId.set(list[0].watchlistId);
			}
		});

		return () => {
			clearInterval(countdownInterval);
			unsubscribeWatchlists();
		};
	});

	function cleanCompanyName(name: string): string {
		if (!name || name === 'N/A') return name;

		// Remove common stock class designations
		return name
			.replace(/\s+(Class\s+[A-Z]+\s+)?Common\s+Stock$/i, '')
			.replace(/\s+Class\s+[A-Z]+\s+Shares?$/i, '')
			.replace(/\s+Class\s+[A-Z]+$/i, '')
			.replace(/\s+Common\s+Shares?$/i, '')
			.replace(/\s+Ordinary\s+Shares?$/i, '')
			.trim();
	}

	function addToWatchlist() {
		if (get(isPublicViewing)) {
			showAuthModal('watchlists', 'signup');
			return;
		}

		const currentInstance = get(instance);
		if (!currentInstance?.securityId || !currentInstance?.ticker) {
			return;
		}

		const watchlistsValue = get(watchlists);
		if (!Array.isArray(watchlistsValue) || watchlistsValue.length === 0) {
			alert('No watchlists available. Please create a watchlist first.');
			return;
		}

		// Use currently selected watchlist or fall back to first available
		const currentWatchlistIdValue = get(currentWatchlistId);
		const watchlistId = currentWatchlistIdValue || watchlistsValue[0]?.watchlistId;

		privateRequest<number>('newWatchlistItem', {
			watchlistId: watchlistId,
			securityId: currentInstance.securityId
		})
			.then((watchlistItemId: number) => {
				const targetWatchlist = watchlistsValue.find((w) => w.watchlistId === watchlistId);
				console.log(
					`Added ${currentInstance.ticker} to ${targetWatchlist?.watchlistName || 'watchlist'}`
				);

				// Update the appropriate store with the new item (same logic as watchlist component)
				const newItem = {
					...currentInstance,
					watchlistItemId: watchlistItemId
				};

				// Update the appropriate global stores
				if (watchlistId === flagWatchlistId) {
					// Update the global flagWatchlist store
					flagWatchlist.update((items) => {
						const currentItems = Array.isArray(items) ? items : [];
						// Check if item already exists to avoid duplicates
						if (!currentItems.find((item) => item.ticker === newItem.ticker)) {
							return [...currentItems, newItem];
						}
						return currentItems;
					});
				}

				// Also update currentWatchlistItems if this is the currently selected watchlist
				const currentWatchlistIdValue = get(currentWatchlistId);
				if (watchlistId === currentWatchlistIdValue) {
					currentWatchlistItems.update((items) => {
						const currentItems = Array.isArray(items) ? items : [];
						// Check if item already exists to avoid duplicates
						if (!currentItems.find((item) => item.ticker === newItem.ticker)) {
							return [...currentItems, newItem];
						}
						return currentItems;
					});
				}
			})
			.catch((error) => {
				console.error('Error adding to watchlist:', error);
			});
	}
</script>

<div class="ticker-info-container" bind:this={container}>
	<div class="content">
		<!-- Header Section -->
		<div class="quote-header">
			<div class="logo-container">
				{#if $instance?.logo || currentDetails?.logo}
					<img
						src={$instance?.logo || currentDetails?.logo}
						alt="{$instance?.name || currentDetails?.name || 'Company'} logo"
						class="company-logo-rect"
					/>
				{/if}
			</div>
			<div class="ticker-row">
				<div class="icon-circle">
					{#if $instance?.icon || currentDetails?.icon}
						<img
							src={$instance?.icon || currentDetails?.icon}
							alt="{$instance?.name || currentDetails?.name || 'Company'} icon"
							class="company-logo"
						/>
					{:else}
						<span class="ticker-letter">
							{($instance?.ticker || currentDetails?.ticker || '?').charAt(0)}
						</span>
					{/if}
				</div>
				<div class="ticker-wrapper">
					<div class="ticker-line">
						<div class="ticker">{$instance.ticker || '--'}</div>
						{#if $instance?.primary_exchange || currentDetails?.primary_exchange}
							<div class="exchange">
								{getExchangeName($instance?.primary_exchange || currentDetails?.primary_exchange)}
							</div>
						{/if}
					</div>
					{#if $instance?.active === false || currentDetails?.active === false}
						<div class="warning-triangle-container">
							<div class="warning-triangle"></div>
							<div class="tooltip">Delisted</div>
						</div>
					{/if}
				</div>
				<button
					class="add-to-watchlist-button"
					on:click|stopPropagation={addToWatchlist}
					title="Add to Watchlist"
				>
					+
				</button>
			</div>
			<div class="company-info">
				<div class="name">{cleanCompanyName($instance?.name || currentDetails?.name || '')}</div>
				<div class="sector-industry">
					{($instance?.sector || currentDetails?.sector || '').trim()}
					{#if ($instance?.industry || currentDetails?.industry || '').trim()}
						| {($instance?.industry || currentDetails?.industry || '').trim()}
					{/if}
				</div>
			</div>
		</div>

		<!-- Key Metrics Section -->
		<div class="quote-key-metrics">
			<div class="main-price-row">
				<div class="price-large">
					<StreamCell instance={$instance} type="price" disableFlash={true} />
				</div>
				<div class="change-absolute">
					<StreamCell instance={$instance} type="change" disableFlash={true} />
				</div>
				<div class="change-percent">
					<StreamCell instance={$instance} type="change %" disableFlash={true} />
				</div>
			</div>
			<div class="extended-hours-row">
				<span class="ext-label">Extended Hours:</span>
				<div class="ext-change">
					<StreamCell instance={$instance} type="change % extended" disableFlash={true} />
				</div>
			</div>
		</div>

		<!-- Market Data Section -->
		<div class="quote-market-data">
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
		<div class="quote-details">
			<div class="detail-item">
				<span class="label">Market Cap:</span>
				<span class="value">
					{#if $instance?.totalShares || currentDetails?.totalShares}
						<StreamCell instance={$instance} type="market cap" disableFlash={true} />
					{:else}
						N/A
					{/if}
				</span>
			</div>

			<div class="detail-item">
				<span class="label">Shares Outstanding:</span>
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
		<div class="countdown-section">
			<div class="countdown-container">
				<span class="countdown-label">Next Bar Close:</span>
				<span class="countdown-value">{$countdown}</span>
			</div>
		</div>

		<!-- Description Section -->
		{#if $instance?.description || currentDetails?.description}
			<div class="description">
				<span class="label">Description:</span>
				<p class="value description-text">
					{$instance?.description || currentDetails?.description}
				</p>
			</div>
		{/if}
	</div>
</div>

<style>
	.ticker-info-container {
		background: transparent;
		font-family: var(--font-primary);
		height: 100%;
		width: 100%;
		padding: 0;
		margin: 0;
		text-align: left;
		display: flex;
		flex-direction: column;
		outline: none;
		border: none;
	}

	.content {
		padding: 0 clamp(0.2rem, 0.4vw, 0.4rem) clamp(0.5rem, 1vw, 1rem) clamp(0.2rem, 0.4vw, 0.4rem);
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
		flex-direction: column;
		align-items: center;
		gap: 4px;
		margin: 0;
		padding: 0;
		background: transparent;
		border: none;
	}

	.ticker-row {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-left: 8px;
		margin-right: 8px;
		width: calc(100% - 16px);
		align-self: stretch;
	}

	.icon-circle {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: var(--ui-bg-secondary);
		border: 1px solid var(--ui-border);
		overflow: hidden;
	}

	.logo-container {
		width: 140px;
		height: 36px;
		border-radius: 2px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: transparent;
		overflow: hidden;
		margin: 0px auto;
	}

	.company-logo-rect {
		width: 100%;
		height: 100%;
		object-fit: contain;
		border-radius: 2px;
	}

	.company-logo {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
	}

	.ticker-letter {
		font-size: 14px;
		font-weight: 700;
		text-transform: uppercase;
		color: var(--text-primary);
		user-select: none;
	}

	.ticker-wrapper {
		display: flex;
		align-items: center;
		gap: 6px;
		flex: 1;
		min-width: 0;
	}

	.ticker-line {
		display: flex;
		align-items: baseline;
		gap: 12px;
		flex: 1;
		min-width: 0;
	}

	.ticker {
		font-size: 1.4em;
		font-weight: 700;
		color: var(--text-primary);
		text-transform: uppercase;
		line-height: 1.1;
	}

	.exchange {
		font-size: 0.75em;
		font-weight: 500;
		color: var(--text-secondary);
		line-height: 1.1;
		opacity: 0.8;
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
		transition:
			opacity 0.2s ease,
			visibility 0.2s ease;
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

	.add-to-watchlist-button {
		color: #ffffff;
		width: clamp(28px, 4vw, 32px);
		height: clamp(28px, 4vw, 32px);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: clamp(1rem, 0.7rem + 0.6vw, 1.2rem);
		font-weight: 300;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		background: transparent;
		border: none;
		border-radius: 6px;
		transition: all 0.2s ease;
		cursor: pointer;
		flex-shrink: 0;
		margin-left: auto;
	}

	.add-to-watchlist-button:hover {
		background: rgba(255, 255, 255, 0.2);
		color: #ffffff;
		transform: scale(1.05);
	}

	.company-info {
		display: flex;
		flex-direction: column;
		width: 100%;
		margin-left: 0px;
		margin-top: 12px;
		align-self: stretch;
	}

	.name {
		font-size: 1.1em;
		color: var(--text-primary);
		line-height: 1.2;
		font-weight: 600;
		padding: 0;
		margin-left: 8px;
		word-break: break-word;
		text-align: left;
	}

	.sector-industry {
		font-size: 0.75em;
		color: var(--text-secondary);
		line-height: 1.2;
		font-weight: 500;
		margin: 4px 0 0 0;
		margin-left: 8px;
		padding: 0;
		word-break: break-word;
		text-align: left;
	}

	/* Key Metrics */
	.quote-key-metrics {
		display: flex;
		flex-direction: column;
		gap: clamp(2px, 0.3vw, 3px);
		margin-bottom: clamp(10px, 2vw, 16px);
		padding: clamp(8px, 1.5vw, 12px) clamp(8px, 1.5vw, 12px) clamp(8px, 1.5vw, 12px) 8px;
	}

	.main-price-row {
		display: flex;
		align-items: baseline;
		gap: clamp(6px, 1vw, 8px);
		flex-wrap: wrap;
		margin-left: -6px;
	}

	.price-large {
		font-size: clamp(1.2rem, 2vw, 1.6rem);
		font-weight: 400;
		color: var(--text-primary);
		line-height: 1;
	}

	.change-absolute,
	.change-percent {
		font-size: clamp(0.8rem, 1.2vw, 1rem);
		font-weight: 600;
		line-height: 1;
	}

	.extended-hours-row {
		display: flex;
		align-items: baseline;
		gap: clamp(3px, 0.5vw, 4px);
	}

	.ext-label {
		font-size: clamp(0.6rem, 0.8vw, 0.7rem);
		color: var(--text-secondary);
		font-weight: 500;
	}

	.ext-change {
		font-size: clamp(0.6rem, 0.8vw, 0.7rem);
		font-weight: 600;
		line-height: 1;
	}

	/* Market Data */
	.quote-market-data {
		margin-bottom: clamp(4px, 1vw, 8px);
		padding: clamp(2px, 0.5vw, 4px) clamp(8px, 1.5vw, 12px) clamp(2px, 0.5vw, 4px)
			clamp(4px, 0.8vw, 6px);
	}

	/* Details */
	.quote-details {
		display: flex;
		flex-direction: column;
		gap: clamp(2px, 0.5vw, 4px);
		margin-bottom: clamp(10px, 2vw, 16px);
		padding: clamp(8px, 1.5vw, 12px) clamp(8px, 1.5vw, 12px) clamp(8px, 1.5vw, 12px)
			clamp(4px, 0.8vw, 6px);
	}

	.detail-item {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		font-size: 0.8em;
		padding: 4px 0;
	}

	.detail-item .label {
		color: #ffffff;
		margin-right: 8px;
		white-space: nowrap;
		font-weight: 500;
	}

	.detail-item .value {
		color: #ffffff;
		text-align: right;
		font-weight: 500;
	}

	/* Countdown */
	.countdown-section {
		margin-top: clamp(8px, 1.5vw, 12px);
		padding: clamp(8px, 1.5vw, 12px);
	}

	.countdown-container {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: clamp(6px, 1vw, 8px) clamp(8px, 1.5vw, 12px);
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
		font-size: clamp(0.65rem, 0.4rem + 0.4vw, 0.8rem);
		color: var(--text-primary);
		padding: clamp(3px, 0.5vw, 4px) clamp(6px, 1vw, 8px);
		min-width: clamp(50px, 8vw, 60px);
		text-align: center;
	}

	/* Description */
	.description {
		margin-top: clamp(10px, 2vw, 16px);
		padding: clamp(8px, 1.5vw, 12px);
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
	@media (max-width: 1400px) {
		.quote-key-metrics {
			gap: clamp(1px, 0.2vw, 2px);
			padding: clamp(8px, 1.5vw, 10px) clamp(8px, 1.5vw, 10px) clamp(8px, 1.5vw, 10px) 8px;
		}

		.main-price-row {
			gap: clamp(4px, 0.8vw, 6px);
		}

		.price-large {
			font-size: clamp(1.1rem, 1.8vw, 1.4rem);
		}

		.change-absolute,
		.change-percent {
			font-size: clamp(0.75rem, 1.1vw, 0.9rem);
		}

		.quote-details {
			gap: clamp(1px, 0.3vw, 3px);
			padding: clamp(8px, 1.5vw, 10px);
		}

		.detail-item {
			font-size: clamp(0.6rem, 0.4rem + 0.4vw, 0.8rem);
		}
	}

	@media (max-width: 1000px) {
		.main-price-row {
			gap: clamp(3px, 0.6vw, 5px);
		}

		.price-large {
			font-size: clamp(1rem, 1.6vw, 1.3rem);
		}

		.change-absolute,
		.change-percent {
			font-size: clamp(0.7rem, 1vw, 0.85rem);
		}
	}

	@media (max-width: 600px) {
		.main-price-row {
			flex-direction: column;
			align-items: flex-start;
			gap: clamp(2px, 0.4vw, 3px);
		}

		.price-large {
			font-size: clamp(0.9rem, 1.5vw, 1.2rem);
		}

		.change-absolute,
		.change-percent {
			font-size: clamp(0.65rem, 0.9vw, 0.8rem);
		}
	}
</style>
