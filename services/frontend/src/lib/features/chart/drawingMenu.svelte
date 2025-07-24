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
		alertLines: {
			price: number;
			line: IPriceLine;
			alertId: number;
		}[];
		isDragging: boolean;
		selectedLinePrice: number;
		selectedLineColor: string;
		selectedLineWidth: LineWidth;
		selectedLineId?: number;
		selectedLineType?: 'horizontal' | 'alert' | null;
		securityId?: number;
	}

	export let drawingMenuProps: Writable<DrawingMenuProps> = writable({
		chartCandleSeries: null,
		selectedLine: null,
		clientX: 0,
		clientY: 0,
		active: false,
		selectedLineId: -1,
		selectedLineType: null,
		horizontalLines: [],
		alertLines: [],
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
			// console.warn(
			// 	'addHorizontalLine called with existing ID - this should be handled by the reactive block'
			// );
		}
	}
</script>

<script lang="ts">
	import '$lib/styles/global.css';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { horizontalLines, activeAlerts } from '$lib/utils/stores/stores';
	export let drawingMenuProps: Writable<DrawingMenuProps>;

	// Helper function to convert viewport coordinates to chart-relative coordinates
	function getRelativeMouseY(event: MouseEvent): number | null {
		if (!$drawingMenuProps.chartCandleSeries) return null;

		// Find the chart container - we need to look for it dynamically since we don't have chartId here
		// Look for the closest chart container from the event target
		let chartContainer: HTMLElement | null = null;

		// First try to find the chart container by traversing up from the event target
		let target = event.target as HTMLElement | null;
		while (target && target !== document.body) {
			if (target.id && target.id.startsWith('chart_container-')) {
				chartContainer = target;
				break;
			}
			target = target.parentElement;
		}

		// Fallback: search for any chart container
		if (!chartContainer) {
			chartContainer = document.querySelector('.chart[id*="chart_container"]') as HTMLElement;
		}

		if (!chartContainer) return null;

		const rect = chartContainer.getBoundingClientRect();
		return event.clientY - rect.top;
	}

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

	function updateAlertPrice() {
		if (
			!$drawingMenuProps.selectedLine ||
			!$drawingMenuProps.chartCandleSeries ||
			$drawingMenuProps.selectedLineType !== 'alert'
		) {
			return;
		}

		const alertId = $drawingMenuProps.selectedLineId;
		if (!alertId || alertId <= 0) return;

		const newPrice = parseFloat(
			parseFloat($drawingMenuProps.selectedLinePrice.toString()).toFixed(2)
		);

		// Optimistically update the line on the chart
		$drawingMenuProps.selectedLine.applyOptions({ price: newPrice });

		// Update the activeAlerts store
		activeAlerts.update((alerts) => {
			if (!alerts) return [];
			return alerts.map((alert) =>
				alert.alertId === alertId ? { ...alert, alertPrice: newPrice } : alert
			);
		});

		// Update backend
		privateRequest<void>('updateAlert', { alertId, price: newPrice }, true).catch((err) => {
			console.error('Failed to update alert price:', err);
			// Revert on failure if needed
		});
	}

	function deleteHorizontalLine() {
		if (!$drawingMenuProps.selectedLine || !$drawingMenuProps.chartCandleSeries) {
			return;
		}

		if ($drawingMenuProps.selectedLineType === 'horizontal') {
			// Handle horizontal line deletion
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
		} else if ($drawingMenuProps.selectedLineType === 'alert') {
			// Handle alert deletion
			const alertId = $drawingMenuProps.selectedLineId;
			if (alertId && alertId > 0) {
				privateRequest('deleteAlert', {
					alertId: alertId
				});

				// Update the activeAlerts store to remove the alert
				activeAlerts.update((alerts) =>
					alerts ? alerts.filter((alert) => alert.alertId !== alertId) : []
				);
			}
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
		// console.log('🖱️ handleClickOutside called');
		event.stopImmediatePropagation();
		if (!$drawingMenuProps.active || $drawingMenuProps.isDragging) {
			// console.log('❌ Menu not active or dragging, returning early');
			return;
		}
		if (!menuElement) {
			// console.log('❌ No menu element, returning early');
			return;
		}

		const deleteButton = menuElement.querySelector('button');
		if (
			menuElement.contains(event.target as Node) ||
			(deleteButton && deleteButton.contains(event.target as Node))
		) {
			// console.log('✅ Clicked inside menu, keeping menu open');
			return;
		}

		const relativeY = getRelativeMouseY(event);
		const isClickInMenu =
			event.target === menuElement || menuElement.contains(event.target as Node);

		const selectedLine = $drawingMenuProps.selectedLine;
		const chartCandleSeries = $drawingMenuProps.chartCandleSeries;
		let isClickNearLine = false;

		if (selectedLine && chartCandleSeries && relativeY !== null) {
			const linePrice = selectedLine.options().price;
			const lineY = chartCandleSeries.priceToCoordinate(linePrice) || 0;
			const CLICK_THRESHOLD = 5; // pixels
			isClickNearLine = Math.abs(relativeY - lineY) <= CLICK_THRESHOLD;
			// console.log('🔍 Click near line check:', {
			// 	relativeY,
			// 	lineY,
			// 	distance: Math.abs(relativeY - lineY),
			// 	threshold: CLICK_THRESHOLD,
			// 	isClickNearLine
			// });
		}

		if (!isClickInMenu && !isClickNearLine) {
			// console.log('❌ Click outside menu and not near line, closing menu');
			drawingMenuProps.update((v: DrawingMenuProps) => ({
				...v,
				active: false
			}));
		} else {
			// console.log('✅ Click near line or in menu, keeping menu open');
		}
	}

	function handleKeyDown(event: KeyboardEvent) {
		// Prevent keyboard events from bubbling up to the chart container
		event.stopPropagation();

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

		// Get the chart container - try both selectors for compatibility
		const chartContainer =
			menuElement.closest('.chart') ||
			menuElement.closest('.chart-wrapper') ||
			document.querySelector('.chart') ||
			document.querySelector('.chart-wrapper');

		if (!chartContainer) {
			// console.warn(
			// 	'Could not find chart container for menu positioning, using fallback positioning'
			// );
			// Fallback: position relative to viewport with some padding
			const viewportWidth = window.innerWidth;
			const viewportHeight = window.innerHeight;

			let menuX = $drawingMenuProps.clientX;
			let menuY = $drawingMenuProps.clientY;

			// Ensure menu stays within viewport bounds
			if (menuX + 200 > viewportWidth) {
				// 200px is menu width
				menuX = viewportWidth - 220; // 200px width + 20px padding
			}
			if (menuY + 300 > viewportHeight) {
				// Estimate menu height
				menuY = viewportHeight - 320; // 300px height + 20px padding
			}
			if (menuX < 10) menuX = 10;
			if (menuY < 10) menuY = 10;

			adjustedMenuStyle = `
				position: fixed;
				left: ${menuX}px; 
				top: ${menuY}px;
				pointer-events: ${$drawingMenuProps.isDragging ? 'none' : 'auto'};
			`;
			return;
		}

		// Get the dimensions of the chart container
		const chartRect = chartContainer.getBoundingClientRect();

		// Menu dimensions
		const menuWidth = menuElement.offsetWidth;
		const menuHeight = menuElement.offsetHeight;

		// Calculate the proposed position using viewport coordinates (for fixed positioning)
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

		// Update the position with fixed positioning
		adjustedMenuStyle = `
			position: fixed;
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

	// Debug logging for drawingMenuProps changes
	$: {
		// console.log('📊 drawingMenuProps changed:', {
		// 	active: $drawingMenuProps.active,
		// 	isDragging: $drawingMenuProps.isDragging,
		// 	selectedLine: $drawingMenuProps.selectedLine ? 'exists' : 'null',
		// 	selectedLineId: $drawingMenuProps.selectedLineId
		// });
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
		<button on:click={removePriceLine} class="delete-button">
			{#if $drawingMenuProps.selectedLineType === 'alert'}
				Delete Alert
			{:else}
				Delete
			{/if}
		</button>

		{#if $drawingMenuProps.selectedLineType === 'horizontal'}
			<!-- Full menu for horizontal lines -->
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
		{:else if $drawingMenuProps.selectedLineType === 'alert'}
			<!-- Simplified menu for alerts -->
			<div class="menu-section">
				<label for="alert-price">Price:</label>
				<div class="price-input-container">
					<input
						id="alert-price"
						value={formattedPrice}
						on:input={handlePriceInput}
						on:change={updateAlertPrice}
						type="number"
						step="0.01"
						class="price-input"
					/>
				</div>
			</div>
		{/if}
	</div>
{/if}

<style>
	.drawing-menu {
		/* Position will be set dynamically via inline styles */
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

	.alert-info {
		padding: 8px 0 0 0;
		text-align: center;
	}

	.alert-price {
		font-size: 14px;
		font-weight: bold;
		color: #ffb74d; /* Orange color matching alert lines */
		margin-bottom: 4px;
	}

	.alert-instruction {
		font-size: 11px;
		color: #ccc;
		font-style: italic;
	}
</style>
