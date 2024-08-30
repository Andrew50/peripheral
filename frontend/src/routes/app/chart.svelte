<script lang="ts">
import { createChart, ColorType} from 'lightweight-charts';
import {privateRequest} from '../../store';
import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon} from 'lightweight-charts';
import { onMount } from 'svelte';
let mainChart: IChartApi;
let mainChartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
let currentTicker: string;
let currentTimeframe: string; 
interface barData {
    time: string;
    open: number; 
    high: number;
    low: number;
    close: number;
}
function initializeChart()  {
    const chartOptions = { layout: { textColor: 'black', background: { type: ColorType.Solid, color: 'white' } } };
    const chartContainer = document.getElementById('chart_container');
    if (chartContainer) {
        chartContainer.addEventListener('keydown', event => {
            if (/^[a-zA-Z]$/.test(event.key)) {
                event.preventDefault();
                currentTicker += event.key.toUpperCase();
                // Perform action for any letter key
            } else if (event.key === 'Backspace') {
                event.preventDefault();
                currentTicker = currentTicker.slice(0, -1);
            } else if (event.key === 'Enter') {
                event.preventDefault();
                updateChart(mainChart);
            }
            
         });
        mainChart = createChart(chartContainer, chartOptions);

        mainChartCandleSeries = mainChart.addCandlestickSeries({
            upColor: '#26a69a', downColor: '#ef5350', borderVisible: false,
            wickUpColor: '#26a69a', wickDownColor: '#ef5350',
        });
        const candleData = [
            {time: "2024-08-12", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-13", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-14", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-15", open: 1, high: 2, low: 0.5, close: 2},
            {time: "2024-08-16", open: 1, high: 2, low: 0.5, close: 2},
        ]

        mainChartCandleSeries.setData(candleData);
        mainChart.timeScale().fitContent();
    }


}
function updateChart(chart: IChartApi) {
    console.log(currentTicker)
    let barDataList: barData[] = []
    privateRequest<barData[]>("getChartData", {ticker:currentTicker, timeframe:currentTimeframe})
        .then((result: barData[]) => {
            barDataList = result;

            let newData = [];
            for (let i =0; i < barDataList.length; i++) {
                newData.push({
                    time: barDataList[i].time, 
                    open: barDataList[i].open, 
                    high: barDataList[i].high, 
                    low: barDataList[i].low,
                    close: barDataList[i].close
                });
            }
           
            console.log(newData)
            mainChart.removeSeries(mainChartCandleSeries)
            mainChartCandleSeries = mainChart.addCandlestickSeries({
                    upColor: '#26a69a', downColor: '#ef5350', borderVisible: false,
                    wickUpColor: '#26a69a', wickDownColor: '#ef5350',
                })
            mainChartCandleSeries.setData(newData)
            mainChart.timeScale().fitContent();
        })
        .catch((error: string) => {
            console.error("Error fetching chart data:", error);
        });

}
    onMount(() => {
    currentTicker = ""
    initializeChart(); // Optionally call Chart function on component mount
  });
</script>
<style>
    #chart_container {
      width: 85%;
      height: 800px; /* Adjust height as needed */
    }
  </style>
<p> Lightweight Charts</p>
<input bind:value={currentTicker} placeholder="ticker"/>
<input bind:value={currentTimeframe} placeholder="timeframe"/>
<button on:click={() => updateChart(mainChart)}>Get Data</button>
<div id="chart_container" tabindex="0"></div>