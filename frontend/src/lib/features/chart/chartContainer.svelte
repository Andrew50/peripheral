<script lang="ts">
	import Chart from './chart.svelte';
	import { settings } from '$lib/core/stores';
	import { onMount, tick } from 'svelte';
	import { get } from 'svelte/store';
	export let width: number;

	// Add focus management
	let containerRef: HTMLDivElement;

	onMount(async () => {
		// Wait the next microtask so the DOM is ready
		await tick();

		if (containerRef) {
			containerRef.focus();
		}

		// Add global keyboard event listener for chart container
		const handleGlobalKeydown = (event: KeyboardEvent) => {
			if (/^[a-zA-Z0-9]$/.test(event.key) && !event.ctrlKey && !event.metaKey) {
				containerRef.focus();
			}
		};

		document.addEventListener('keydown', handleGlobalKeydown);
		return () => {
			document.removeEventListener('keydown', handleGlobalKeydown);
		};
	});

	// Handle focus management
	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Tab') {
			event.preventDefault(); // Prevent default tab behavior
		}
	}
</script>

<div class="chart-container" bind:this={containerRef} tabindex="0" on:keydown={handleKeyDown}>
	{#each Array.from({ length: $settings.chartRows }) as _, j}
		<div class="row" style="height: calc(100% / {$settings.chartRows})">
			{#each Array.from({ length: $settings.chartColumns }) as _, i}
				<Chart width={width / $settings.chartColumns} chartId={i + j * $settings.chartColumns} />
			{/each}
		</div>
	{/each}
</div>

<style>
	.chart-container {
		display: flex;
		flex-direction: column;
		width: 100%;
		height: 100%;
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		outline: none; /* Remove focus outline but maintain accessibility */
	}

	.row {
		display: flex;
		width: 100%;
		justify-content: space-between;
		flex: 1;
		min-height: 0;
	}
</style>
