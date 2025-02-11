<script lang="ts">
	import '$lib/core/global.css';
	import type { DrawingMenuProps } from './chart';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import type { Writable } from 'svelte/store';
	export let drawingMenuProps: Writable<DrawingMenuProps>;

    let price: Number;
	let menuElement: HTMLDivElement;

	function removePriceLine(event: MouseEvent) {
		console.log('removePriceLine');
		event.preventDefault();
		event.stopImmediatePropagation();
		console.log('removePriceLine');
		if ($drawingMenuProps.selectedLine !== null) {
			console.log('removing price line');
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

    function editHorizontalLinePrice(){
        console.log("updating price line price")



		if ($drawingMenuProps.selectedLine !== null) {
            $drawingMenuProps.chartCandleSeries.updatePriceLine($drawingMenuProps.selectedLine,price)
            //$drawingMenuProps.selectedLine.price = price already updated by bind:Lthis?
        }
    }

	function handleClickOutside(event: MouseEvent) {
		console.log($drawingMenuProps.active, $drawingMenuProps.isDragging);
		if (!$drawingMenuProps.active || $drawingMenuProps.isDragging) {
			console.log('handleClickOutside -----');
			return;
		}
		console.log('handleClickOutside');
		console.log(menuElement);

		if (!menuElement) return;

		const deleteButton = menuElement.querySelector('button');
		if (deleteButton && deleteButton.contains(event.target as Node)) {
			return;
		}

		const clickY = event.clientY;
		const isClickInMenu =
			event.target === menuElement || menuElement.contains(event.target as Node);

		const selectedLine = $drawingMenuProps.selectedLine;
		const chartCandleSeries = $drawingMenuProps.chartCandleSeries;
		let isClickNearLine = false;

		if (selectedLine && chartCandleSeries) {
			const linePrice = selectedLine.options().price;
			const lineY = chartCandleSeries.priceToCoordinate(linePrice) || 0;
			const CLICK_THRESHOLD = 5; // pixels
			isClickNearLine = Math.abs(clickY - lineY) <= CLICK_THRESHOLD;
		}

		if (!isClickInMenu && !isClickNearLine) {
			drawingMenuProps.update((v: DrawingMenuProps) => ({
				...v,
				active: false
			}));
		}
	}

	onMount(() => {
		document.addEventListener('mousedown', handleClickOutside);
		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
		};
	});

	$: menuStyle = `
		left: ${$drawingMenuProps.clientX}px; 
		top: ${$drawingMenuProps.clientY}px;
		pointer-events: ${$drawingMenuProps.isDragging ? 'none' : 'auto'};
	`;
</script>

{#if $drawingMenuProps.active && !$drawingMenuProps.isDragging}
	<div bind:this={menuElement} class="drawing-menu" style={menuStyle}>
		<button on:click={removePriceLine}>Delete</button>
        <input on:change={editHorizontalLinePrice} bind:this={$drawingMenuProps.selectedLine.price}/>


	</div>
{/if}

<style>
	.drawing-menu {
		position: absolute;
		z-index: 1000;
		background-color: rgba(0, 0, 0, 0.5);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 4px;
		padding: 4px;
	}
	button {
		width: 100%;
	}
</style>
