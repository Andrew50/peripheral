<script lang="ts">
import { createChart, ColorType } from 'lightweight-charts';
import { onMount } from 'svelte';
function Chart() {
    console.log("Working")
    const chartOptions = { layout: { textColor: 'black', background: { type: ColorType.Solid, color: 'white' } } };
    const chartContainer = document.getElementById('chart_container');
    if (chartContainer) {
        const chart = createChart(chartContainer, chartOptions);

        const candleSeries = chart.addCandlestickSeries({
            upColor: '#26a69a', downColor: '#ef5350', borderVisible: false,
            wickUpColor: '#26a69a', wickDownColor: '#ef5350',
        });
        candleSeries.setData([
            {time: "2024-08-12", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-13", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-14", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-15", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-16", open: 1, high: 2, low: 0.5, close: 2},
        ]);
        chart.timeScale().fitContent();
    }


}
    onMount(() => {
    Chart(); // Optionally call Chart function on component mount
  });
</script>
<style>
    #chart_container {
      width: 100%;
      height: 400px; /* Adjust height as needed */
    }
  </style>
<p> Lightweight Charts</p>
<button on:click={Chart}>Test</button>
<div id="chart_container"></div>