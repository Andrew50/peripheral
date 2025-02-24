<script lang="ts">
	import L1 from './l1.svelte';
	import TimeAndSales from './timeAndSales.svelte';
	import { get, writable, type Writable } from 'svelte/store';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import type { Instance } from '$lib/core/types';
	import { activeChartInstance, queryChart } from '$lib/features/chart/interface';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { streamInfo, formatTimestamp } from '$lib/core/stores';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';

	let instance: Writable<Instance> = writable({});
	let container: HTMLDivElement;
	let showTimeAndSales = false;
	let currentDetails: Record<string, any> = {};
	let lastFetchedSecurityId: number | null = null;

	// Sync instance with activeChartInstance and handle details fetching
	activeChartInstance.subscribe((chartInstance: Instance | null) => {
		console.log('Quote component: activeChartInstance update received', chartInstance);
		if (chartInstance?.ticker) {
			// Only update if we have a valid ticker
			console.log('Quote component: Setting new instance with ticker:', chartInstance.ticker);
			instance.set(chartInstance);

			// Handle details fetching in the main subscription
			if (chartInstance.securityId && lastFetchedSecurityId !== chartInstance.securityId) {
				console.log('Quote component: Fetching details for security ID:', chartInstance.securityId);
				lastFetchedSecurityId = chartInstance.securityId;
				privateRequest<Record<string, any>>(
					'getTickerMenuDetails',
					{ securityId: chartInstance.securityId },
					true
				)
					.then((details) => {
						console.log('Quote component: Received details:', details);
						if (lastFetchedSecurityId === chartInstance.securityId) {
							currentDetails = details;
							// Update the instance directly instead of activeChartInstance
							instance.update((inst) => ({
								...inst,
								...details
							}));
						} else {
							console.log('Quote component: Ignoring stale details response');
						}
					})
					.catch((error) => {
						console.error('Quote component: Error fetching details:', error);
						if (lastFetchedSecurityId === chartInstance.securityId) {
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

	function handleClick(event: MouseEvent | TouchEvent) {
		if ($activeChartInstance) {
			queryChart($activeChartInstance);
		}
	}

	$: if (container) {
		container.addEventListener('keydown', handleKey);
	}

	onMount(() => {
		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);

		return () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
		};
	});

	function handleMouseMove(e: MouseEvent | TouchEvent) {
		// This function is now empty as the height-related variables and functions are removed
	}

	function handleMouseUp() {
		// This function is now empty as the height-related variables and functions are removed
	}
</script>

<div
	class="ticker-info-container"
	bind:this={container}
	on:click={handleClick}
	on:touchstart={handleClick}
>
	<div class="content">
		<div class="ticker-container">
			<div class="ticker-display">
				<span class="ticker">{$instance.ticker || '--'}</span>
			</div>
		</div>

		{#if $activeChartInstance?.logo}
			<div class="logo-container">
				<img
					src="data:image/svg+xml;base64,{$activeChartInstance.logo}"
					alt="{$activeChartInstance.name} logo"
					class="company-logo"
				/>
			</div>
		{/if}

		<div class="stream-cells">
			<div class="stream-cell-container">
				<span class="label">Price</span>
				<StreamCell instance={$instance} type="price" />
			</div>
			<div class="stream-cell-container">
				<span class="label">Chg %</span>
				<StreamCell instance={$instance} type="change %" />
			</div>
			<div class="stream-cell-container">
				<span class="label">Chg $</span>
				<StreamCell instance={$instance} type="change" />
			</div>
			<div class="stream-cell-container">
				<span class="label">Ext %</span>
				<StreamCell instance={$instance} type="change % extended" />
			</div>
		</div>

		<div class="quotes-section">
			<L1 {instance} />
			<button class="time-sales-button" on:click|stopPropagation={toggleTimeAndSales}>
				{showTimeAndSales ? 'Hide Time & Sales' : 'Show Time & Sales'}
			</button>
			{#if showTimeAndSales}
				<TimeAndSales {instance} />
			{/if}
		</div>

		<div class="info-row">
			<span class="label">Name:</span>
			<span class="value">{$instance?.name || currentDetails?.name || 'N/A'}</span>
		</div>
		<div class="info-row">
			<span class="label">Active:</span>
			<span class="value">{$instance?.active || currentDetails?.active || 'N/A'}</span>
		</div>
		<div class="info-row">
			<span class="label">Market Cap:</span>
			<span class="value">
				{#if $instance?.totalShares || currentDetails?.totalShares}
					<StreamCell instance={$instance} type="market cap" />
				{:else}
					N/A
				{/if}
			</span>
		</div>
		<div class="info-row">
			<span class="label">Sector:</span>
			<span class="value">{$instance?.sector || currentDetails?.sector || 'N/A'}</span>
		</div>
		<div class="info-row">
			<span class="label">Industry:</span>
			<span class="value">{$instance?.industry || currentDetails?.industry || 'N/A'}</span>
		</div>
		<div class="info-row">
			<span class="label">Exchange:</span>
			<span class="value"
				>{$instance?.primary_exchange || currentDetails?.primary_exchange || 'N/A'}</span
			>
		</div>
		<div class="info-row">
			<span class="label">Market:</span>
			<span class="value">{$instance?.market || currentDetails?.market || 'N/A'}</span>
		</div>
		<div class="info-row">
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
		{#if $activeChartInstance?.description}
			<div class="description">
				<span class="label">Description:</span>
				<p class="value description-text">{$activeChartInstance?.description}</p>
			</div>
		{/if}
	</div>
</div>

<style>
	.ticker-info-container {
		background: var(--ui-bg-primary);
		backdrop-filter: var(--backdrop-blur);
		border-top: 1px solid var(--ui-border);
		overflow: hidden;
		will-change: height;
		font-family: var(--font-primary);
		height: 100%;
	}

	.ticker-info-container.expanded {
		transition: none;
	}

	.ticker-info-container:not(.expanded) {
		transition: height 0.2s ease;
	}

	.content {
		padding: 15px;
		overflow-y: auto;
		scrollbar-width: none;
		-ms-overflow-style: none;
		height: 100%;
		color: var(--text-primary);
	}

	.content::-webkit-scrollbar {
		display: none;
	}

	.company-logo {
		max-height: 40px;
		max-width: 200px;
		object-fit: contain;
	}

	.ticker-display {
		font-family: var(--font-primary);
		font-size: 28px;
		font-weight: 600;
		color: var(--text-primary);
		background: var(--ui-bg-secondary);
		width: 100%;
		height: 50px;
		text-align: center;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 6px;
	}

	.ticker {
		letter-spacing: 0.5px;
		text-transform: uppercase;
	}

	.ticker-container {
		margin-bottom: 15px;
		padding: 0 15%; /* Add padding on sides to make ticker display narrower */
	}

	.description {
		margin-top: 15px;
		padding-top: 10px;
		border-top: 1px solid var(--ui-border);
	}

	.stream-cell-container {
		margin: 0;
		padding: 0;
		background: none;
		font-weight: 500;
		color: var(--text-secondary);
	}

	.logo-container {
		display: flex;
		justify-content: center;
		margin-bottom: 15px;
	}

	.stream-cells {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 8px;
		margin: 15px 0;
	}

	.stream-cell-container {
		margin: 0;
		padding: 0;
		background: none;
		font-weight: 500;
		color: var(--text-secondary);
		overflow: hidden;
	}

	.stream-cell-container .label {
		font-size: 0.85em;
		display: block;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		margin-bottom: 2px;
	}

	.time-sales-button {
		background: var(--ui-bg-secondary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		padding: 6px 12px;
		font-size: 0.9em;
		cursor: pointer;
		transition: background-color 0.2s;
		margin: 10px 0;
		width: 100%;
	}

	.time-sales-button:hover {
		background: var(--ui-bg-hover);
	}

	.quotes-section {
		margin-top: 15px;
		border-top: 1px solid var(--ui-border);
		padding-top: 15px;
	}
</style>
