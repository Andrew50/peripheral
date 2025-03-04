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
		horizontalLines: { 
			price: number; 
			line: IPriceLine; 
			id: number; 
			color: string; 
			lineWidth: number
		}[];
		isDragging: boolean;
		securityId: number;
		selectedLinePrice: number;
		selectedLineColor: string;
		selectedLineWidth: number;
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
		selectedLineColor: '#FFFFFF',
		selectedLineWidth: 1,
		securityId: -1
	});
	
	export function addHorizontalLine(price: number, securityId: number, id: number = -1, color: string = '#FFFFFF', lineWidth: number = 1) {
		price = parseFloat(price.toFixed(2));
		const priceLine = get(drawingMenuProps).chartCandleSeries.createPriceLine({
			price: price,
			color: color,
			lineWidth: lineWidth,
			lineStyle: 0, // Solid line
			axisLabelVisible: true,
			title: `Price: ${price}`
		});
		get(drawingMenuProps).horizontalLines.push({
			id,
			price,
			line: priceLine,
			color,
			lineWidth
		});
		if (id == -1) {
			// only add to backend if it's being added not from a ticker load but from a new added line
			privateRequest<number>('setHorizontalLine', {
				price: price,
				securityId: securityId,
				color: color,
				lineWidth: lineWidth
			}).then((res: number) => {
				get(drawingMenuProps).horizontalLines[get(drawingMenuProps).horizontalLines.length - 1].id = res;
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
	
	// Common colors for lines
	const colorPresets = [
		'#FFFFFF', // White
		'#FF0000', // Red
		'#00FF00', // Green
		'#0000FF', // Blue
		'#FFFF00', // Yellow
		'#FF00FF', // Magenta
		'#00FFFF', // Cyan
		'#FFA500'  // Orange
	];
	
	// Line width options
	const lineWidthOptions = [1, 2, 3, 4, 5];

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

	function updateHorizontalLine() {
		if ($drawingMenuProps.selectedLine !== null) {
			// Update the existing line with all properties
			const price = parseFloat($drawingMenuProps.selectedLinePrice.toFixed(2));
			const color = $drawingMenuProps.selectedLineColor;
			const lineWidth = $drawingMenuProps.selectedLineWidth;
			
			$drawingMenuProps.selectedLine.applyOptions({
				price,
				color,
				lineWidth,
				title: `Price: ${price}`
			});
			
			// Update the stored properties in horizontalLines array
			const lineIndex = $drawingMenuProps.horizontalLines.findIndex(
				(line) => line.line === $drawingMenuProps.selectedLine
			);
			
			if (lineIndex !== -1) {
				$drawingMenuProps.horizontalLines[lineIndex].price = price;
				$drawingMenuProps.horizontalLines[lineIndex].color = color;
				$drawingMenuProps.horizontalLines[lineIndex].lineWidth = lineWidth;
				
				// Update in backend
				privateRequest<void>(
					'updateHorizontalLine',
					{
						id: $drawingMenuProps.horizontalLines[lineIndex].id,
						price,
						color,
						lineWidth,
						securityId: $drawingMenuProps.securityId
					},
					true
				);
			}
			
			// Keep the menu open
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
	
	function handleColorChange(e) {
		$drawingMenuProps.selectedLineColor = e.target.value;
		updateHorizontalLine();
	}
	
	function handleLineWidthChange(e) {
		$drawingMenuProps.selectedLineWidth = parseInt(e.target.value);
		updateHorizontalLine();
	}
	
	function selectColorPreset(color: string) {
		$drawingMenuProps.selectedLineColor = color;
		updateHorizontalLine();
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
		<button on:click={removePriceLine} class="delete-button">Delete</button>
		
		<div class="menu-section">
			<label for="line-price">Price:</label>
			<div class="price-input-container">
				<input
					id="line-price"
					value={formattedPrice}
					on:input={handlePriceInput}
					on:change={updateHorizontalLine}
					type="number"
					step="0.01"
					class="price-input"
				/>
			</div>
		</div>
		
		<div class="menu-section">
			<label for="line-width">Width:</label>
			<select 
				id="line-width" 
				value={$drawingMenuProps.selectedLineWidth} 
				on:change={handleLineWidthChange}
				class="line-width-select"
			>
				{#each lineWidthOptions as width}
					<option value={width}>{width}px</option>
				{/each}
			</select>
		</div>
		
		<div class="menu-section">
			<label for="line-color">Color:</label>
			<input 
				id="line-color"
				type="color" 
				value={$drawingMenuProps.selectedLineColor} 
				on:change={handleColorChange}
				class="color-picker"
			/>
		</div>
		
		<div class="color-presets">
			{#each colorPresets as color}
				<div 
					class="color-preset" 
					style="background-color: {color}; border: 2px solid {$drawingMenuProps.selectedLineColor === color ? '#4CAF50' : 'transparent'}" 
					on:click={() => selectColorPreset(color)} 
					role="button" 
					tabindex="0" 
					on:keydown={(e) => e.key === 'Enter' && selectColorPreset(color)}
				></div>
			{/each}
		</div>
		
		<div class="preview-section">
			<div class="line-preview" style="height: {$drawingMenuProps.selectedLineWidth}px; background-color: {$drawingMenuProps.selectedLineColor};"></div>
		</div>
	</div>
{/if}

<style>
	.drawing-menu {
		position: absolute;
		z-index: 1002;
		background-color: rgba(20, 20, 20, 0.9);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
		padding: 10px;
		width: 200px;
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
	}
	
	.menu-section {
		margin-bottom: 10px;
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	
	label {
		font-size: 12px;
		color: #ccc;
		margin-right: 8px;
	}
	
	.delete-button {
		width: 100%;
		background-color: rgba(220, 53, 69, 0.7);
		color: white;
		border: none;
		border-radius: 4px;
		padding: 6px;
		margin-bottom: 10px;
		cursor: pointer;
		transition: background-color 0.2s;
	}
	
	.delete-button:hover {
		background-color: rgba(220, 53, 69, 1);
	}
	
	.price-input-container {
		flex: 1;
	}
	
	.price-input, .line-width-select {
		width: 100%;
		background-color: rgba(30, 30, 30, 0.7);
		color: white;
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		padding: 5px;
		font-size: 12px;
	}
	
	.color-picker {
		width: 30px;
		height: 30px;
		border: none;
		background: none;
		cursor: pointer;
	}
	
	.color-presets {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
		margin-bottom: 10px;
	}
	
	.color-preset {
		width: 20px;
		height: 20px;
		border-radius: 50%;
		cursor: pointer;
		transition: transform 0.1s;
	}
	
	.color-preset:hover {
		transform: scale(1.1);
	}
	
	.preview-section {
		margin-top: 10px;
		padding: 8px;
		background-color: rgba(0, 0, 0, 0.3);
		border-radius: 4px;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	
	.line-preview {
		width: 100%;
		min-height: 1px;
	}
</style>
