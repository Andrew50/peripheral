<!-- chart.svelte-->
<script lang="ts">
import { createChart, ColorType} from 'lightweight-charts';
import {privateRequest, chartQuery, instanceInputVisible} from '../../store';
import type {ChartQuery} from '../../store'
import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, MouseEventParams, UTCTimestamp} from 'lightweight-charts';
import type {HistogramStyleOptions, HistogramSeriesPartialOptions, IChartApiBase, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
import { onMount, onDestroy } from 'svelte';
	import Page from '../+page.svelte';

let mainChart: IChartApi;
let mainChartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
let mainChartVolumeSeries: ISeriesApi<"Histogram", Time, WhitespaceData<Time> | HistogramData<Time>, HistogramSeriesOptions, DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>>;
//let currentTicker: string;
//let currentTimeframe: string = ""; 
let latestCrosshairPositionTime: Time;
// Right Click context menu variables 
let showMenu = false; 
let menuStyle = {
    top: '0px', 
    left: '0px'
};
let menuCrosshairPositionTime: Time; 

interface barData {
    time: UTCTimestamp;
    open: number; 
    high: number;
    low: number;
    close: number;
    volume: number;
}
function initializeChart()  {
    const chartOptions = { 
        layout: { 
            textColor: 'black', 
            background: { type: ColorType.Solid, color: 'white' } 
        },
        timeScale:  {
            timeVisible: true
        },
    };
    const chartContainer = document.getElementById('chart_container');
    if (!chartContainer) {return;}
    chartContainer.addEventListener('keydown', event => {
        if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
            instanceInputVisible.set(true);
        }
     });
    mainChart = createChart(chartContainer, chartOptions);

    mainChartCandleSeries = mainChart.addCandlestickSeries({
        upColor: '#089981', downColor: '#ef5350', borderVisible: false,
        wickUpColor: '#089981', wickDownColor: '#ef5350',
    });
    const initCandleData = [
        {time: "2024-08-12", open: 1, high: 2, low: 0.5, close: 2},
        {time: "2024-08-13", open: 1, high: 2, low: 0.5, close: 2},
        {time: "2024-08-14", open: 1, high: 2, low: 0.5, close: 2},
        {time: "2024-08-15", open: 1, high: 2, low: 0.5, close: 2},
        {time: "2024-08-16", open: 1, high: 2, low: 0.5, close: 2},
    ]
    mainChartVolumeSeries = mainChart.addHistogramSeries({
        priceFormat: {
            type: 'volume',
        },
        priceScaleId: '',
    });
    mainChartVolumeSeries.priceScale().applyOptions({
        scaleMargins: {
            top: 0.8,
            bottom: 0,
        },
    });
    mainChartCandleSeries.priceScale().applyOptions({
        scaleMargins: {
            top: 0.1,
            bottom: 0.2,
        },
    });
    const initVolumeData = [
        {time: "2024-08-12", value: 1000000.0, color:'#ef5350'},
        {time: "2024-08-13", value: 2000000.0, color:'#089981'},
        {time: "2024-08-14", value: 3000000.0, color:'red'},
        {time: "2024-08-15", value: 4000000.0, color:'green'},
        {time: "2024-08-16", value: 5000000.0, color:'red'},

    ]
    mainChartCandleSeries.setData(initCandleData);
    mainChartVolumeSeries.setData(initVolumeData)
    mainChart.subscribeCrosshairMove(crosshairMoveEvent);
    mainChart.timeScale().fitContent();


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
function loadNewChart(v: ChartQuery) {
    let barDataList: barData[] = []
        privateRequest<barData[]>("getChartData", {security:v.securityId, timeframe:v.timeframe, datetime:v.datetime})
        .then((result: barData[]) => {
            barDataList = result;

            let newCandleData = [];
            let newVolumeData = [];
            for (let i =0; i < barDataList.length; i++) {
                newCandleData.push({
                    time: barDataList[i].time as UTCTimestamp, 
                    open: barDataList[i].open, 
                    high: barDataList[i].high, 
                    low: barDataList[i].low,
                    close: barDataList[i].close, 
                });
                const candleColor = barDataList[i].close > barDataList[i].open 
                newVolumeData.push({
                    time: barDataList[i].time, 
                    value: barDataList[i].volume, 
                    color: candleColor ? '#089981' : '#ef5350',
                })
            }
           
            mainChart.removeSeries(mainChartCandleSeries)
            mainChart.removeSeries(mainChartVolumeSeries)
            mainChartCandleSeries = mainChart.addCandlestickSeries({
                    upColor: '#089981', downColor: '#ef5350', borderVisible: false,
                    wickUpColor: '#089981', wickDownColor: '#ef5350',
                })
            mainChartVolumeSeries = mainChart.addHistogramSeries({
                priceFormat: {
                    type: 'volume',
                },
                priceScaleId: '',
            });
            mainChartVolumeSeries.priceScale().applyOptions({
                scaleMargins: {
                    top: 0.8,
                    bottom: 0,
                },
            });
            mainChartCandleSeries.setData(newCandleData)
            mainChartVolumeSeries.setData(newVolumeData)
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
    menuCrosshairPositionTime = latestCrosshairPositionTime;
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
    chartQuery.subscribe((v:ChartQuery) => {
        loadNewChart(v);
        /*privateRequest<barData[]>("getChartData", {security:v.securityId, timeframe:v.timeframe})
        .then((result: barData[]) => {
            if (Array.isArray(result)){
                let barDataList: barData[] = []
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
                mainChart.removeSeries(mainChartCandleSeries)
                mainChartCandleSeries = mainChart.addCandlestickSeries({
                        upColor: '#26a69a', downColor: '#ef5350', borderVisible: false,
                        wickUpColor: '#26a69a', wickDownColor: '#ef5350',
                    })
                mainChartCandleSeries.setData(newData)
                mainChart.timeScale().fitContent();
            }else{
                console.log("39ffdw invalid bar data")
            }
        })
        .catch((error: string) => {
            console.error("Error fetching chart data:", error);
        });*/
    });

    initializeChart(); 
    const chartContainer = document.getElementById('chart_container');
    if (chartContainer) {
        chartContainer.addEventListener('contextmenu', chartRightClick)
    }
//    document.addEventListener('click', closeRightClickMenu)
    return () => {
        if (chartContainer) {
                chartContainer.removeEventListener('contextmenu', chartRightClick);
            }
    };

});
//onDestroy(() => {
 //       document.removeEventListener('click', closeRightClickMenu)
//})
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
<div id="chart_container" tabindex="0"></div>
{#if showMenu}
    <div class="context-menu" style="top: {menuStyle.top}; left: {menuStyle.left};">
        <button class="context-menu-item" on:click={closeRightClickMenu}>Create Instance at {menuCrosshairPositionTime}</button>
        <button class="context-menu-item" on:click={closeRightClickMenu}>Option 2</button>
        <button class="context-menu-item" on:click={closeRightClickMenu}>Option 3</button>
    </div>
{/if}
