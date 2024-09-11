<!-- chart.svelte-->
<script lang="ts">
    import Legend from './legend.svelte'
    import Shift from './shift.svelte'
    import {privateRequest} from '$lib/core/backend';
    import type {Instance} from '$lib/core/types'
    import {chartQuery, changeChart} from './interface'
    import type {ShiftOverlay, BarData, ChartRequest} from './interface'
    import { queryInstanceInput } from '$lib/utils/input.svelte'
    import { queryInstanceRightClick } from '$lib/utils/rightClick.svelte'
    import { createChart, ColorType} from 'lightweight-charts';
    import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, UTCTimestamp,HistogramStyleOptions, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
    import {calculateSMA} from './indicators'
    import type {Writable} from 'svelte/store';
    import {writable, get} from 'svelte/store';
    import { onMount  } from 'svelte';
    import { UTCtoEST, ESTtoUTC, ESTSecondstoUTC} from '$lib/core/datetime';
    let chartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
    let chartVolumeSeries: ISeriesApi<"Histogram", Time, WhitespaceData<Time> | HistogramData<Time>, HistogramSeriesOptions, DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>>;
    let sma10Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let sma20Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let chart: IChartApi;
    let latestCrosshairPositionTime: number;
    let chartEarliestDataReached = false;
    let chartLatestDataReached = false;  
    let isLoadingChartData = false    
    let lastChartRequestTime = 0; 
    let queuedLoad: Function | null = null
    let shiftDown = false
    const chartRequestThrottleDuration = 200; 
    const hoveredCandleData = writable({ open: 0, high: 0, low: 0, close: 0, volume: 0, })
    const shiftOverlay: Writable<ShiftOverlay> = writable({ x: 0, y: 0, startX: 0, startY: 0, width: 0, height: 0, isActive: false, startPrice: 0, currentPrice: 0, })

    function backendLoadChartData(inst:ChartRequest): void{
        if (isLoadingChartData ||!inst.ticker || !inst.timeframe || !inst.securityId) { return; }
        isLoadingChartData = true;
        lastChartRequestTime = Date.now()
        privateRequest<BarData[]>("getChartData", {securityId:inst.securityId, timeframe:inst.timeframe, timestamp:inst.timestamp, direction:inst.direction, bars:inst.bars, extendedhours:inst.extendedHours})
            .then((barDataList: BarData[]) => {
                if (! (Array.isArray(barDataList) && barDataList.length > 0)){ return}
                let newCandleData = barDataList.map((bar) => ({
                  time: UTCtoEST(bar.time as UTCTimestamp) as UTCTimestamp,open: bar.open, high: bar.high, low: bar.low, close: bar.close, }));
                let newVolumeData = barDataList.map((bar) => ({
                  time: UTCtoEST(bar.time as UTCTimestamp) as UTCTimestamp, value: bar.volume, color: bar.close > bar.open ? '#089981' : '#ef5350', }));
                if (inst.requestType === 'loadAdditionalData' && inst.direction === 'backward') {
                  const earliestCandleTime = chartCandleSeries.data()[0]?.time;
                  if (typeof earliestCandleTime === 'number' && newCandleData[newCandleData.length - 1].time <= earliestCandleTime) {
                    newCandleData = [...newCandleData.slice(0, -1), ...chartCandleSeries.data()] as any;
                    newVolumeData = [...newVolumeData.slice(0, -1), ...chartVolumeSeries.data()] as any;
                  }
                } else if (inst.requestType === 'loadAdditionalData') {
                  const latestCandleTime = chartCandleSeries.data()[chartCandleSeries.data().length - 1]?.time;
                  if (typeof latestCandleTime === 'number' && newCandleData[0].time >= latestCandleTime) {
                    newCandleData = [...chartCandleSeries.data(), ...newCandleData.slice(1)] as any;
                    newVolumeData = [...chartVolumeSeries.data(), ...newVolumeData.slice(1)] as any;
                  }
                }
                // Check if we reach end of avaliable data 
                if (inst.timestamp == 0) {
                    chartLatestDataReached = true;
                }else if (barDataList.length < inst.bars) {
                    if(inst.direction == 'backward') {
                        chartEarliestDataReached = true;
                    } else if (inst.direction == "forward"){
                        chartLatestDataReached = true;
                    }
                }
                queuedLoad = () => {
                    if (inst.direction == "forward") {
                        const visibleRange = chart.timeScale().getVisibleRange()
                        const vrFrom = visibleRange?.from as Time
                        const vrTo = visibleRange?.to as Time
                        chartCandleSeries.setData(newCandleData);
                        chartVolumeSeries.setData(newVolumeData);
                        chart.timeScale().setVisibleRange({from: vrFrom, to: vrTo})
                    }else if (inst.direction == "backward"){
                        chartCandleSeries.setData(newCandleData);
                        chartVolumeSeries.setData(newVolumeData);
                    }
                    queuedLoad = null
                    sma10Series.setData(calculateSMA(newCandleData, 10));
                    sma20Series.setData(calculateSMA(newCandleData, 20));
                    if (inst.requestType == 'loadNewTicker') {
                        chart.timeScale().fitContent();
                    }
                    isLoadingChartData = false; // Ensure this runs after data is loaded
                }
                if (inst.direction == "backward" || inst.requestType == "loadNewTicker"){
                    queuedLoad()
                }
            })
            .catch((error: string) => {
                console.error(error)
                isLoadingChartData = false; // Ensure this runs after data is loaded
            });
    }
    onMount(() => {
        const chartOptions = { layout: { textColor: 'black', background: { type: ColorType.Solid, color: 'white' } }, timeScale:  { timeVisible: true }, };
        const chartContainer = document.getElementById('chart_container');
        if (!chartContainer) {return;}
        //init event listeners
        chartContainer.addEventListener('contextmenu', (event:MouseEvent) => {
            event.preventDefault();
            const timestamp = ESTSecondstoUTC(latestCrosshairPositionTime);
            const dt = new Date(timestamp);
            const datePart = dt.toLocaleDateString('en-CA'); // 'en-CA' gives you the yyyy-mm-dd format
            const timePart = dt.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
            const formattedDate = `${datePart} ${timePart}`;
            const ins: Instance = { ...get(chartQuery), datetime: formattedDate, timestamp: timestamp}
            queryInstanceRightClick(event,ins,"chart")
        })
        chartContainer.addEventListener('keyup', event => {
            if (event.key == "Shift"){
                shiftDown = false
            }
        })
        function shiftOverlayTrack(event:MouseEvent):void{
            shiftOverlay.update((v:ShiftOverlay) => {
                const god = {
                    ...v,
                    width: Math.abs(event.clientX - v.startX),
                    height: Math.abs(event.clientY - v.startY),
                    x: Math.min(event.clientX, v.startX),
                    y: Math.min(event.clientY, v.startY),
                    currentPrice: chartCandleSeries.coordinateToPrice(event.clientY) || 0,
                }
                console.log(god)
                return god
            })
        }
        chartContainer.addEventListener('mouseup', () => {
            if (queuedLoad != null){
                queuedLoad()
            }
        })
        chartContainer.addEventListener('mousedown',event  => {
            console.log(get(shiftOverlay))
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
                        v.startPrice = chartCandleSeries.coordinateToPrice(v.startY) || 0
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
                if (get(shiftOverlay).isActive){
                    shiftOverlay.update((v:ShiftOverlay) => {
                        if (v.isActive){
                            v.isActive = false
                            return {
                                ...v,
                                isActive: false
                            }
                        }
                     });
                }
            }
        })
        chart = createChart(chartContainer, chartOptions);
        chartCandleSeries = chart.addCandlestickSeries({ upColor: '#089981', downColor: '#ef5350', borderVisible: false, wickUpColor: '#089981', wickDownColor: '#ef5350', });
        chartVolumeSeries = chart.addHistogramSeries({ priceFormat: { type: 'volume', }, priceScaleId: '', });
        chartVolumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0, }, });
        chartCandleSeries.priceScale().applyOptions({ scaleMargins: { top: 0.1, bottom: 0.2, }, });
        sma10Series = chart.addLineSeries({ color: 'blue', lineWidth: 2, });
        sma20Series = chart.addLineSeries({ color: 'orange', lineWidth: 2, });
        chart.subscribeCrosshairMove((param)=>{
            if (!param.point) {
                return;
            }
            const validCrosshairPoint = !(param === undefined || param.time === undefined || param.point.x < 0 || param.point.y < 0);
            if(!validCrosshairPoint) { return; }
            if(!chartCandleSeries) {return;}

            const bar = param.seriesData.get(chartCandleSeries)
            if(!bar) {return;}
            const volumeData = param.seriesData.get(chartVolumeSeries);
            const volume = volumeData ? volumeData.value : 0;
            hoveredCandleData.set({ open: bar.open, high: bar.high, low: bar.low, close: bar.close, volume: volume })
            latestCrosshairPositionTime = bar.time as number //god
        }); 
        chart.timeScale().subscribeVisibleLogicalRangeChange(logicalRange => {
            if (!logicalRange || Date.now() - lastChartRequestTime < chartRequestThrottleDuration) {return;}
            let direction: string;
            let timestampToRequest: number;
            let barsToRequest: number;
            if(logicalRange.from < 10) {
                if(chartLatestDataReached) {return;}
                chartQuery.update((v:ChartRequest) => {
                    return {
                        timestamp: ESTSecondstoUTC(chartCandleSeries.data()[0].time as UTCTimestamp) as number,
                        bars: barsToRequest = 50 - Math.floor(logicalRange.from),
                        direction: "backward",
                        requestType: "loadAdditionalData",
                    }
                })
            } else if (logicalRange.to > chartCandleSeries.data().length-10) {
                if(chartEarliestDataReached) {return;}
                chartQuery.update((v:ChartRequest) => {
                    return {
                        ...v,
                        timestamp: ESTSecondstoUTC(chartCandleSeries.data()[chartCandleSeries.data().length-1].time as UTCTimestamp) as UTCTimestamp,
                        bars: barsToRequest = 150 + 2*Math.floor(logicalRange.to) - chartCandleSeries.data().length,
                        direction: "forward",
                        requestType: "loadAdditionalData",
                    }
                })
            }
        })
       chartQuery.subscribe((req:ChartRequest)=>{
            chartEarliestDataReached = false;
            chartLatestDataReached = false; 
            if (req.timeframe?.includes('m') || req.timeframe?.includes('w') || 
                    req.timeframe?.includes('d') || req.timeframe?.includes('q')){
                    chart.applyOptions({timeScale: {timeVisible: false}});
            }else { chart.applyOptions({timeScale: {timeVisible: true}}); }
            backendLoadChartData(req)
        }) 
    });
</script>

<div autofocus id="chart_container" tabindex="0"></div>
<Legend hoveredCandleData={hoveredCandleData} />
<Shift shiftOverlay={shiftOverlay}/>

<style>
    #chart_container {
      width: 85%;
      height: 800px;
    }
</style>
