<script lang="ts">
	import '$lib/core/global.css';
	import type { DrawingMenuProps } from './chart';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import type { Writable } from 'svelte/store';
	export let drawingMenuProps: Writable<DrawingMenuProps>;
	function removePriceLine(event: MouseEvent) {
		event.preventDefault();
		event.stopPropagation();
		if ($drawingMenuProps.selectedLine !== null) {
			$drawingMenuProps.chartCandleSeries.removePriceLine($drawingMenuProps.selectedLine);
			$drawingMenuProps.horizontalLines = $drawingMenuProps.horizontalLines.filter(
				(line) => line.line !== $drawingMenuProps.selectedLine
			);

			drawingMenuProps.update((v: DrawingMenuProps) => {
				v.selectedLine = null;
				v.active = false;
				return v;
			});
			privateRequest<void>('deleteHorizontalLine', { id: $drawingMenuProps.selectedLineId }, true);
			console.log('Price line removed');
		}
	}
	onMount(() => {
		document.addEventListener('click', handleClickOutside);
	});

	function handleClickOutside(event: MouseEvent) {
		const popup = document.querySelector('.test');
		if (popup && !popup.contains(event.target as Node)) {
			console.log('clicked outside drawing menu');
			drawingMenuProps.update((v: DrawingMenuProps) => {
				v.active = false;
				return v;
			});
		}
	}
</script>

{#if $drawingMenuProps.active}
	<div
		class="test"
		style="left: {$drawingMenuProps.clientX}px; top: {$drawingMenuProps.clientY}px;"
	>
		<button on:click={removePriceLine}>Delete</button>
	</div>
{/if}

<style>
	.popup-container {
		width: 180px;
		background-color: rgba(0, 0, 0, 0.5); /* Semi-transparent black background */
		border: 1px solid rgba(255, 255, 255, 0.1); /* Subtle border */
		border-radius: 4px; /* Rounded corners */
		padding: 4px;
		position: absolute;
		z-index: 1000;
	}
	button {
		width: 100%;
	}
	.test {
		position: absolute;
		top: 0; /* Adjust as needed */
		left: 0; /* Adjust as needed */
		/* Optionally, you can set right and bottom to control the size */
		/* right: 0; */
		/* bottom: 0; */
		z-index: 1000; /* Ensure it's on top of other elements */
		background-color: rgba(0, 0, 0, 0.5); /* Example background color */
		border: 1px solid rgba(255, 255, 255, 0.1); /* Example border */
		border-radius: 4px; /* Example rounded corners */
		padding: 4px; /* Example padding */
	}
</style>
