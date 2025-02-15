<script lang="ts">
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { activeChartInstance } from '$lib/features/chart/interface';
	import type { Instance } from '$lib/core/types';
	import { writable } from 'svelte/store';
	import StreamCell from '$lib/utils/stream/streamCell.svelte';

	import { streamInfo, formatTimestamp } from '$lib/core/stores';

	const tickerInfoState = writable({
		isExpanded: true,
		currentHeight: 300
	});

	let startY = 0;
	let isDragging = false;
	let container: HTMLDivElement;

	//let tickerDetails: TickerDetails | null = null;

	onMount(() => {
		/*activeChartInstance.subscribe((instance: Instance | null) => {
			if (instance !== null && instance !== undefined && instance.securityId !== undefined) {
				loadTickerData(instance.securityId);
			}
		});*/
		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);

		return () => {
			document.removeEventListener('mousemove', handleMouseMove);
			document.removeEventListener('mouseup', handleMouseUp);
		};
	});

	/*async function loadTickerData(securityId: number) {
		try {
			const data = await privateRequest('getTickerDetails', { securityId }, true);
	//		tickerDetails = data as TickerDetails;
		} catch (error) {
			console.error('Error loading ticker data:', error);
		}
	}*/

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
			currentHeight: Math.min(Math.max(state.currentHeight + deltaY, 50), 400)
		}));
	}

	function handleMouseUp() {
		isDragging = false;
		document.body.style.cursor = '';
		document.body.style.userSelect = '';
	}

	function toggleExpand() {
		tickerInfoState.update((state) => ({
			...state,
			isExpanded: !state.isExpanded,
			currentHeight: !state.isExpanded ? state.currentHeight : 200
		}));
	}
</script>

<div
	class="ticker-info-container {$tickerInfoState.isExpanded ? 'expanded' : ''}"
	style="height: {$tickerInfoState.isExpanded ? $tickerInfoState.currentHeight : '30'}px"
	bind:this={container}
>
	<div
		class="drag-handle"
		on:mousedown={handleMouseDown}
		on:touchstart|preventDefault={handleMouseDown}
	>
		<button class="expand-button" on:click|stopPropagation={toggleExpand}>
			{$tickerInfoState.isExpanded ? '▼' : '▲'}
		</button>
		<span>Ticker Info - {$activeChartInstance?.ticker || 'No ticker selected'}</span>
	</div>

	{#if $activeChartInstance !== null && $activeChartInstance !== null}
		{#if $tickerInfoState.isExpanded && $activeChartInstance.detailsUpdateStore}
			<div class="content">
				<div class="system-clock">
					<h3>
						{$streamInfo.timestamp !== undefined
							? formatTimestamp($streamInfo.timestamp)
							: 'Loading Time...'}
					</h3>
				</div>
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
					<span class="label">Shares Outstanding:</span>
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
			</div>
		{/if}
	{/if}
</div>

<style>
	.system-clock {
		background-color: var(--c2);
		padding: 0px 10px;
		font-size: 1rem;
		color: var(--f1);
		width: 100%;
		text-align: center;
		box-sizing: border-box;
	}
	.ticker-info-container {
		position: absolute;
		bottom: 0;
		left: 0;
		right: 0;
		background: var(--c2);
		border-top: 1px solid var(--c4);
		overflow: hidden;
		will-change: height;
	}

	.ticker-info-container.expanded {
		transition: none; /* Remove transition when expanded for better drag response */
	}

	.ticker-info-container:not(.expanded) {
		transition: height 0.2s ease; /* Only animate when collapsing/expanding */
	}

	.drag-handle {
		width: 100%;
		height: 30px;
		background: #2a2e39;
		cursor: ns-resize;
		display: flex;
		align-items: center;
		padding: 0 10px;
		user-select: none;
		touch-action: none; /* Improve touch handling */
	}

	.expand-button {
		background: none;
		border: none;
		color: #fff;
		cursor: pointer;
		padding: 5px;
		margin-right: 10px;
		z-index: 2; /* Ensure button is clickable */
	}

	.expand-button:hover {
		background: rgba(255, 255, 255, 0.1);
	}

	.logo-container {
		display: flex;
		justify-content: center;
		margin-bottom: 15px;
		padding: 10px;
		/*background: rgba(255, 255, 255, 0.05);*/
		border-radius: 4px;
	}

	.company-logo {
		max-height: 40px;
		max-width: 200px;
		object-fit: contain;
	}

	.description {
		margin-top: 15px;
		padding-top: 10px;
		border-top: 1px solid var(--c4);
	}

	.description-text {
		margin-top: 5px;
		font-size: 11px;
		line-height: 1.4;
		color: #ccc;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.content {
		padding: 15px;
		padding-bottom: 30px;
		overflow-y: auto;
		scrollbar-width: none; /* Firefox */
		-ms-overflow-style: none; /* Internet Explorer 10+ */
		height: calc(100% - 30px);
	}

	.content::-webkit-scrollbar {
		display: none; /* WebKit */
	}

	.info-row {
		display: flex;
		justify-content: space-between;
		margin-bottom: 8px;
		color: #fff;
		font-size: 12px;
		padding: 4px 0;
	}

	.label {
		color: #8f95a3;
	}

	.value {
		font-family: monospace;
	}

	.stream-cells {
		display: flex;
		gap: 10px; /* Adjust spacing between cells as needed */
	}

	.stream-cell {
		font-size: 4rem; /* Increase font size */
		color: white;
	}
</style>
