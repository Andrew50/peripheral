<script lang="ts">
import { createChart, ColorType} from 'lightweight-charts';
import {privateRequest} from '../../store';
import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, MouseEventParams, UTCTimestamp} from 'lightweight-charts';
import { onMount, onDestroy } from 'svelte';

let mainChart: IChartApi;
let mainChartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
let currentTicker: string;
let currentTimeframe: string; 
let latestCrosshairPositionTime: Time;
// Right Click context menu variables 
let showMenu = false; 
let menuStyle = {
    top: '0px', 
    left: '0px'
};
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
        mainChart.subscribeCrosshairMove(crosshairMoveEvent);
        mainChart.timeScale().fitContent();
    }


}
function crosshairMoveEvent(param: MouseEventParams) {
    if (!param.point) {
        return;
    }
    const validCrosshairPoint = !(param === undefined || param.time === undefined || param.point.x < 0 || param.point.y < 0);
    if(!validCrosshairPoint) { return; }
    if(!mainChartCandleSeries) {return;}

    const bar = param.seriesData.get(mainChartCandleSeries)
    if(!bar) {return;}
    latestCrosshairPositionTime = bar.time 

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
function chartRightClick(event: MouseEvent) {
    event.preventDefault();
    console.log("Chart right clicked")
    console.log(latestCrosshairPositionTime)
    menuStyle = {
            top: `${event.clientY + 10}px`,
            left: `${event.clientX + 10}px`
        };
    showMenu = true;
    


}
function closeRightClickMenu() {
    showMenu = false;
}
onMount(() => {
    currentTicker = ""
    initializeChart(); 
    const chartContainer = document.getElementById('chart_container');
    if (chartContainer) {
        chartContainer.addEventListener('contextmenu', chartRightClick)
    }
    return () => {
        if (chartContainer) {
                chartContainer.removeEventListener('contextmenu', chartRightClick);
            }
    };

});
document.addEventListener('click', closeRightClickMenu)
onDestroy(() => {
        document.removeEventListener('click', closeRightClickMenu)
})
</script>
<style>
    #chart_container {
      width: 85%;
      height: 800px; /* Adjust height as needed */
    }
    .context-menu {
        position: absolute;
        background-color: white;
        border: 1px solid #ccc;
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        padding: 10px;
        display: flex;
        flex-direction: column;
    }

    .context-menu-item {
        padding: 5px 10px;
        cursor: pointer;
    }

    .context-menu-item:hover {
        background-color: #f0f0f0;
    }
</style>
<p> Lightweight Charts</p>
<input bind:value={currentTicker} placeholder="ticker"/>
<input bind:value={currentTimeframe} placeholder="timeframe"/>
<button on:click={() => updateChart(mainChart)}>Get Data</button>
<div id="chart_container" tabindex="0"></div>
{#if showMenu}
    <div class="context-menu" style="top: {menuStyle.top}; left: {menuStyle.left};">
        <button class="context-menu-item" on:click={closeRightClickMenu}>Option 1</button>
        <button class="context-menu-item" on:click={closeRightClickMenu}>Option 2</button>
        <button class="context-menu-item" on:click={closeRightClickMenu}>Option 3</button>
    </div>
{/if}