<script lang="ts">
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { activeChartInstance, queryChart } from '$lib/features/chart/interface';
	import type { Instance } from '$lib/core/types';
	import { writable } from 'svelte/store';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';
	import { streamInfo, formatTimestamp } from '$lib/core/stores';

	// Import the Quotes module here, so it can be toggled
	import Quotes from '$lib/features/quotes/quotes.svelte';

	const tickerInfoState = writable({
		isExpanded: true,
		currentHeight: 600
	});

	let showQuotes = false; // new dropdown toggle for Quotes
	let container: HTMLDivElement;
	let isDragging = false;
	let startY = 0;

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

		tickerInfoState.update((state) => ({
			...state,
			currentHeight: Math.min(Math.max(state.currentHeight + deltaY, 200), 800)
		}));
	}

	function handleMouseUp() {
		isDragging = false;
		document.body.style.cursor = '';
		document.body.style.userSelect = '';
	}

	function handleClick(event: MouseEvent | TouchEvent) {
		// Only query chart if not clicking expand or drag handle
		if (
			event.target instanceof HTMLButtonElement ||
			(event.target instanceof HTMLElement && event.target.classList.contains('drag-handle'))
		) {
			return;
		}

		if ($activeChartInstance) {
			queryChart($activeChartInstance);
		}
	}

	function toggleQuotes() {
		showQuotes = !showQuotes;
	}
</script>

<div
	class="ticker-info-container expanded"
	style="height: {$tickerInfoState.currentHeight}px"
	bind:this={container}
	on:click={handleClick}
	on:touchstart={handleClick}
>
	<div
		class="drag-handle"
		on:mousedown={handleMouseDown}
		on:touchstart|preventDefault={handleMouseDown}
	>
		<span class="ticker-name">{$activeChartInstance?.ticker || 'NaN'}</span>
	</div>

	{#if $activeChartInstance !== null}
		<div class="content">
			{#if $activeChartInstance.logo}
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
					<span class="info-row">Price</span>
					<StreamCell instance={$activeChartInstance} type="price" />
				</div>
				<div class="stream-cell-container">
					<span class="info-row">Change %</span>
					<StreamCell instance={$activeChartInstance} type="change %" />
				</div>
				<div class="stream-cell-container">
					<span class="info-row">Change $</span>
					<StreamCell instance={$activeChartInstance} type="change" />
				</div>
				<div class="stream-cell-container">
					<span class="info-row">Change % extended</span>
					<StreamCell instance={$activeChartInstance} type="change % extended" />
				</div>
			</div>
			<div class="info-row">
				<span class="label">Name:</span>
				<span class="value">{$activeChartInstance.name}</span>
			</div>
			<div class="info-row">
				<span class="label">Status:</span>
				<span class="value">{$activeChartInstance.active ? 'Active' : 'Inactive'}</span>
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

			<!-- Toggle for showing quotes -->
			<div class="info-row quotes-toggle">
				<span class="label">Quotes:</span>
				<button on:click|stopPropagation={toggleQuotes}>
					{showQuotes ? 'Hide' : 'Show'}
				</button>
			</div>
			{#if showQuotes}
				<div class="quotes-section">
					<Quotes />
				</div>
			{/if}
		</div>
	{/if}
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
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		cursor: pointer;
		height: 300px; /* Default height */
	}

	.ticker-info-container.expanded {
		transition: none;
	}

	.ticker-info-container:not(.expanded) {
		transition: height 0.2s ease;
	}

	.drag-handle {
		width: 100%;
		height: 30px;
		background: var(--ui-bg-primary);
		cursor: ns-resize;
		display: flex;
		align-items: center;
		padding: 0 10px;
		user-select: none;
		touch-action: none;
		border-bottom: 1px solid var(--ui-border);
		color: var(--text-primary);
		font-size: 14px;
		font-weight: 500;
	}

	.logo-container {
		display: flex;
		justify-content: center;
		margin: 15px 0;
		padding: 10px;
		background: var(--ui-bg-element);
		border-radius: 4px;
	}
	.company-logo {
		max-height: 40px;
		max-width: 200px;
		object-fit: contain;
	}

	.content {
		padding: 0 15px 30px;
		overflow-y: auto;
		scrollbar-width: none;
		-ms-overflow-style: none;
		height: calc(100% - 30px);
		color: var(--text-primary);
	}
	.content::-webkit-scrollbar {
		display: none;
	}

	.info-row {
		display: flex;
		justify-content: space-between;
		margin-bottom: 8px;
		padding: 6px 8px;
		font-size: 12px;
		background: var(--ui-bg-element);
		border-radius: 4px;
	}

	.info-row:hover {
		background: var(--ui-bg-hover);
	}
	.label {
		color: var(--text-secondary);
		font-weight: 500;
	}

	.value {
		font-family: monospace;
		font-weight: 500;
		font-variant-numeric: tabular-nums;
	}

	.stream-cells {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 10px;
		margin: 15px 0;
		padding: 10px;
		background: var(--ui-bg-secondary);
		border-radius: 4px;
		border: 1px solid var(--ui-border);
	}

	.stream-cell-container {
		padding: 8px;
		background: var(--ui-bg-element);
		border-radius: 4px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.stream-cell-container .info-row {
		margin: 0;
		padding: 0;
		background: none;
		font-weight: 500;
		color: var(--text-secondary);
	}

	.description {
		margin-top: 15px;
		padding-top: 10px;
		border-top: 1px solid var(--ui-border);
	}
	.description-text {
		margin-top: 5px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--text-primary);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.ticker-name {
		font-weight: 600;
	}

	.quotes-toggle {
		background: var(--ui-bg-element);
	}
	.quotes-section {
		margin-top: 10px;
	}
</style>
