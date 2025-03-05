<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	export let delayed = false; // Add this prop to control when to show the chart

	/* -------------------------------
	 * Interfaces & Config
	 * ------------------------------- */
	interface PricePoint {
		price: number;
		high: number;
		low: number;
	}

	// We'll maintain exactly this many data points in our chart
	const DATA_LENGTH = 30;

	// Update the chart constants for a more compact display
	const ROW_COUNT = 20;

	let data: PricePoint[] = [];
	let yAxisLabels = '';
	let chartContent = '';

	let frameId: number;

	// Add browser environment check
	const isBrowser = typeof window !== 'undefined';

	let visible = false; // Add this for fade control

	// Remove fixed CHART_MAX/MIN constants and make them dynamic
	let currentMin = 90;
	let currentMax = 110;
	let basePrice = 90;

	let moveCount = 0; // Track movement pattern
	let downPhase = false; // Track if we're in a down phase
	let downTicks = 0; // Track how long we've been moving down

	/* -------------------------------
	 * Helpers
	 * ------------------------------- */

	// Clamp a number to [min..max]
	function clamp(val: number, min: number, max: number) {
		return Math.max(min, Math.min(max, val));
	}

	/**
	 * Convert a price to a row index [0..ROW_COUNT - 1],
	 * where row=0 is the TOP (CHART_MAX) and row=ROW_COUNT-1 is the BOTTOM (CHART_MIN).
	 */
	function scaleValue(price: number): number {
		const STEP = (currentMax - currentMin) / (ROW_COUNT - 1);
		const row = (currentMax - price) / STEP;
		return Math.round(clamp(row, 0, ROW_COUNT - 1));
	}

	/**
	 * Build the ASCII chart given the current data array.
	 */
	function buildChart(points: PricePoint[]): void {
		const priceGrid: string[][] = Array.from({ length: ROW_COUNT }, () =>
			Array.from({ length: DATA_LENGTH }, () => ' ')
		);

		// Add y-axis line
		for (let row = 0; row < ROW_COUNT; row++) {
			priceGrid[row][0] = '│';
		}

		// Add x-axis line at the bottom
		for (let col = 0; col < DATA_LENGTH; col++) {
			priceGrid[ROW_COUNT - 1][col] = '─';
		}

		// Add corner where axes meet
		priceGrid[ROW_COUNT - 1][0] = '└';

		// Place price indicators only if they're within the current scale range
		points.forEach((p, col) => {
			// Skip null points or points below currentMin
			if (!p.price || p.price < currentMin) return;

			const rowP = scaleValue(p.price);

			// Only show movement arrows after the first valid point
			let char = '·';
			if (col > 0 && points[col - 1]?.price >= currentMin) {
				const prevPrice = points[col - 1].price;
				if (p.price > prevPrice) {
					char = '<span style="color: #00ff00">˄</span>';
				} else if (p.price < prevPrice) {
					char = '<span style="color: #ff0000">˅</span>';
				}
			}

			// Don't overwrite axis lines with price indicators
			if (col > 0) {
				priceGrid[rowP][col] = char;
			}
		});

		// Generate y-axis labels with current scale
		yAxisLabels = Array.from({ length: ROW_COUNT }, (_, r) => {
			const STEP = (currentMax - currentMin) / (ROW_COUNT - 1);
			const labelPrice = currentMax - r * STEP;
			return labelPrice.toFixed(2);
		}).join('\n');

		// Generate chart content
		chartContent = priceGrid.map((row) => row.join('')).join('\n') + '\n';
	}

	/**
	 * Generate a random PricePoint close to a previous point.
	 */
	function generateDataPoint(prev?: PricePoint, index?: number): PricePoint {
		if (index !== undefined) {
			// During initialization, start at base price
			const price = basePrice + Math.random() * 0.5;

			const high = price + Math.random() * 0.5;
			const low = price - Math.random() * 0.5;
			return { price, high, low };
		}

		// During animation
		const lastPrice = prev?.price ?? basePrice;

		// Create a more complex pattern
		moveCount = (moveCount + 1) % 8; // Longer pattern cycle

		// Decide if we should start a down phase
		if (moveCount === 0) {
			downPhase = Math.random() < 0.7; // 70% chance to start down phase
			downTicks = Math.floor(Math.random() * 3) + 2; // 2-4 down ticks
		}

		let change;
		if (downPhase && downTicks > 0) {
			// Down move phase
			const downwardBias = -1.0;
			const variation = Math.random() * 0.6;
			change = variation + downwardBias;
			downTicks--;

			if (downTicks === 0) {
				downPhase = false;
			}
		} else {
			// Up move phase
			const upwardBias = 1.8;
			const variation = Math.random() * 0.8;
			change = variation + upwardBias;
		}

		const price = lastPrice + change;

		// Smoother scale adjustment
		if (price > currentMax - 15) {
			const scaleFactor = (price - (currentMax - 15)) / 15;
			currentMin += scaleFactor * 1.2;
			currentMax += scaleFactor * 2.0;
		}

		const high = price + Math.random() * 0.5;
		const low = price - Math.random() * 0.5;
		return { price, high, low };
	}

	/**
	 * Main animation loop
	 * - occasionally shift the data and add a new point
	 * - rebuild the ASCII chart
	 */
	function animate() {
		if (!isBrowser) return;
		frameId = window.requestAnimationFrame(animate);

		// Update roughly 12 times per second
		if (Math.random() < 0.2) {
			// Instead of shifting and pushing, we'll maintain position at the right
			const newPoint = generateDataPoint(data[data.length - 1]);

			// Move all points one step left
			for (let i = 0; i < data.length - 1; i++) {
				data[i] = data[i + 1];
			}

			// Put new point at the end
			data[data.length - 1] = newPoint;

			buildChart([...data]); // Create new array to trigger update
		}
	}

	/* -------------------------------
	 * Lifecycle
	 * ------------------------------- */
	onMount(() => {
		if (!delayed) {
			visible = true;
		}

		// Initialize data array with empty points, filling from right to left
		data = Array(DATA_LENGTH)
			.fill(null)
			.map(() => ({
				price: 0,
				high: 0,
				low: 0
			}));

		// Delay the start of animation
		setTimeout(() => {
			// Start animation only in browser
			if (isBrowser) {
				animate();
			}
		}, 500);
	});

	onDestroy(() => {
		if (isBrowser && frameId) {
			window.cancelAnimationFrame(frameId);
		}
	});
</script>

<div class="container" class:visible>
	<div class="chart-container">
		<pre class="y-axis">{yAxisLabels}</pre>
		<pre class="chart">{@html chartContent}</pre>
	</div>
</div>

<style lang="postcss">
	.container {
		min-height: 100vh;
		width: 100%;
		background: transparent;
		color: #ffffff;
		font-family: monospace;
		display: flex;
		align-items: center;
		justify-content: center;
		box-sizing: border-box;
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		opacity: 0;
		transition: opacity 1s ease-in-out;
	}

	.container.visible {
		opacity: 1;
	}

	/* Wrap the y-axis and chart together in an inline-flex container */
	.chart-container {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		background: transparent;
		padding: 0 2rem;
		transform: scale(1.5); /* Make the chart slightly larger */
	}

	.y-axis {
		font-size: 18px;
		line-height: 1.1em;
		margin: 0;
		padding-right: 4px;
		white-space: pre;
		font-variant-numeric: tabular-nums;
		text-align: right;
		flex-shrink: 0;
		min-width: 90px;
		color: #ffffff;
	}

	.chart {
		font-size: 18px;
		line-height: 1.1em;
		margin: 0;
		white-space: pre;
		overflow-x: visible; /* Changed from auto to visible */
		text-align: left;
		color: #3b82f6; /* Match the blue theme */
	}

	/* Responsive adjustments */
	@media (max-width: 768px) {
		.container {
			padding: 0.5rem;
		}

		.chart-container {
			padding: 0 1rem;
		}
	}
</style>
