<!-- chart.svelte-->
<script lang="ts" context="module">
    import { createChart, ColorType} from 'lightweight-charts';
    import {privateRequest} from '$lib/api/backend';
    import type {Instance} from '$lib/api/backend'
    import { queryInstanceInput } from '$lib/utils/input.svelte'
    import { queryInstanceRightClick } from '$lib/utils/rightClick.svelte'
    import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, MouseEventParams, UTCTimestamp} from 'lightweight-charts';
    import type {HistogramStyleOptions, HistogramSeriesPartialOptions, IChartApiBase, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
    import type {Writable} from 'svelte/store';
    import {writable, get} from 'svelte/store';
    import { onMount  } from 'svelte';
    let latestCrosshairPositionTime: Time;
    interface barData {
        time: UTCTimestamp;
        open: number; 
        high: number;
        low: number;
        close: number;
        volume: number;
    }
    export let chartQuery: Writable<Instance> = writable({datetime:"", extendedHours:false, timeframe:"1d",ticker:""})
    export function changeChart(newInstance: Instance):void{
        chartQuery.update((oldInstance:Instance)=>{
            return { ...oldInstance, ...newInstance }
        })
    }

</script>
<script lang="ts">
    let mainChart: IChartApi;
    let mainChartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
    let mainChartVolumeSeries: ISeriesApi<"Histogram", Time, WhitespaceData<Time> | HistogramData<Time>, HistogramSeriesOptions, DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>>;


    function initializeChart()  {
        const chartOptions = { layout: { textColor: 'black', background: { type: ColorType.Solid, color: 'white' } }, timeScale:  { timeVisible: true }, };
        const chartContainer = document.getElementById('chart_container');
        if (!chartContainer) {return;}
        chartContainer.addEventListener('keydown', event => {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                queryInstanceInput("any",get(chartQuery))
                .then((v:Instance)=>{
                    changeChart(v)
                })
            }
         });
        mainChart = createChart(chartContainer, chartOptions);
        mainChartCandleSeries = mainChart.addCandlestickSeries({ upColor: '#089981', downColor: '#ef5350', borderVisible: false, wickUpColor: '#089981', wickDownColor: '#ef5350', });
        mainChartVolumeSeries = mainChart.addHistogramSeries({ priceFormat: { type: 'volume', }, priceScaleId: '', });
        mainChartVolumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0, }, });
        mainChartCandleSeries.priceScale().applyOptions({ scaleMargins: { top: 0.1, bottom: 0.2, }, });
        mainChart.subscribeCrosshairMove(crosshairMoveEvent); 
        mainChart.timeScale().subscribeVisibleLogicalRangeChange(logicalRange => {
            if(logicalRange) {
                console.log(logicalRange?.from, logicalRange?.to)
                if(logicalRange.from < 10) {
                    const barsToRequest = 50 - logicalRange.from; 
                    privateRequest
                }
            }
        })
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
    function chartRightClick(event: MouseEvent){// {{menuStyle.top}; left: {menuStyle.left
        event.preventDefault();
        const dt = new Date(1000*latestCrosshairPositionTime);
        const datePart = dt.toLocaleDateString('en-CA'); // 'en-CA' gives you the yyyy-mm-dd format
        const timePart = dt.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
        const formattedDate = `${datePart} ${timePart}`;
        const ins: Instance = { ...get(chartQuery), datetime: formattedDate, }
        queryInstanceRightClick(event,ins,"chart")
    }

    onMount(() => {
        chartQuery.subscribe((v:Instance)=>{
            let barDataList: barData[] = []
            if (!v.ticker || !v.timeframe || !v.securityId){return}
            const timeframe = v.timeframe
            if (timeframe && timeframe.length < 1){
                return
            }
            privateRequest<barData[]>("getChartData", {securityId:v.securityId, timeframe:v.timeframe, datetime:v.datetime, direction:"backward", bars:100, extendedhours:false})
            .then((result: barData[]) => {
                if (! (Array.isArray(result) && result.length > 0)){ return}
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
                console.log("Done updating chart!")
            })
            .catch((error: string) => {
                console.error("Error fetching chart data:", error);
            });
        })
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
</script>
<div autofocus id="chart_container" tabindex="0"></div>
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
