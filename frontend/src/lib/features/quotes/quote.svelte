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
	let isDragging = false;
	let startY = 0;
	let currentHeight = 600;

	// Sync instance with activeChartInstance
	activeChartInstance.subscribe((chartInstance) => {
		if (chartInstance) {
			instance.set(chartInstance);
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
		activeChartInstance.subscribe((instance: Instance | null) => {
			if (instance && !instance.detailsFetched && instance.securityId) {
				privateRequest('getTickerMenuDetails', { securityId: instance.securityId }, true).then(
					(details) => {
						activeChartInstance.update((inst: Instance) => ({
							...inst,
							...details,
							detailsFetched: true
						}));
					}
				);
			}
		});

		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);

		return () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
		};
	});

	function handleMouseDown(e: MouseEvent | TouchEvent) {
		if (e.target instanceof HTMLButtonElement) return;
		isDragging = true;
		if (e instanceof MouseEvent) {
			startY = e.clientY;
		} else {
			startY = e.touches[0].clientY;
		}
		document.body.style.cursor = 'ns-resize';
		document.body.style.userSelect = 'none';
	}

	function handleMouseMove(e: MouseEvent | TouchEvent) {
		if (!isDragging) return;
		let currentY;
		if (e instanceof MouseEvent) {
			currentY = e.clientY;
		} else {
			currentY = e.touches[0].clientY;
		}
		const deltaY = startY - currentY;
		startY = currentY;

		currentHeight = Math.min(Math.max(currentHeight + deltaY, 200), 800);
	}

	function handleMouseUp() {
		isDragging = false;
		document.body.style.cursor = '';
		document.body.style.userSelect = '';
	}
</script>

<div
	class="ticker-info-container expanded"
	style="height: {currentHeight}px"
	bind:this={container}
	on:click={handleClick}
	on:touchstart={handleClick}
>
	<div class="content">
		{#if $activeChartInstance?.logo}
			<div class="logo-container">
				<img
					src="data:image/svg+xml;base64,{$activeChartInstance.logo}"
					alt="{$activeChartInstance.name} logo"
					class="company-logo"
				/>
			</div>
		{/if}

		<div class="ticker-container">
			<div class="ticker-display">
				<span class="ticker">{$instance.ticker || '--'}</span>
			</div>
		</div>

		<div class="stream-cells">
			<div class="stream-cell-container">
				<span class="label">Price</span>
				<StreamCell instance={$activeChartInstance} type="price" />
			</div>
			<div class="stream-cell-container">
				<span class="label">Change %</span>
				<StreamCell instance={$activeChartInstance} type="change %" />
			</div>
			<div class="stream-cell-container">
				<span class="label">Change $</span>
				<StreamCell instance={$activeChartInstance} type="change" />
			</div>
			<div class="stream-cell-container">
				<span class="label">Change % extended</span>
				<StreamCell instance={$activeChartInstance} type="change % extended" />
			</div>
		</div>

		<div class="quotes-section">
			<L1 {instance} />
			<button class="toggle-button" on:click|stopPropagation={toggleTimeAndSales}>
				{showTimeAndSales ? 'Hide T&S' : 'Show T&S'}
			</button>
			{#if showTimeAndSales}
				<TimeAndSales {instance} />
			{/if}
		</div>

		{#if $activeChartInstance}
			<div class="info-row">
				<span class="label">Name:</span>
				<span class="value">{$activeChartInstance.name}</span>
			</div>
			<div class="info-row">
				<span class="label">Active:</span>
				<span class="value">{$activeChartInstance.active}</span>
			</div>
			<div class="info-row">
				<span class="label">Market Cap:</span>
				<span class="value">
					{#if $activeChartInstance.market_cap}
						{#if $activeChartInstance.market_cap >= 1e12}
							${($activeChartInstance.market_cap / 1e12).toFixed(2)}T
						{:else if $activeChartInstance.market_cap >= 1e9}
							${($activeChartInstance.market_cap / 1e9).toFixed(2)}B
						{:else}
							${($activeChartInstance.market_cap / 1e6).toFixed(2)}M
						{/if}
					{:else}
						N/A
					{/if}
				</span>
			</div>
			<div class="info-row">
				<span class="label">Sector:</span>
				<span class="value">{$activeChartInstance.sector || 'N/A'}</span>
			</div>
			<div class="info-row">
				<span class="label">Industry:</span>
				<span class="value">{$activeChartInstance.industry || 'N/A'}</span>
			</div>
			<div class="info-row">
				<span class="label">Exchange:</span>
				<span class="value">{$activeChartInstance.primary_exchange || 'N/A'}</span>
			</div>
			<div class="info-row">
				<span class="label">Market:</span>
				<span class="value">{$activeChartInstance.market || 'N/A'}</span>
			</div>
			<div class="info-row">
				<span class="label">Shares Out:</span>
				<span class="value">
					{$activeChartInstance.share_class_shares_outstanding
						? `${($activeChartInstance.share_class_shares_outstanding / 1e6).toFixed(2)}M`
						: 'N/A'}
				</span>
			</div>
			{#if $activeChartInstance.description}
				<div class="description">
					<span class="label">Description:</span>
					<p class="value description-text">{$activeChartInstance.description}</p>
				</div>
			{/if}
		{/if}
	</div>
</div>

<style>
	.ticker-info-container {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		background: var(--ui-bg-primary);
		backdrop-filter: var(--backdrop-blur);
		border-top: 1px solid var(--ui-border);
		overflow: hidden;
		will-change: height;
		font-family: var(--font-primary);
		cursor: pointer;
		height: 300px;
	}

	.ticker-info-container.expanded {
		transition: none;
	}

	.ticker-info-container:not(.expanded) {
		transition: height 0.2s ease;
	}

	.content {
		padding: 15px 15px 30px;
		overflow-y: auto;
		scrollbar-width: none;
		-ms-overflow-style: none;
		height: calc(100% - 30px);
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
		font-size: 20px;
		color: white;
		background-color: black;
		width: 100%;
		height: 40px;
		text-align: center;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 4px;
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
</style>
