<!-- chart.svelte-->
<script lang="ts">
    import Legend from './legend.svelte'
    import Shift from './shift.svelte'
    import Countdown from './countdown.svelte'
    import {privateRequest} from '$lib/core/backend';
    import type {Instance, TradeData,QuoteData} from '$lib/core/types'
    import {setActiveChart,chartQueryDispatcher, chartEventDispatcher,queryChart} from './interface'
    import {streamInfo,settings} from '$lib/core/stores'
    import type {ShiftOverlay,ChartEventDispatch, BarData, ChartQueryDispatch} from './interface'
    import { queryInstanceInput } from '$lib/utils/popups/input.svelte'
    import { queryInstanceRightClick } from '$lib/utils/popups/rightClick.svelte'
    import { createChart, ColorType,CrosshairMode} from 'lightweight-charts';
    import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, UTCTimestamp,HistogramStyleOptions, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
    import {calculateRVOL,calculateSMA,calculateSingleADR,calculateVWAP} from './indicators'
    import type {Writable} from 'svelte/store';
    import {writable, get} from 'svelte/store';
    import { onMount  } from 'svelte';
    import { UTCSecondstoESTSeconds, ESTSecondstoUTCSeconds, ESTSecondstoUTCMillis, getReferenceStartTimeForDateMilliseconds, timeframeToSeconds, getRealTimeTime} from '$lib/core/timestamp';
	import { addStream } from '$lib/utils/stream/interface';
    let bidLine: any
    let askLine: any
    let currentBarTimestamp: number;
    let chartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
    let chartVolumeSeries: ISeriesApi<"Histogram", Time, WhitespaceData<Time> | HistogramData<Time>, HistogramSeriesOptions, DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>>;
    let sma10Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let sma20Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let vwapSeries: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let chart: IChartApi;
    let latestCrosshairPositionTime: number;
    let chartEarliestDataReached = false;
    let chartLatestDataReached = false;  
    let isLoadingChartData = false    
    let lastChartQueryDispatchTime = 0; 
    let queuedLoad: Function | null = null
    let shiftDown = false
    const chartRequestThrottleDuration = 150; 
    const defaultHoveredCandleData = { rvol:0,open: 0, high: 0, low: 0, close: 0, volume: 0, adr:0, chg: 0, chgprct: 0}
    const hoveredCandleData = writable(defaultHoveredCandleData)
    const shiftOverlay: Writable<ShiftOverlay> = writable({ x: 0, y: 0, startX: 0, startY: 0, width: 0, height: 0, isActive: false, startPrice: 0, currentPrice: 0, })
    export let chartId: number;
    export let width: number;
    let chartSecurityId: number; 
    let chartTimeframe: string; 
    let chartTimeframeInSeconds: number; 
    let chartExtendedHours: boolean;
    let releaseFast: () => void = () => {}
    let releaseQuote: () => void = () => {}
    let currentChartInstance: Instance = {ticker:"",timestamp:0,timeframe:""}
    let blockingChartQueryDispatch = {}
    let isPanning = false

    function backendLoadChartData(inst:ChartQueryDispatch): void{
        if (inst.requestType === "loadNewTicker"){
            bidLine.setData([])
            askLine.setData([])
        }
        if (isLoadingChartData ||!inst.ticker || !inst.timeframe || !inst.securityId) { return; }
        isLoadingChartData = true;
        lastChartQueryDispatchTime = Date.now()
        if($streamInfo.replayActive &&( inst.timestamp == 0 || (inst.timestamp ?? 0) > $streamInfo.timestamp)) {
            console.log("adjusting to stream timestamp")
            inst.timestamp = Math.floor($streamInfo.timestamp)
        }
        privateRequest<BarData[]>("getChartData", {
            securityId:inst.securityId, 
            timeframe:inst.timeframe, 
            timestamp:inst.timestamp, 
            direction:inst.direction, 
            bars:inst.bars,
            extendedhours:inst.extendedHours, 
            isreplay: $streamInfo.replayActive},true)
            .then((barDataList: BarData[]) => {
                console.log(barDataList)
                blockingChartQueryDispatch = inst
                if (! (Array.isArray(barDataList) && barDataList.length > 0)){ return}
                let newCandleData = barDataList.map((bar) => ({
                  time: UTCSecondstoESTSeconds(bar.time as UTCTimestamp) as UTCTimestamp,
                  open: bar.open, 
                  high: bar.high, 
                  low: bar.low, 
                  close: bar.close, 
                }));
                let newVolumeData: any
                if (get(settings).dolvol){
                    newVolumeData = barDataList.map((bar) => ({
                      time: UTCSecondstoESTSeconds(bar.time as UTCTimestamp) as UTCTimestamp, value: bar.volume * (bar.close + bar.open) / 2, color: bar.close > bar.open ? '#089981' : '#ef5350', }));
                }else{
                    newVolumeData = barDataList.map((bar) => ({
                      time: UTCSecondstoESTSeconds(bar.time as UTCTimestamp) as UTCTimestamp, value: bar.volume, color: bar.close > bar.open ? '#089981' : '#ef5350', }));
                }
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
                } else if(inst.requestType === 'loadNewTicker') {
                    if(inst.includeLastBar == false && !$streamInfo.replayActive) {
                        newCandleData = newCandleData.slice(0, newCandleData.length-1)
                        newVolumeData = newVolumeData.slice(0, newVolumeData.length-1)
                    }
                    releaseFast()
                    releaseQuote()
                }
                // Check if we reach end of avaliable data 
                if (inst.timestamp == 0) {
                    chartLatestDataReached = true;
                }
                if (barDataList.length < inst.bars) {

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
                if (/^\d+$/.test(inst.timeframe ?? "")) {
                    vwapSeries.setData(calculateVWAP(newCandleData,newVolumeData));
                }else{
                    vwapSeries.setData([])
                }
                if (inst.requestType == 'loadNewTicker') {
                    chart.timeScale().resetTimeScale()
                    //chart.timeScale().fitContent();
                    if (currentChartInstance.timestamp === 0){
                        chart.timeScale().applyOptions({
                        rightOffset: 10
                        });
                    }else{
                        chart.timeScale().applyOptions({
                        rightOffset: 0
                        });
                    }
                    releaseFast = addStream(inst, 'fast',updateLatestChartBar)
                    releaseQuote = addStream(inst, 'quote',updateLatestQuote)
                }
                isLoadingChartData = false; // Ensure this runs after data is loaded
            }
            if (inst.direction == "backward" || inst.requestType == "loadNewTicker"
                || inst.direction == "forward" && !isPanning){
                    queuedLoad()
                    if(inst.requestType === "loadNewTicker" && !chartLatestDataReached 
                        && !$streamInfo.replayActive){
                        console.log("1")
                        backendLoadChartData({
                            ...currentChartInstance,
                            timestamp: ESTSecondstoUTCMillis(chartCandleSeries.data()[chartCandleSeries.data().length-1].time as UTCTimestamp) as UTCTimestamp,
                            bars:  150, //+ 2*Math.floor(chart.getLogicalRange.to) - chartCandleSeries.data().length,
                            direction: "forward",
                            requestType: "loadAdditionalData",
                            includeLastBar: true, 
                        })
                    }
                }
            }) .catch((error: string) => {
                console.error(error)

                isLoadingChartData = false; // Ensure this runs after data is loaded
            });
    }
    
    function updateLatestQuote(data:QuoteData) {
        if (!data?.bidPrice || !data?.askPrice){return}
        const candle = chartCandleSeries.data()[chartCandleSeries.data().length - 1]
        if (!candle)return;
        const time = candle.time
        bidLine.setData([
            { time: time, value: data.bidPrice },
        ]);
        askLine.setData([
            { time: time, value: data.askPrice },
        ]);
    }
    async function updateLatestChartBar(trade:TradeData) {
        const dolvol = get(settings).dolvol
        /*function updateConsolidation(consolidatedTrade:TradeData,data:TradeData){
            if(data.conditions == null || (data.size >= 100 && !data.conditions.some(condition => tradeConditionsToCheck.has(condition)))) {
                //if(!(mostRecentBar.close == data.price)) {
                //console.log(consolidatedTrade)
                consolidatedTrade.timestamp = data.timestamp
                consolidatedTrade.price = data.price
            }
            if(data.conditions == null || !data.conditions.some(condition => tradeConditionsToCheckVolume.has(condition))) {
                consolidatedTrade.size += data.size * (dolvol ? data.price : 1)
            }
        }*/
        if(isLoadingChartData || !trade.price || !trade.size || !trade.timestamp || !chartCandleSeries 
        || chartCandleSeries.data().length == 0) {return}
        //const consolidatedTrade = {timestamp:null,price:0,size:0}
        //const consolidatedTrade2 = {timestamp:null,price:0,size:0}
        var mostRecentBar = chartCandleSeries.data()[chartCandleSeries.data().length-1]
        currentBarTimestamp = mostRecentBar.time as number
        //console.log(trades)
        /*
        trades.forEach((data:TradeData)=>{
            if (UTCSecondstoESTSeconds(data.timestamp/1000) < (currentBarTimestamp) + chartTimeframeInSeconds) {
                updateConsolidation(consolidatedTrade,data)
            }else{
                console.log("should be new")
                updateConsolidation(consolidatedTrade2,data)
            }
        })*/
        const sameBar = UTCSecondstoESTSeconds(trade.timestamp/1000) < (currentBarTimestamp) + chartTimeframeInSeconds
        //console.log(sameBar)

        if (sameBar) {
        //if (trade.timestamp !== null){
            chartCandleSeries.update({
                time: mostRecentBar.time, 
                open: mostRecentBar.open, 
                high: Math.max(mostRecentBar.high, trade.price), 
                low: Math.min(mostRecentBar.low, trade.price),
                close: trade.price 
            })  
            chartVolumeSeries.update({
                time: mostRecentBar.time, 
                value: chartVolumeSeries.data()[chartVolumeSeries.data().length-1].value + trade.size,
                color: mostRecentBar.close > mostRecentBar.open ? '#089981' : '#ef5350'
            }) 
            return
        }else{
            console.log(trade)
        //new bar
            var timeToRequestForUpdatingAggregate = ESTSecondstoUTCSeconds(mostRecentBar.time as number) * 1000;
        //console.log(consolidatedTrade2)
        //if (trade.timestamp !== null){
            const data = trade
            var referenceStartTime = getReferenceStartTimeForDateMilliseconds(data.timestamp, currentChartInstance.extendedHours) // this is in milliseconds 
            var timeDiff = (data.timestamp - referenceStartTime)/1000 // this is in seconds
            var flooredDifference = Math.floor(timeDiff / chartTimeframeInSeconds) * chartTimeframeInSeconds // this is in seconds 
            var newTime = UTCSecondstoESTSeconds((referenceStartTime/1000 + flooredDifference)) as UTCTimestamp
            chartCandleSeries.update({
                time: newTime,
                open: data.price, 
                high: data.price,
                low: data.price,
                close: data.price
            })
            chartVolumeSeries.update({
                time: newTime, 
                value: data.size
            })
        }        await new Promise(resolve => setTimeout(resolve, 3000));
        try {
            const barDataList: BarData[] = await privateRequest<BarData[]>("getChartData", {
                securityId:chartSecurityId, 
                timeframe:chartTimeframe, 
                timestamp: timeToRequestForUpdatingAggregate,
                direction:"backward",
                bars:1,
                extendedHours: chartExtendedHours,
                isreplay: $streamInfo.replayActive
            });

            if (! (Array.isArray(barDataList) && barDataList.length > 0)){ return}
            const bar = barDataList[0];
            var currentCandleData = chartCandleSeries.data()
            for(var c = currentCandleData.length-1; c > 0; c--) {
                if (currentCandleData[c].time == UTCSecondstoESTSeconds(bar.time)) {
                    currentCandleData[c] = {
                        time: UTCSecondstoESTSeconds(bar.time) as UTCTimestamp,
                        open: bar.open,
                        high: bar.high, 
                        low: bar.low,
                        close: bar.close
                    }
                    chartCandleSeries.setData(currentCandleData)
                    var currentVolumeData = chartVolumeSeries.data()
                    currentVolumeData[c] = {
                        time: UTCSecondstoESTSeconds(bar.time) as UTCTimestamp, 
                        value: bar.volume * (dolvol ? bar.close : 1),
                        color: bar.close > bar.open ? '#089981' : '#ef5350'
                    }
                    chartVolumeSeries.setData(currentVolumeData)
                    break
                }
            } 
        }
        catch (error) {
            console.error("Error fetching polygon aggregate 4k6lg", error)
        }
    }
    onMount(() => {
        const chartOptions = { 
            autoSize: true,
            crosshair: {
                mode: CrosshairMode.Normal,
            },
            layout: {
                textColor: 'white',
                background: {
                    type: ColorType.Solid,
                    color: 'black'
                } 
            }, 
            grid: {
                vertLines: {
                    visible: false,
                },
                horzLines: {
                    visible: false
                }
            },
            timeScale:  { 
                timeVisible: true },
            };
        const chartContainer = document.getElementById(`chart_container-${chartId}`);
        if (!chartContainer) {return;}
        //init event listeners
        chartContainer.addEventListener('contextmenu', (event:MouseEvent) => {
            event.preventDefault();
            const timestamp = ESTSecondstoUTCMillis(latestCrosshairPositionTime);
            const ins: Instance = { ...currentChartInstance, timestamp: timestamp}
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
                return god
            })
        }
        chartContainer.addEventListener('mouseup', () => {
            isPanning = false
            if (queuedLoad != null){
                queuedLoad()
            }
        })
        chartContainer.addEventListener('mousedown',event  => {
            setActiveChart(chartId)
            isPanning = true
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

        chartContainer.addEventListener('keydown', (event) => {
            setActiveChart(chartId)
            if (event.key == "r" && event.altKey){
                if (currentChartInstance.timestamp && !$streamInfo.replayActive){
                    queryChart({timestamp:0})
                }else{
                    chart.timeScale().resetTimeScale()
                }
            }else if (event.key == "Tab" || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                if($streamInfo.replayActive) {
                    currentChartInstance.timestamp = 0
                }
                queryInstanceInput("any",currentChartInstance)
                .then((v:Instance)=>{
                    currentChartInstance = v
                    queryChart(v, true)
                }).catch()
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
        chartCandleSeries = chart.addCandlestickSeries({ priceLineVisible:false,upColor: '#089981', downColor: '#ef5350', borderVisible: false, wickUpColor: '#089981', wickDownColor: '#ef5350', });
        chartVolumeSeries = chart.addHistogramSeries({ lastValueVisible:true,priceLineVisible:false,priceFormat: { type: 'volume', }, priceScaleId: '', });
        chartVolumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0, }, });
        chartCandleSeries.priceScale().applyOptions({ scaleMargins: { top: 0.1, bottom: 0.2, }, });
        const smaOptions = { lineWidth: 1, priceLineVisible: false, lastValueVisible:false} as DeepPartial<LineWidth>
        sma10Series = chart.addLineSeries({ color: 'purple',...smaOptions});
        sma20Series = chart.addLineSeries({ color: 'blue', ...smaOptions});
        vwapSeries = chart.addLineSeries({color:'white',...smaOptions})
        //rvolSeries = chart.addLineSeries({color:'green',...smaOptions})
        bidLine = chart.addLineSeries({
            color: 'white',
            lineWidth: 2,
            lastValueVisible: true, // Shows the price on the right
            priceLineVisible: false,
        });
        askLine = chart.addLineSeries({
            color: 'white',
            lineWidth: 2,
            lastValueVisible: true, // Shows the price on the right
            priceLineVisible: false,
        });

        chart.subscribeCrosshairMove((param)=>{
            if (!chartCandleSeries.data().length||!param.point||!currentChartInstance.securityId) {
                return;
            }
            const volumeData = param.seriesData.get(chartVolumeSeries);
            const volume = volumeData ? volumeData.value : 0;
            const allCandleData = chartCandleSeries.data()
            const validCrosshairPoint = !(param === undefined || param.time === undefined || param.point.x < 0 || param.point.y < 0);
            let bar;
            let cursorBarIndex
            if(!validCrosshairPoint) {
                if (param.logical < 0){
                    bar = allCandleData[0]
                    cursorBarIndex = 0
                }else{
                    cursorBarIndex = allCandleData.length - 1
                    bar = allCandleData[cursorBarIndex]
                }
            }else{
                bar = param.seriesData.get(chartCandleSeries)
                if(!bar) {return;}
                const cursorTime = bar.time as number;
                cursorBarIndex = allCandleData.findIndex(candle => candle.time === cursorTime);
            }
            let barsForADR;
            if (cursorBarIndex >= 20) {
                barsForADR = allCandleData.slice(cursorBarIndex-19, cursorBarIndex+1);
            } else {
                barsForADR = allCandleData.slice(0, cursorBarIndex + 1);
            }
            let chg = 0;
            let chgprct = 0
            if (cursorBarIndex > 0){
                chg = bar.close - allCandleData[cursorBarIndex - 1].close 
                chgprct = (bar.close/allCandleData[cursorBarIndex -1].close - 1)*100
            }
            hoveredCandleData.set({ open: bar.open, high: bar.high, low: bar.low, close: bar.close, volume: volume, adr:calculateSingleADR(barsForADR), chg:chg, chgprct:chgprct,rvol:0})
            if (/^\d+$/.test(currentChartInstance.timeframe)) {
                let barsForRVOL
                if (cursorBarIndex >= 1000) {
                    barsForADR = allCandleData.slice(cursorBarIndex-1000, cursorBarIndex+1);
                } else {
                    barsForRVOL = chartVolumeSeries.data().slice(0,cursorBarIndex+1);
                }
                calculateRVOL(barsForRVOL,currentChartInstance.securityId)
                .then((r:any)=>{
                    hoveredCandleData.update((v)=>{
                        v.rvol = r
                        return v
                    })
                })
            }
            latestCrosshairPositionTime = bar.time as number 
        }); 
        chart.timeScale().subscribeVisibleLogicalRangeChange(logicalRange => {
            if (!logicalRange || Date.now() - lastChartQueryDispatchTime < chartRequestThrottleDuration) {return;}
            const barsOnScreen = Math.floor(logicalRange.to) - Math.ceil(logicalRange.from)
            const bufferInScreenSizes = .7;
            if(logicalRange.from / barsOnScreen < bufferInScreenSizes) {
                if(chartEarliestDataReached) {return;}
                        console.log("2")
                backendLoadChartData({
                    ...currentChartInstance,
                    timestamp: ESTSecondstoUTCMillis(chartCandleSeries.data()[0].time as UTCTimestamp) as number,
                    bars: Math.floor(bufferInScreenSizes * barsOnScreen) + 100,
                    direction: "backward",
                    requestType: "loadAdditionalData",
                    includeLastBar: true,
                })
            } else if (((chartCandleSeries.data().length - logicalRange.to) / barsOnScreen) < bufferInScreenSizes) { // forward loa
                if(chartLatestDataReached) {return;}
                if($streamInfo.replayActive) { return;}
                        console.log("3")
                backendLoadChartData({
                    ...currentChartInstance,
                    timestamp: ESTSecondstoUTCMillis(chartCandleSeries.data()[chartCandleSeries.data().length-1].time as UTCTimestamp) as UTCTimestamp,
                    bars: Math.floor(bufferInScreenSizes * barsOnScreen) + 100,
                    direction: "forward",
                    requestType: "loadAdditionalData",
                    includeLastBar: true, 
                })
            }
        })
        function change(newReq:ChartQueryDispatch){
           const req = {...currentChartInstance,...newReq}
           if (chartId !== req.chartId){return}
           if (!req.timeframe){ req.timeframe = "1d" }
            if(!req.securityId || !req.ticker || !req.timeframe) {return}
            hoveredCandleData.set(defaultHoveredCandleData)
            chartEarliestDataReached = false;
            chartLatestDataReached = false; 
            chartSecurityId = req.securityId;
            chartTimeframe = req.timeframe;
            currentChartInstance = {...req}
            chartTimeframeInSeconds = timeframeToSeconds(req.timeframe, (req.timestamp == 0 ? Date.now() : req.timestamp) as number);
            chartExtendedHours = req.extendedHours?? false;
            if (req.timeframe?.includes('m') || req.timeframe?.includes('w') || 
                    req.timeframe?.includes('d') || req.timeframe?.includes('q')){
                    chart.applyOptions({timeScale: {timeVisible: false}});
            }else { chart.applyOptions({timeScale: {timeVisible: true}}); }
            backendLoadChartData(req)
        }
       chartQueryDispatcher.subscribe((req:ChartQueryDispatch)=>{
           change(req)
        }) 
       chartEventDispatcher.subscribe((e:ChartEventDispatch)=>{
           console.log(e)
           if (!currentChartInstance || !currentChartInstance.securityId) return;
           if (e.event == "replay"){
               //currentChartInstance.timestamp = $streamInfo.timestamp
               currentChartInstance.timestamp = 0
                const req: ChartQueryDispatch = {
                    ...currentChartInstance,
                    bars: 400,
                    direction: "backward",
                    requestType: "loadNewTicker",
                    includeLastBar: false,
                    chartId: chartId,
                }
                console.log(req)
                change(req)
           }
       })
    });
</script>

<div autofocus class="chart" id="chart_container-{chartId}" style="width: {width}px" tabindex="-1">
<Legend instance={currentChartInstance} hoveredCandleData={hoveredCandleData} />
<Shift shiftOverlay={shiftOverlay}/>
<Countdown instance={currentChartInstance} currentBarTimestamp={currentBarTimestamp}/>
</div>
<style>
.chart {
    position:relative;
}
</style>

