<!-- drawingMenu.svelte -->
<script lang="ts" context="module">
	import type { ISeriesApi, IPriceLine, LineWidth, LineStyle } from 'lightweight-charts';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';

	export interface DrawingMenuProps {
		chartCandleSeries: ISeriesApi<'Candlestick'> | null;
		selectedLine: IPriceLine | null;
		clientX: number;
		clientY: number;
		active: boolean;
		horizontalLines: {
			price: number;
			line: IPriceLine;
			id: number;
			color: string;
			lineWidth: LineWidth;
		}[];
		isDragging: boolean;
		selectedLinePrice: number;
		selectedLineColor: string;
		selectedLineWidth: LineWidth;
		selectedLineId?: number;
		securityId?: number;
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
		selectedLineWidth: 1 as LineWidth,
		securityId: -1
	});

	export function addHorizontalLine(
		price: number,
		securityId: number,
		id: number = -1,
		color: string = '#FFFFFF',
		lineWidth: LineWidth = 1 as LineWidth
	) {
		price = parseFloat(price.toFixed(2));

		if (id == -1) {
			// This is a user-initiated line creation (Alt+H) - add to backend and store
			privateRequest<number>('setHorizontalLine', {
				price: price,
				securityId: securityId,
				color: color,
				lineWidth: lineWidth
			}).then((res: number) => {
				// Update the store with the new line - reactive block will handle rendering
				horizontalLines.update((lines) => {
					const newLine = {
						id: res,
						securityId: securityId,
						price: price,
						color: color,
						lineWidth: lineWidth
					};

					// Check if line already exists to avoid duplicates
					const existingIndex = lines.findIndex((line) => line.id === res);
					if (existingIndex !== -1) {
						// Update existing line
						lines[existingIndex] = newLine;
						return [...lines];
					} else {
						// Add new line
						return [...lines, newLine];
					}
				});
			});
		} else {
			// This is for lines loaded from backend - they should already be in the store
			// This branch is now primarily for backwards compatibility
			// The reactive block should handle the actual rendering
			console.warn(
				'addHorizontalLine called with existing ID - this should be handled by the reactive block'
			);
		}
	}
</script>

<script lang="ts">
	import '$lib/styles/global.css';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { horizontalLines } from '$lib/utils/stores/stores';
	export let drawingMenuProps: Writable<DrawingMenuProps>;

	let menuElement: HTMLDivElement;
	let adjustedMenuStyle: string = '';

	// Common colors for lines
	const colorPresets = [
		'#FFFFFF', // White
		'#FF0000', // Red
		'#00FF00', // Green
		'#0000FF', // Blue
		'#FFFF00', // Yellow
		'#FF00FF', // Magenta
		'#00FFFF', // Cyan
		'#FFA500' // Orange
	];

	// Line width options
	const lineWidthOptions = [1, 2, 3, 4, 5];

	function removePriceLine(event: MouseEvent) {
		event.preventDefault();
		event.stopImmediatePropagation();
		if ($drawingMenuProps.selectedLine !== null) {
			deleteHorizontalLine();
		}
	}

	function deleteHorizontalLine() {
		if (!$drawingMenuProps.selectedLine || !$drawingMenuProps.chartCandleSeries) {
			return;
		}

		// Find the line ID before removing it
		const lineIndex = $drawingMenuProps.horizontalLines.findIndex(
			(line) => line.line === $drawingMenuProps.selectedLine
		);

		let deletedLineId = -1;

		// If the line has an ID, delete it from the server
		if (lineIndex >= 0 && $drawingMenuProps.horizontalLines[lineIndex].id > 0) {
			deletedLineId = $drawingMenuProps.horizontalLines[lineIndex].id;
			privateRequest('deleteHorizontalLine', {
				id: deletedLineId
			});

			// Update the store to remove the line - reactive block will handle chart removal
			horizontalLines.update((lines) => lines.filter((line) => line.id !== deletedLineId));
		}

		// Close the menu
		$drawingMenuProps.active = false;
		$drawingMenuProps.selectedLine = null;
	}

	function updateHorizontalLine() {
		if (!$drawingMenuProps.selectedLine || !$drawingMenuProps.chartCandleSeries) {
			return;
		}

		// Update the existing line with all properties
		const price = parseFloat($drawingMenuProps.selectedLinePrice.toFixed(2));
		const color = $drawingMenuProps.selectedLineColor;
		const lineWidth = $drawingMenuProps.selectedLineWidth;

		// Find the line ID from the local state
		const lineIndex = $drawingMenuProps.horizontalLines.findIndex(
			(line) => line.line === $drawingMenuProps.selectedLine
		);

		if (lineIndex !== -1) {
			const lineId = $drawingMenuProps.horizontalLines[lineIndex].id;
			const securityId = $drawingMenuProps.securityId;

			// Update in backend
			privateRequest<void>(
				'updateHorizontalLine',
				{
					id: lineId,
					price,
					color,
					lineWidth,
					securityId: securityId
				},
				true
			);

			// Update the store - reactive block will handle chart updates
			if (lineId > 0 && securityId) {
				horizontalLines.update((lines) =>
					lines.map((line) => (line.id === lineId ? { ...line, price, color, lineWidth } : line))
				);
			}
		}

		// Keep the menu open
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

	// Function to position the menu within chart boundaries
	function positionMenuWithinChart() {
		if (!menuElement) return;

		// Get the chart container
		const chartContainer =
			menuElement.closest('.chart-wrapper') || document.querySelector('.chart-wrapper');
		if (!chartContainer) return;

		// Get the dimensions of the chart container
		const chartRect = chartContainer.getBoundingClientRect();

		// Menu dimensions
		const menuWidth = menuElement.offsetWidth;
		const menuHeight = menuElement.offsetHeight;

		// Calculate the proposed position
		let menuX = $drawingMenuProps.clientX;
		let menuY = $drawingMenuProps.clientY;

		// Check if menu would overflow right side
		if (menuX + menuWidth > chartRect.right) {
			menuX = chartRect.right - menuWidth - 10; // 10px padding
		}

		// Check if menu would overflow left side
		if (menuX < chartRect.left) {
			menuX = chartRect.left + 10; // 10px padding
		}

		// Check if menu would overflow bottom
		if (menuY + menuHeight > chartRect.bottom) {
			menuY = chartRect.bottom - menuHeight - 10; // 10px padding
		}

		// Check if menu would overflow top
		if (menuY < chartRect.top) {
			menuY = chartRect.top + 10; // 10px padding
		}

		// Update the position
		adjustedMenuStyle = `
			left: ${menuX}px; 
			top: ${menuY}px;
			pointer-events: ${$drawingMenuProps.isDragging ? 'none' : 'auto'};
			max-width: ${chartRect.width - 20}px; 
			max-height: ${chartRect.height - 20}px;
		`;
	}

	onMount(() => {
		document.addEventListener('mousedown', handleClickOutside);
		document.addEventListener('keydown', handleKeyDown);

		// Position the menu when it becomes active
		const unsubscribe = drawingMenuProps.subscribe((props) => {
			if (props.active && !props.isDragging && menuElement) {
				// Wait for the next tick to ensure menu is rendered
				setTimeout(positionMenuWithinChart, 0);
			}
		});

		return () => {
			document.removeEventListener('mousedown', handleClickOutside);
			document.removeEventListener('keydown', handleKeyDown);
			unsubscribe();
		};
	});

	// Add this computed property to format the price
	$: formattedPrice =
		$drawingMenuProps.selectedLinePrice !== undefined &&
		$drawingMenuProps.selectedLinePrice !== null
			? parseFloat($drawingMenuProps.selectedLinePrice.toString()).toFixed(2)
			: '0.00';

	// Position menu whenever relevant properties change
	$: if ($drawingMenuProps.active && !$drawingMenuProps.isDragging && menuElement) {
		positionMenuWithinChart();
	}

	// Handle input changes
	function handlePriceInput(e: Event) {
		$drawingMenuProps.selectedLinePrice = parseFloat((e.target as HTMLInputElement).value);
	}

	function handleColorChange(e: Event) {
		$drawingMenuProps.selectedLineColor = (e.target as HTMLInputElement).value;
		updateHorizontalLine();
	}

	function handleLineWidthChange(e: Event) {
		$drawingMenuProps.selectedLineWidth = parseInt(
			(e.target as HTMLInputElement).value
		) as LineWidth;
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
		tabindex="0"
		on:mousedown={handleClickOutside}
		on:keydown={handleKeyDown}
		class="drawing-menu"
		style={adjustedMenuStyle}
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
					style="background-color: {color}; border: 2px solid {$drawingMenuProps.selectedLineColor ===
					color
						? '#4CAF50'
						: 'transparent'}"
					on:click={() => selectColorPreset(color)}
					role="button"
					tabindex="0"
					on:keydown={(e) => e.key === 'Enter' && selectColorPreset(color)}
				></div>
			{/each}
		</div>

		<div class="preview-section">
			<div
				class="line-preview"
				style="height: {$drawingMenuProps.selectedLineWidth}px; background-color: {$drawingMenuProps.selectedLineColor};"
			></div>
		</div>
	</div>
{/if}

<style>
	.drawing-menu {
		position: absolute;
		z-index: 9000;
		background-color: rgba(20, 20, 20, 0.9);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
		padding: 10px;
		width: 200px;
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
		overflow: auto; /* Add scrolling if content becomes too large */
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

	.price-input,
	.line-width-select {
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
