<script lang="ts">
	import Chart from './chart.svelte';
	import { settings } from '$lib/utils/stores/stores';
	import { onMount, tick } from 'svelte';
	import { get } from 'svelte/store';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryChart } from './interface';
	export let width: number;

	// Add focus management
	let containerRef: HTMLDivElement;

	onMount(() => {
		// Wait for DOM to be ready
		tick().then(() => {
			if (containerRef) {
				containerRef.focus();
			}
		});

		// Add global keyboard event listener for chart container
		const handleGlobalKeydown = (event: KeyboardEvent) => {
			// Check if input popup is active by looking for the hidden input
			const hiddenInput = document.getElementById('hidden-input');
			if (hiddenInput && document.activeElement === hiddenInput) {
				// Input popup is active, don't trigger new input
				return;
			}

			// Check if the user is currently in any standard input field
			const activeElement = document.activeElement;
			const isInputField =
				activeElement?.tagName === 'INPUT' ||
				activeElement?.tagName === 'TEXTAREA' ||
				activeElement?.getAttribute('contenteditable') === 'true';

			// If user is typing in any input field, don't intercept keystrokes
			if (isInputField) {
				return;
			}

			if (/^[a-zA-Z0-9]$/.test(event.key) && !event.ctrlKey && !event.metaKey) {
				// Create an initial instance with the first key as the inputString
				const initialKey = event.key.toUpperCase();

				// Use type assertion to allow the inputString property
				const instanceWithInput = {
					inputString: initialKey
				} as any;

				queryInstanceInput(
					'any',
					['ticker', 'timeframe', 'timestamp', 'extendedHours'],
					instanceWithInput
				).then((updatedInstance) => {
					queryChart(updatedInstance, true);
				});

				// Only focus if we're not in an input field or similar
				if (containerRef) {
					containerRef.focus();
				}
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

<div
	class="chart-container"
	bind:this={containerRef}
	tabindex="0"
	role="application"
	aria-label="Chart Container"
	on:keydown={handleKeyDown}
>
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
		position: relative; /* Changed from absolute to relative */
		outline: none; /* Remove focus outline but maintain accessibility */
	}

	.row {
		display: flex;
		width: 100%;
		justify-content: space-between;
		flex: 1;
		min-height: 0;
	}

	/* Add responsive focus indicator for accessibility */
	.chart-container:focus {
		box-shadow: inset 0 0 0 clamp(1px, 0.2vw, 2px) var(--ui-accent, rgba(59, 130, 246, 0.5));
	}

	@media (max-width: 768px) {
		/* Adjust for smaller screens if needed */
		.chart-container {
			overflow: hidden;
		}
	}
</style>
