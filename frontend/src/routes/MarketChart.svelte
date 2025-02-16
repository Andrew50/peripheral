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
  const CHART_MAX = 110.00;
  const CHART_MIN = 90.00;
  const ROW_COUNT = 20;

  // Derived "step" between each row
  const STEP = (CHART_MAX - CHART_MIN) / (ROW_COUNT - 1);

  let data: PricePoint[] = [];
  let yAxisLabels = '';
  let chartContent = '';

  let frameId: number;

  // Add browser environment check
  const isBrowser = typeof window !== 'undefined';

  let visible = false; // Add this for fade control

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
    // row = how far the price is below CHART_MAX, divided by STEP
    const row = (CHART_MAX - price) / STEP;
    return Math.round(clamp(row, 0, ROW_COUNT - 1));
  }

  /**
   * Build the ASCII chart given the current data array.
   */
  function buildChart(points: PricePoint[]): void {
    // Create two separate grids - one for lines and one for price indicators
    const linesGrid: string[][] = Array.from({ length: ROW_COUNT }, () =>
      Array.from({ length: DATA_LENGTH }, () => '│')
    );
    const priceGrid: string[][] = Array.from({ length: ROW_COUNT }, () =>
      Array.from({ length: DATA_LENGTH }, () => ' ')
    );

    // First pass: place vertical lines
    points.forEach((p, col) => {
      const rowP = scaleValue(p.price);
      
      // Clear vertical lines below the price point
      for (let r = rowP; r < ROW_COUNT; r++) {
        linesGrid[r][col] = ' ';
      }
    });

    // Second pass: place price indicators
    points.forEach((p, col) => {
      const rowP = scaleValue(p.price);

      // Add single arrow or dot for price movement
      let char = '·';
      if (col > 0) {
        const prevPrice = points[col - 1].price;
        if (p.price > prevPrice) {
          char = '˄';
        } else if (p.price < prevPrice) {
          char = '˅';
        }
      }
      
      // Place single price indicator at the price point
      priceGrid[rowP][col] = char;
    });

    // Generate y-axis labels separately
    yAxisLabels = Array.from({ length: ROW_COUNT }, (_, r) => {
      const labelPrice = CHART_MAX - r * STEP;
      return labelPrice.toFixed(2);
    }).join('\n');

    // Generate chart content separately
    const chartLines = Array.from({ length: ROW_COUNT }, (_, r) => {
      const verticalLines = linesGrid[r].join('');
      const priceIndicators = priceGrid[r].join('');
      
      // Overlay price indicators on top of vertical lines
      let finalLine = '';
      for (let i = 0; i < DATA_LENGTH; i++) {
        finalLine += priceIndicators[i] === ' ' ? verticalLines[i] : priceIndicators[i];
      }
      
      return finalLine;
    });

    chartContent = chartLines.join('\n') + '\n';
  }

  /**
   * Generate a random PricePoint close to a previous point.
   */
  function generateDataPoint(prev?: PricePoint): PricePoint {
    const lastPrice = prev?.price ?? 100;
    // small random change in [-1..1]
    const change = (Math.random() - 0.5) * 2;
    const price = lastPrice + change;

    // random offset for high/low
    const high = price + Math.random() * 1.5;
    const low = price - Math.random() * 1.5;
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

    if (Math.random() < 0.3) {
      data.shift();
      data.push(generateDataPoint(data[data.length - 1]));
    }

    buildChart(data);
  }

  /* -------------------------------
   * Lifecycle
   * ------------------------------- */
  onMount(() => {
    if (!delayed) {
      visible = true;
    }
    
    // Initialize data array
    let p: PricePoint = { price: 100, high: 101, low: 99 };
    data = [];
    for (let i = 0; i < DATA_LENGTH; i++) {
      p = generateDataPoint(p);
      data.push(p);
    }

    // Start animation only in browser
    if (isBrowser) {
      animate();
    }
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
    <pre class="chart">{chartContent}</pre>
  </div>
</div>

<style>
.container {
  min-height: 100vh;
  width: 100%;
  background: black;
  color: #00ff00;
  font-family: monospace;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
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
  gap: 0.5rem; /* Adjust gap as needed */
  background: transparent;
  padding: 0 2rem;
}

.y-axis {
  font-size: 12px;
  line-height: 1.1em;
  margin: 0;
  padding-right: 4px;
  white-space: pre;
  font-variant-numeric: tabular-nums;
  text-align: right;
  flex-shrink: 0;
  min-width: 60px;
}

.chart {
  font-size: 12px;
  line-height: 1.1em;
  margin: 0;
  white-space: pre;
  overflow-x: auto;
  /* You can adjust alignment here; for example, center or left */
  text-align: left;
  max-width: 1200px;
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