<!-- drawingMenu.svelte -->
<script lang="ts" context="module">
	import type { ISeriesApi, IPriceLine } from 'lightweight-charts';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';

	export interface DrawingMenuProps {
		chartCandleSeries: ISeriesApi<'Candlestick'>;
		selectedLine: IPriceLine | null;
		clientX: number;
		clientY: number;
		active: boolean;
		horizontalLines: { price: number; line: IPriceLine; id: number }[];
		isDragging: boolean;
		securityId: number;
	}
	export let drawingMenuProps: Writable<DrawingMenuProps> = writable({
		chartCandleSeries: null,
		selectedLine: null,
		clientX: 0,
		clientY: 0,
		active: false,
		selectedLineId: -1,
		horizontalLines: [],
		isDragging: false,
		selectedLinePrice: 0,
		securityId: -1
	});
	export function addHorizontalLine(price: number, securityId: number, id: number = -1) {
		price = parseFloat(price.toFixed(2));
		const priceLine = get(drawingMenuProps).chartCandleSeries.createPriceLine({
			price: price,
			color: 'white',
			lineWidth: 1,
			lineStyle: 0, // Solid line
			axisLabelVisible: true,
			title: `Price: ${price}`
		});
		get(drawingMenuProps).horizontalLines.push({
			id,
			price,
			line: priceLine
		});
		if (id == -1) {
			// only add to baceknd if its being added not from a ticker load but from a new added line
			privateRequest<number>('setHorizontalLine', {
				price: price,
				securityId: securityId
			}).then((res: number) => {
				get(drawingMenuProps).horizontalLines[get(drawingMenuProps).horizontalLines.length - 1].id =
					res;
			});
		}
	}
</script>

<script lang="ts">
	import '$lib/core/global.css';
	import type { DrawingMenuProps } from './chart';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import type { Writable } from 'svelte/store';
	export let drawingMenuProps: Writable<DrawingMenuProps>;

	let menuElement: HTMLDivElement;

	function removePriceLine(event: MouseEvent) {
		event.preventDefault();
		event.stopImmediatePropagation();
		if ($drawingMenuProps.selectedLine !== null) {
			deleteHorizontalLine($drawingMenuProps.selectedLine);
		}
	}

	function deleteHorizontalLine(line: IPriceLine) {
		$drawingMenuProps.chartCandleSeries.removePriceLine(line);
		$drawingMenuProps.horizontalLines = $drawingMenuProps.horizontalLines.filter(
			(l) => l.line !== line
		);
		drawingMenuProps.update((v: DrawingMenuProps) => {
			v.selectedLine = null;
			v.active = false;
			return v;
		});
		privateRequest<void>('deleteHorizontalLine', { id: $drawingMenuProps.selectedLineId }, true);
	}

	function editHorizontalLinePrice() {
		if ($drawingMenuProps.selectedLine !== null) {
			// Update the existing line instead of creating a new one and deleting the old one
			const price = parseFloat($drawingMenuProps.selectedLinePrice.toFixed(2));
			$drawingMenuProps.selectedLine.applyOptions({
				price: price,
				title: `Price: ${price}`
			});
			
			// Update the stored price in horizontalLines array
			const lineIndex = $drawingMenuProps.horizontalLines.findIndex(
				(line) => line.line === $drawingMenuProps.selectedLine
			);
			
			if (lineIndex !== -1) {
				$drawingMenuProps.horizontalLines[lineIndex].price = price;
				
				// Update in backend
				privateRequest<void>(
					'updateHorizontalLine',
					{
						id: $drawingMenuProps.horizontalLines[lineIndex].id,
						price: price,
						securityId: $drawingMenuProps.securityId
					},
					true
				);
			}
			
			// Keep the menu open
			// No need to reset selectedLine or active state
		}
	}

	function handleClickOutside(event: MouseEvent) {
		event.stopImmediatePropagation();
		if (!$drawingMenuProps.active || $drawingMenuProps.isDragging) {
			return;
		}
		if (!menuElement) return;

		const deleteButton = menuElement.querySelector('button');
		if (
			menuElement.contains(event.target as Node) ||
			(deleteButton && deleteButton.contains(event.target as Node))
		) {
			('clicked inside menu');
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

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			drawingMenuProps.update((v: DrawingMenuProps) => ({
				...v,
				active: false
			}));
		}
	}

	onMount(() => {
		document.addEventListener('mousedown', handleClickOutside);
		document.addEventListener('keydown', handleKeyDown);
		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
			document.removeEventListener('keydown', handleKeyDown);
		};
	});

	$: menuStyle = `
		left: ${$drawingMenuProps.clientX}px; 
		top: ${$drawingMenuProps.clientY}px;
		pointer-events: ${$drawingMenuProps.isDragging ? 'none' : 'auto'};
	`;

	// Add this computed property to format the price
	$: formattedPrice = $drawingMenuProps.selectedLinePrice !== undefined && 
						$drawingMenuProps.selectedLinePrice !== null ? 
						parseFloat($drawingMenuProps.selectedLinePrice).toFixed(2) : "0.00";
	
	// Handle input changes
	function handlePriceInput(e) {
		$drawingMenuProps.selectedLinePrice = parseFloat(e.target.value);
	}
</script>

{#if $drawingMenuProps.active && !$drawingMenuProps.isDragging}
	<div
		bind:this={menuElement}
		role="menu"
		on:mousedown={handleClickOutside}
		on:keydown={handleKeyDown}
		class="drawing-menu"
		style={menuStyle}
	>
		<button on:click={removePriceLine}>Delete</button>
		<div>
			<input
				value={formattedPrice}
				on:input={handlePriceInput}
				on:change={editHorizontalLinePrice}
				type="number"
				step="0.01"
			/>
		</div>
	</div>
{/if}

<style>
	.drawing-menu {
		position: absolute;
		z-index: 1002;
		background-color: rgba(0, 0, 0, 0.5);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 4px;
		padding: 4px;
	}
</style>
