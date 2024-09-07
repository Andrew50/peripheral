<!-- chart.svelte-->
<script lang="ts" context="module">
    import { createChart, ColorType} from 'lightweight-charts';
    import {privateRequest} from '$lib/api/backend';
    import type {Instance, chartRequest} from '$lib/core/types'
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
    interface securityDateBounds {
        minDate: number;
        maxDate: number;
    }
    export let chartQuery: Writable<Instance> = writable({datetime:"", extendedHours:false, timeframe:"1d",ticker:""})
    export function changeChart(newInstance : Instance):void{
        chartQuery.update((oldInstance:Instance)=>{
            return { ...oldInstance, ...newInstance}
        })
    }

</script>
<script lang="ts">
    let mainChart: IChartApi;
    let mainChartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
    let mainChartVolumeSeries: ISeriesApi<"Histogram", Time, WhitespaceData<Time> | HistogramData<Time>, HistogramSeriesOptions, DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>>;
    let mainChartEarliestDataReached = false;
    let mainChartLatestDataReached = false;  
    let sma10Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let sma20Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;

    let isloadingChartData: boolean = false
    let lastChartRequestTime = 0; 
    const chartRequestThrottleDuration = 200; 
    let shiftDown = false
    const hoveredCandleData = writable({
        open: 0,
        high: 0, 
        low: 0,
        close: 0,
        volume: 0,
    })
    interface ShiftOverlay {
        startX: number
        startY: number
        x: number
        y: number
        width: number
        height: number
        isActive: boolean
        startPrice: number
        currentPrice: number
    }
    const shiftOverlay: Writable<ShiftOverlay> = writable({
        startX: 0,
        startY: 0,
        width: 0,
        height: 0,
        isActive: false,
        startPrice: 0,
        currentPrice: 0,
    })

    function calculateSMA(data: CandlestickData[], period: number): { time: UTCTimestamp, value: number }[] {
        let smaData: { time: UTCTimestamp, value: number }[] = [];
        for (let i = 0; i < data.length; i++) {
            if (i >= period - 1) {
                let sum = 0;
                for (let j = 0; j < period; j++) {
                    sum += data[i - j].close;
                }
                const average = sum / period;
                smaData.push({ time: data[i].time, value: average });
            }
        }
        return smaData;
    }

    function initializeEventListeners(chartContainer:HTMLElement) {
        chartContainer.addEventListener('contextmenu', (event:MouseEvent) => {
            event.preventDefault();
            const dt = new Date(1000*latestCrosshairPositionTime);
            const datePart = dt.toLocaleDateString('en-CA'); // 'en-CA' gives you the yyyy-mm-dd format
            const timePart = dt.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
            const formattedDate = `${datePart} ${timePart}`;
            const ins: Instance = { ...get(chartQuery), datetime: formattedDate, }
            queryInstanceRightClick(event,ins,"chart")
        })
        chartContainer.addEventListener('keyup', event => {
            if (event.key == "Shift"){
                shiftDown = false
            }
        })
        function shiftOverlayTrack(event:MouseEvent):void{
            shiftOverlay.update((v:ShiftOverlay) => {

                return {
                    ...v,
                    width: Math.abs(event.clientX - v.startX),
                    height: Math.abs(event.clientY - v.startY),
                    x: Math.min(event.clientX, v.startX),
                    y: Math.min(event.clientY, v.startY),
                    currentPrice: mainChartCandleSeries.coordinateToPrice(event.clientY) || 0,
                }
            })
        }
        chartContainer.addEventListener('mousedown',event  => {
            if (shiftDown || get(shiftOverlay).isActive){
                shiftOverlay.update((v:ShiftOverlay) => {
                    v.isActive = !v.isActive
                    if (v.isActive){
                        v.startX = event.clientX
                        v.startY = event.clientY
                        v.width = 0
                        v.height = 0
                        v.x = v.startX
                        v.y = v.startY
                        v.startPrice = mainChartCandleSeries.coordinateToPrice(v.startY) || 0
                        chartContainer.addEventListener("mousemove",shiftOverlayTrack)
                    }else{
                        chartContainer.removeEventListener("mousemove",shiftOverlayTrack)
                    }
                    return v
                })
            }
        })
        chartContainer.addEventListener('keydown', event => {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                queryInstanceInput("any",get(chartQuery))
                .then((v:Instance)=>{
                    changeChart(v)
                })
            }else if (event.key == "Shift"){
                shiftDown = true
            }else if (event.key == "Escape"){
                shiftOverlay.update((v:ShiftOverlay) => {
                    if (v.isActive){
                        v.isActive = false
                        return v
                    }
                 });
            }
        })

    }

    function initializeChart()  {
        const chartOptions = { layout: { textColor: 'black', background: { type: ColorType.Solid, color: 'white' } }, timeScale:  { timeVisible: true }, };
        const chartContainer = document.getElementById('chart_container');
        if (!chartContainer) {return;}
        initializeEventListeners(chartContainer)
        mainChart = createChart(chartContainer, chartOptions);
        mainChartCandleSeries = mainChart.addCandlestickSeries({ upColor: '#089981', downColor: '#ef5350', borderVisible: false, wickUpColor: '#089981', wickDownColor: '#ef5350', });
        mainChartVolumeSeries = mainChart.addHistogramSeries({ priceFormat: { type: 'volume', }, priceScaleId: '', });
        mainChartVolumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0, }, });
        mainChartCandleSeries.priceScale().applyOptions({ scaleMargins: { top: 0.1, bottom: 0.2, }, });
        sma10Series = mainChart.addLineSeries({
        color: 'blue', // Color of the 10-period moving average
        lineWidth: 2,
    });

    sma20Series = mainChart.addLineSeries({
        color: 'orange', // Color of the 20-period moving average
        lineWidth: 2,
    });
        mainChart.subscribeCrosshairMove(crosshairMoveEvent); 
        mainChart.timeScale().subscribeVisibleLogicalRangeChange(logicalRange => {
            if(logicalRange) {
                if (Date.now() - lastChartRequestTime < chartRequestThrottleDuration) {return;}
                if(logicalRange.from < 10) {
                    if (!mainChartEarliestDataReached) {
                        const candleData = mainChartCandleSeries.data();
                        if (candleData.length === 0) {
                            console.error("No candle data to request additional bars");
                            return;
                        }
                        const barsToRequest = 50 - Math.floor(logicalRange.from); 
                        const req : chartRequest = {
                            ticker: get(chartQuery).ticker, 
                            datetime: mainChartCandleSeries.data()[0].time.toString(),
                            securityId: get(chartQuery).securityId, 
                            timeframe: get(chartQuery).timeframe, 
                            extendedHours: get(chartQuery).extendedHours, 
                            bars: barsToRequest, 
                            direction: "backward",
                            requestType: "loadAdditionalData"
                        }
                       backendLoadChartData(req);
                    } else {
                        console.log("LIMIT REACHED!")
                    }
                    
                } else if (logicalRange.to > mainChartCandleSeries.data().length-10) {
                    if(mainChartLatestDataReached) {return;}
                    const barsToRequest = 50 + Math.floor(logicalRange.to) - mainChartCandleSeries.data().length; 
                    const req : chartRequest = {
                        ticker: get(chartQuery).ticker, 
                        datetime: mainChartCandleSeries.data()[mainChartCandleSeries.data().length-1].time.toString(),
                        securityId: get(chartQuery).securityId, 
                        timeframe: get(chartQuery).timeframe, 
                        extendedHours: get(chartQuery).extendedHours, 
                        bars: barsToRequest, 
                        direction: "forward",
                        requestType: "loadAdditionalData"
                    }
                    backendLoadChartData(req);
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
        const volumeData = param.seriesData.get(mainChartVolumeSeries);
        const volume = volumeData ? volumeData.value : 0;
        hoveredCandleData.set({
            open: bar.open,
            high: bar.high,
            low: bar.low,
            close: bar.close,
            volume: volume
        })
        latestCrosshairPositionTime = bar.time 

    }
    function backendLoadChartData(inst:chartRequest): void{
        if(isloadingChartData) {return;}
        isloadingChartData = true;
        lastChartRequestTime = Date.now()
        if (!inst.ticker || !inst.timeframe || !inst.securityId) {
            isloadingChartData = false;
            return;
        }
        const timeframe = inst.timeframe 
        if (timeframe && timeframe.length < 1) {
            isloadingChartData = false;
            return 
        }
        let barDataList: barData[] = []
        privateRequest<barData[]>("getChartData", {securityId:inst.securityId, timeframe:inst.timeframe, datetime:inst.datetime, direction:inst.direction, bars:inst.bars, extendedhours:inst.extendedHours})
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
                if (inst.requestType == 'loadAdditionalData') {
                    if(inst.direction == 'backward') {
                        const lastCandleTime = mainChartCandleSeries.data()[0].time;
                        if (typeof lastCandleTime === 'number') {
                            if (newCandleData[newCandleData.length-1].time < lastCandleTime) {
                                newCandleData = [...newCandleData, ...mainChartCandleSeries.data()]
                                newVolumeData = [...newVolumeData, ...mainChartVolumeSeries.data()]
                            }
                        }
                        console.log("loaded more data")
                    } else{
                        newCandleData = [...mainChartCandleSeries.data(), ...newCandleData]
                        newVolumeData = [...mainChartVolumeSeries.data(), ...newVolumeData]

                    }
                }
                // Check if we reach end of avaliable data 
                if (inst.datetime == '' ) {
                    mainChartLatestDataReached = true;
                }
                else if (result.length < inst.bars) {
                    if(inst.direction == 'backward') {
                        mainChartEarliestDataReached = true;
                    } else {
                        mainChartLatestDataReached = true;
                    }
                }
                mainChartCandleSeries.setData(newCandleData);
                mainChartVolumeSeries.setData(newVolumeData);
                const sma10Data = calculateSMA(newCandleData, 10);
                const sma20Data = calculateSMA(newCandleData, 20);

                sma10Series.setData(sma10Data);
                sma20Series.setData(sma20Data);
                if (inst.requestType == 'loadNewTicker') {
                    mainChart.timeScale().fitContent();
                }
                console.log("Done updating chart!")
                return;
            })
            .finally(() => {
                isloadingChartData = false; // Ensure this runs after data is loaded
            })
            .catch((error: string) => {
                console.error("Error fetching chart data:", error);
                isloadingChartData = false; // Ensure this runs after data is loaded
            });
            
        
        
    }
    function 
    onMount(() => {
       chartQuery.subscribe((v:Instance)=>{
            const req : chartRequest = {
                ticker: v.ticker,
                datetime: v.datetime,
                securityId: v.securityId,
                timeframe: v.timeframe,
                extendedHours: v.extendedHours,
                bars: 150,
                direction: "backward",
                requestType: "loadNewTicker"

            }
            mainChartEarliestDataReached = false;
            mainChartLatestDataReached = false; 
            backendLoadChartData(req)
        }) 
       initializeChart()

    });
</script>
<div autofocus id="chart_container" tabindex="0"></div>
{#if $shiftOverlay.isActive}
    
    <div class="shiftOverlay" style="left: {$shiftOverlay.x}px; top: {$shiftOverlay.y}px;
    width: {$shiftOverlay.width}px; height: {$shiftOverlay.height}px;">
    <div class="percentageText">
    {Math.round(($shiftOverlay.currentPrice / $shiftOverlay.startPrice - 1) * 10000)/100}%
    </div>
    </div>
{/if}
<div class="legend">
    O: {$hoveredCandleData.open}
    H: {$hoveredCandleData.high}
    L: {$hoveredCandleData.low}
    C: {$hoveredCandleData.close}
    V: {$hoveredCandleData.volume}
</div>


<style>
    #chart_container {
      width: 85%;
      height: 800px; /* Adjust height as needed */
    }
    .shiftOverlay {
        position: absolute;
        border: 2px dashed lightgrey;
        background-color: rgba(211, 211, 211, 0.2); /* Light grey with transparency */
        z-index: 1000; /* Ensure the overlay is on top of everything */
        pointer-events: none; /* Prevent blocking clicks on other elements */
        display: flex;
        justify-content: center;
        align-items: center;
    }
    .legend {
    position: absolute;
    top: 10px;
    left: 10px;
    background-color: rgba(255, 255, 255, 0.7); /* More transparency */
    padding: 5px; /* Smaller padding */
    border-radius: 5px;
    font-size: 12px; /* Smaller font */
    font-family: Arial, sans-serif;
    color: grey; /* Grey text color */
    z-index: 1000;
}




    .percentageText {
        color: var(--highlight-color); /* Use your theme's highlight color */
        font-size: 1.5rem;
        font-weight: bold;
        text-align: center;
    }
    .context-menu {
        position: absolute;
        background-color: white;
        border: 1px solid #ccc;
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
        padding: 10px;
        display: flex;
        flex-direction: column;
        z-index: 9999;
        justify-content: center;
        align-items: center;
    }

    .context-menu-item {
        padding: 5px 10px;
        cursor: pointer;
    }

    .context-menu-item:hover {
        background-color: #f0f0f0;
    }
</style>
