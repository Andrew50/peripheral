<!-- chart.svelte-->
<script lang="ts">
    import Legend from './legend.svelte'
    import Shift from './shift.svelte'
    import Countdown from './countdown.svelte'
    import {privateRequest} from '$lib/core/backend';
    import type {Instance, TradeData,QuoteData} from '$lib/core/types'
    import {setActiveChart,chartQuery, changeChart,selectedChartId} from './interface'
    import {currentTimestamp,settings} from '$lib/core/stores'
    import type {ShiftOverlay, BarData, ChartRequest} from './interface'
    import { queryInstanceInput } from '$lib/utils/input.svelte'
    import { queryInstanceRightClick } from '$lib/utils/rightClick.svelte'
    import { createChart, ColorType,CrosshairMode} from 'lightweight-charts';
    import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, UTCTimestamp,HistogramStyleOptions, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
    import {calculateRVOL,calculateSMA,calculateSingleADR,calculateVWAP} from './indicators'
    import type {Writable} from 'svelte/store';
    import {writable, get} from 'svelte/store';
    import { onMount  } from 'svelte';
    import { UTCSecondstoESTSeconds, ESTSecondstoUTCSeconds, ESTSecondstoUTCMillis, getReferenceStartTimeForDateMilliseconds, timeframeToSeconds, getRealTimeTime} from '$lib/core/timestamp';
	import { getStream, replayStream } from '$lib/utils/stream';
    import {timeEvent,replayInfo} from '$lib/core/stores'
    import type {TimeEvent} from '$lib/core/stores'

    import {DateTime} from 'luxon';


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
    let lastChartRequestTime = 0; 
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
    let unsubscribe = () => {} 
    let release = () => {}
    let releaseQuote = () => {}
    let unsubscribeQuote = () => {}
    let currentChartInstance: Instance = {ticker:"",timestamp:0,timeframe:""}
    let blockingChartRequest = {}
    let isPanning = false
    //const tradeConditionsToCheck = new Set([2, 5, 7, 10, 12, 13, 15, 16, 20, 21, 22, 29, 33, 37, 52, 53])
    const tradeConditionsToCheck = new Set([2, 5, 7, 10, 13, 15, 16, 20, 21, 22, 29, 33, 37, 52, 53])
    const tradeConditionsToCheckVolume = new Set([15, 16, 38])

    function backendLoadChartData(inst:ChartRequest): void{
        if (inst.requestType === "loadNewTicker"){
            bidLine.setData([])
            askLine.setData([])
        }

        if (isLoadingChartData ||!inst.ticker || !inst.timeframe || !inst.securityId) { return; }
        isLoadingChartData = true;
        lastChartRequestTime = Date.now()
        if((get(replayInfo).status == "active" || get(replayInfo).status == "paused") && inst.timestamp == 0) {
            inst.timestamp = Math.floor(get(currentTimestamp))
        }
        
        privateRequest<BarData[]>("getChartData", {
            securityId:inst.securityId, 
            timeframe:inst.timeframe, 
            timestamp:inst.timestamp, 
            direction:inst.direction, 
            bars:inst.bars,
            extendedhours:inst.extendedHours, 
            isreplay: (get(replayInfo).status == "active" || get(replayInfo).status == "paused") ? true : false,}
            ,true)
            .then((barDataList: BarData[]) => {
                blockingChartRequest = inst
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
                    const lastBar = newCandleData[newCandleData.length - 1]
                    //bidLine.setData([{time:lastBar.time,value:lastBar.close}])
                    if(inst.includeLastBar == false) {
                        // cuts off the last bar 
                        newCandleData = newCandleData.slice(0, newCandleData.length-1)
                        newVolumeData = newVolumeData.slice(0, newVolumeData.length-1)
                    }
                    const [priceStore, r] = getStream<TradeData[]>(inst, 'fast')
                    release = r
                    unsubscribe = priceStore.subscribe((v:TradeData[]) => {
                        updateLatestChartBar(v)
                    })
                    const [quoteStore, rq] = getStream<QuoteData[]>(inst, 'quote')
                    releaseQuote = rq
                    unsubscribeQuote = quoteStore.subscribe((v:QuoteData[]) => {
                        updateLatestQuote(v)
                    })
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
                        console.log("queued load time", Date.now())
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

                        }
                        isLoadingChartData = false; // Ensure this runs after data is loaded
                }
                // Handling the aggregation of the most recent candle 
                if(inst.timestamp == 0) { // IF REAL TIME DATA 
                    var referenceStartTime: number; 
                    var aggregateOpen: number;
                    var aggregateHigh: number;
                    var aggregateLow: number;
                    var aggregateClose: number; 
                    if(inst.timeframe?.includes('m')) {
                        
                        if(Date.now() - ESTSecondstoUTCMillis(newCandleData[newCandleData.length-1].time) > parseInt(inst.timeframe)*2678400000) {
                            const d = new Date(ESTSecondstoUTCMillis(newCandleData[newCandleData.length-1].time));
                            referenceStartTime = new Date(d.getFullYear(), d.getMonth(), 1).getTime();
                            privateRequest<BarData[]>("getChartData", {
                                securityId:inst.securityId, 
                                timeframe:"1d", 
                                timestamp:referenceStartTime, 
                                direction:inst.direction, 
                                bars:31,
                                extendedhours:inst.extendedHours, 
                                isreplay: (get(replayInfo).status == "active" || get(replayInfo).status == "paused") ? true : false,})
                                .then((barDataL: BarData[]) => {       
                                    aggregateOpen = barDataL[0].open;

                                    // Initialize aggregateHigh and aggregateLow with the first bar's values
                                    aggregateHigh = barDataL[0].high;
                                    aggregateLow = barDataL[0].low;

                                    // Iterate over the barDataL array to find the highest high and lowest low
                                    for (let i = 1; i < barDataL.length; i++) {
                                        if (barDataL[i].high > aggregateHigh) {
                                            aggregateHigh = barDataL[i].high;
                                        } 
                                        if (barDataL[i].low < aggregateLow) {
                                            aggregateLow = barDataL[i].low;
                                        }
                                    }

                                    // Optionally, set aggregateClose to the close of the last bar
                                    aggregateClose = barDataL[barDataL.length - 1].close;
                                });
                        } else {
                            referenceStartTime = newCandleData[newCandleData.length-1].time
                            aggregateOpen = newCandleData[newCandleData.length-1].open 
                            aggregateHigh =  newCandleData[newCandleData.length-1].high
                            aggregateLow =  newCandleData[newCandleData.length-1].low
                        }
                    }
                    else if(inst.timeframe?.includes('w')) {
                        if(Date.now() - ESTSecondstoUTCMillis(newCandleData[newCandleData.length-1].time) > parseInt(inst.timeframe)*604800) {
                            //const d = new Date(ESTSecondstoUTCMillis())
                        } else {
                            referenceStartTime = newCandleData[newCandleData.length-1].time 
                            aggregateOpen = newCandleData[newCandleData.length-1].open 
                            aggregateHigh =  newCandleData[newCandleData.length-1].high
                            aggregateLow =  newCandleData[newCandleData.length-1].low

                        }
                    }
                    else if(inst.timeframe?.includes('d')) {
                        console.log("---------------")
                        
                    }
                    else if(inst.timeframe?.includes('s')) {
                        referenceStartTime =  getReferenceStartTimeForDateMilliseconds(newCandleData[newCandleData.length-1].time*1000, inst.extendedHours)
                        const now = getRealTimeTime();
                        const elapsedTime = now - referenceStartTime; 
                        if(elapsedTime <0) {
                            queuedLoad() //if the market hasn't opened yet 12am-3:59:59am
                        }
                        else {
                            const timeframeMs = chartTimeframeInSeconds * 1000;
                            const numFullBars = Math.floor(elapsedTime / timeframeMs); 
                            // const candleStartTimeUTC
                        }
                    }
                    else { // minute data OR HOURLY 
                        referenceStartTime =  getReferenceStartTimeForDateMilliseconds(newCandleData[newCandleData.length-1].time*1000, inst.extendedHours)
                        
                        const now = getRealTimeTime(); 
                        const elapsedTime = now - referenceStartTime; 
                        console.log("elapsed time is:", elapsedTime)
                        if(elapsedTime < 0) {
                            console.log("Trading session has not started yet.")
                            queuedLoad()
                        } 
                        else {
                            const timeframeMs = chartTimeframeInSeconds * 1000; 
                            const numFullBars = Math.floor(elapsedTime / timeframeMs);
                            const candleStartTimeUTC = referenceStartTime + numFullBars*timeframeMs;
                            console.log("Candle Start Time UTC:", candleStartTimeUTC)
                            const lastBar = newCandleData[newCandleData.length - 1];
                            const lastBarTimeMs = ESTSecondstoUTCMillis(lastBar.time); 

                            const lastCompleteMinuteUTC = DateTime.utc().startOf('minute')
                            let minuteBarsEndTimeUTC = lastCompleteMinuteUTC.toMillis();
                            if(lastCompleteMinuteUTC.toMillis() <= candleStartTimeUTC) {
                                minuteBarsEndTimeUTC = candleStartTimeUTC;
                            }
                            const minuteBarsDurationMs = minuteBarsEndTimeUTC - candleStartTimeUTC;
                            console.log("minuteBarsDurationMs", minuteBarsDurationMs)
                            const numMinuteBars = Math.floor(minuteBarsDurationMs / (60*1000))
                            let minuteBarsPromise: Promise<BarData[]> = Promise.resolve([]);
                            if(numMinuteBars > 0) {
                                minuteBarsPromise = privateRequest<BarData[]>("getChartData", {
                                    securityId: inst.securityId,
                                    timeframe: "1",
                                    timestamp: candleStartTimeUTC,
                                    direction: "forward",
                                    bars: numMinuteBars, 
                                    extendedhours: inst.extendedHours, 
                                    isreplay: (get(replayInfo).status === "active" || get(replayInfo).status === "paused"),
                                });
                            }
                            const tickDataStartTimeUTC = minuteBarsEndTimeUTC; 
                            const tickDataDurationMs = now - tickDataStartTimeUTC; 
                            let tickDataPromise: Promise<TradeData[]> = Promise.resolve([]);
                            if(tickDataDurationMs >0) {
                                tickDataPromise = privateRequest<TradeData[]>("getTradeData", {
                                    securityId: inst.securityId,
                                    time: tickDataStartTimeUTC, 
                                    lengthOfTime: tickDataDurationMs, 
                                    extendedHours: inst.extendedHours,
                                });
                            }
                            Promise.all([minuteBarsPromise, tickDataPromise]).then(([minuteBars, tickData]) => {
                                const allPrices: number[] = [];
                                if(minuteBars && minuteBars.length >0) {
                                    console.log(minuteBars)
                                    aggregateOpen = minuteBars[0].open;
                                    aggregateClose = minuteBars[minuteBars.length-1].close;
                                    minuteBars.forEach(bar => {
                                        allPrices.push(bar.high, bar.low)
                                    })
                                }
                                if(tickData && tickData.length > 0) {
                                    const filteredTickData = tickData.filter(tick => tick.size >= 100);

                                    if (filteredTickData.length > 0) {
                                        const tickPrices = filteredTickData.map(tick => tick.price);
                                        allPrices.push(...tickPrices);

                                        if (aggregateOpen === undefined) {
                                            aggregateOpen = tickPrices[0];
                                        }
                                        aggregateClose = tickPrices[tickPrices.length - 1];
                                    }
                                } 
                                if(allPrices.length > 0 && aggregateOpen !== undefined && aggregateClose !== undefined) {
                                    aggregateHigh = Math.max(...allPrices);
                                    aggregateLow = Math.min(...allPrices);
                                    console.log(allPrices)
                                    console.log({
                                        time: UTCSecondstoESTSeconds(candleStartTimeUTC /1000) as UTCTimestamp, 
                                        open: aggregateOpen, 
                                        high: aggregateHigh,
                                        low: aggregateLow,
                                        close: aggregateClose,
                                    })
                                    if(candleStartTimeUTC > lastBarTimeMs) {
                                        newCandleData.push({
                                        time: UTCSecondstoESTSeconds(candleStartTimeUTC /1000) as UTCTimestamp, 
                                        open: aggregateOpen, 
                                        high: aggregateHigh,
                                        low: aggregateLow,
                                        close: aggregateClose,
                                        })
                                        console.log("push time", Date.now())
                                        if(queuedLoad) {
                                            queuedLoad()
                                        }
                                    }
                                    else {
                                        newCandleData[newCandleData.length-1] = {
                                        time: UTCSecondstoESTSeconds(candleStartTimeUTC /1000) as UTCTimestamp, 
                                        open: aggregateOpen, 
                                        high: aggregateHigh,
                                        low: aggregateLow,
                                        close: aggregateClose,
                                        }
                                        if(queuedLoad) {
                                            queuedLoad()
                                        }
                                    }
                                } 
                                else {
                                    console.log("No data returned for aggregation.")
                                    if(queuedLoad) {
                                        queuedLoad()
                                    }
                                }
                            }).catch(error => {
                                console.error("Error fetching data for aggregation:", error)
                            })
                        }
                    }

                }
                    // if (inst.direction == "backward" || inst.requestType == "loadNewTicker"
                    // || inst.requestType == "forward" && !isPanning){
                    //     queuedLoad()
                    // }
                else if ((get(replayInfo).status == "active" || get(replayInfo).status == "paused")) {
                    if(inst.timeframe?.includes('s')) {
                        referenceStartTime =  getReferenceStartTimeForDateMilliseconds(newCandleData[newCandleData.length-1].time*1000, inst.extendedHours)
                        const now = getRealTimeTime();
                        const elapsedTime = now - referenceStartTime; 
                        if(elapsedTime <0) {
                            queuedLoad() //if the market hasn't opened yet 12am-3:59:59am
                        }
                        else {
                            const timeframeMs = chartTimeframeInSeconds * 1000;
                            const numFullBars = Math.floor(elapsedTime / timeframeMs); 
                            // const candleStartTimeUTC
                        }
                    }
                    else { // minute data OR HOURLY 
                        referenceStartTime =  getReferenceStartTimeForDateMilliseconds(newCandleData[newCandleData.length-1].time*1000, inst.extendedHours)
                        
                        const now = get(currentTimestamp); 
                        console.log("Current timestamp:", now)
                        const elapsedTime = now - referenceStartTime; 
                        console.log("elapsed time is:", elapsedTime)
                        if(elapsedTime < 0) {
                            console.log("Trading session has not started yet.")
                            queuedLoad()
                        } 
                        else {
                            const timeframeMs = chartTimeframeInSeconds * 1000; 
                            const numFullBars = Math.floor(elapsedTime / timeframeMs);
                            const candleStartTimeUTC = referenceStartTime + numFullBars*timeframeMs;
                            console.log("Candle Start Time UTC:", candleStartTimeUTC)
                            const lastBar = newCandleData[newCandleData.length - 1];
                            const lastBarTimeMs = ESTSecondstoUTCMillis(lastBar.time); 

                            const lastCompleteMinuteUTC = DateTime.fromMillis(now, {zone: 'utc'}).startOf('minute')
                            let minuteBarsEndTimeUTC = lastCompleteMinuteUTC.toMillis();
                            if(lastCompleteMinuteUTC.toMillis() <= candleStartTimeUTC) {
                                minuteBarsEndTimeUTC = candleStartTimeUTC;
                            }
                            const minuteBarsDurationMs = minuteBarsEndTimeUTC - candleStartTimeUTC;
                            console.log("minuteBarsDurationMs", minuteBarsDurationMs)
                            const numMinuteBars = Math.floor(minuteBarsDurationMs / (60*1000))
                            let minuteBarsPromise: Promise<BarData[]> = Promise.resolve([]);
                            console.log("numMinuteBars:", numMinuteBars)
                            if(numMinuteBars > 0) {
                                minuteBarsPromise = privateRequest<BarData[]>("getChartData", {
                                    securityId: inst.securityId,
                                    timeframe: "1",
                                    timestamp: candleStartTimeUTC,
                                    direction: "forward",
                                    bars: numMinuteBars, 
                                    extendedhours: inst.extendedHours, 
                                    isreplay: (get(replayInfo).status === "active" || get(replayInfo).status === "paused"),
                                });
                            }
                            const tickDataStartTimeUTC = minuteBarsEndTimeUTC; 
                            const tickDataDurationMs = now - tickDataStartTimeUTC; 
                            let tickDataPromise: Promise<TradeData[]> = Promise.resolve([]);
                            if(tickDataDurationMs >0) {
                                tickDataPromise = privateRequest<TradeData[]>("getTradeData", {
                                    securityId: inst.securityId,
                                    time: tickDataStartTimeUTC, 
                                    lengthOfTime: tickDataDurationMs, 
                                    extendedHours: inst.extendedHours,
                                });
                            }
                            Promise.all([minuteBarsPromise, tickDataPromise]).then(([minuteBars, tickData]) => {
                                const allPrices: number[] = [];
                                if(minuteBars && minuteBars.length >0) {
                                    console.log(minuteBars)
                                    aggregateOpen = minuteBars[0].open;
                                    aggregateClose = minuteBars[minuteBars.length-1].close;
                                    minuteBars.forEach(bar => {
                                        allPrices.push(bar.high, bar.low)
                                    })
                                }
                                if(tickData && tickData.length > 0) {
                                    const filteredTickData = tickData.filter(tick => tick.size >= 100);

                                    if (filteredTickData.length > 0) {
                                        const tickPrices = filteredTickData.map(tick => tick.price);
                                        allPrices.push(...tickPrices);

                                        if (aggregateOpen === undefined) {
                                            aggregateOpen = tickPrices[0];
                                        }
                                        aggregateClose = tickPrices[tickPrices.length - 1];
                                    }
                                } 
                                if(allPrices.length > 0 && aggregateOpen !== undefined && aggregateClose !== undefined) {
                                    aggregateHigh = Math.max(...allPrices);
                                    aggregateLow = Math.min(...allPrices);
                                    console.log({
                                        time: UTCSecondstoESTSeconds(candleStartTimeUTC /1000) as UTCTimestamp, 
                                        open: aggregateOpen, 
                                        high: aggregateHigh,
                                        low: aggregateLow,
                                        close: aggregateClose,
                                    })
                                    if(candleStartTimeUTC > lastBarTimeMs) {
                                        newCandleData.push({
                                        time: UTCSecondstoESTSeconds(candleStartTimeUTC /1000) as UTCTimestamp, 
                                        open: aggregateOpen, 
                                        high: aggregateHigh,
                                        low: aggregateLow,
                                        close: aggregateClose,
                                        })
                                        console.log("push time", Date.now())
                                        if(queuedLoad) {
                                            queuedLoad()
                                        }
                                    }
                                    else {
                                        newCandleData[newCandleData.length-1] = {
                                        time: UTCSecondstoESTSeconds(candleStartTimeUTC /1000) as UTCTimestamp, 
                                        open: aggregateOpen, 
                                        high: aggregateHigh,
                                        low: aggregateLow,
                                        close: aggregateClose,
                                        }
                                        if(queuedLoad) {
                                            queuedLoad()
                                        }
                                    }
                                } 
                                else {
                                    console.log("No data returned for aggregation.")
                                    if(queuedLoad) {
                                        queuedLoad()
                                    }
                                }
                            }).catch(error => {
                                console.error("Error fetching data for aggregation:", error)
                            })
                        }
                    }
                }
                else { // REQUEST IS NOT FOR REAL TIME DATA // IT IS FOR BACK/FRONT LOAD HISTORICAL
                    //console.log("testing", chartCandleSeries.data()[chartCandleSeries.data().length-1].time)
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
                        if (/^\d+$/.test(inst.timeframe)) {
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
                        }
                        isLoadingChartData = false; // Ensure this runs after data is loaded
                    }
                    if (inst.direction == "backward" || inst.requestType == "loadNewTicker"
                        || inst.direction == "forward" && !isPanning){
                            queuedLoad()
                            if(inst.requestType === "loadNewTicker" && !chartLatestDataReached 
                                && get(replayInfo).status === "inactive"){
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
                    }
                }) .catch((error: string) => {
                    console.error(error)

                    isLoadingChartData = false; // Ensure this runs after data is loaded
                });
    }
    
    function updateLatestQuote(quotes:QuoteData[]) {
        const data = quotes[quotes.length-1]
        //if(isLoadingChartData) {return}
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
    async function updateLatestChartBar(trades:TradeData[]) {
        const dolvol = get(settings).dolvol
        function updateConsolidation(consolidatedTrade:TradeData,data:TradeData){
            if(data.conditions == null || (data.size >= 100 && !data.conditions.some(condition => tradeConditionsToCheck.has(condition)))) {
                //if(!(mostRecentBar.close == data.price)) {
                console.log(consolidatedTrade)
                consolidatedTrade.timestamp = data.timestamp
                consolidatedTrade.price = data.price
            }
            if(data.conditions == null || !data.conditions.some(condition => tradeConditionsToCheckVolume.has(condition))) {
                consolidatedTrade.size += data.size * (dolvol ? data.price : 1)
            }
        }
        if(isLoadingChartData || !trades[0].price || !trades[0].size || !trades[0].timestamp || !chartCandleSeries 
        || chartCandleSeries.data().length == 0) {return}
        const consolidatedTrade = {timestamp:null,price:0,size:0}
        const consolidatedTrade2 = {timestamp:null,price:0,size:0}
        var mostRecentBar = chartCandleSeries.data()[chartCandleSeries.data().length-1]
        currentBarTimestamp = mostRecentBar.time as number
        console.log(trades)
        trades.forEach((data:TradeData)=>{
            if (UTCSecondstoESTSeconds(data.timestamp/1000) < (currentBarTimestamp) + chartTimeframeInSeconds) {
                updateConsolidation(consolidatedTrade,data)
            }else{
                console.log("should be new")
                updateConsolidation(consolidatedTrade2,data)
            }
        })
        if (consolidatedTrade.timestamp !== null){
            chartCandleSeries.update({
                time: mostRecentBar.time, 
                open: mostRecentBar.open, 
                high: Math.max(mostRecentBar.high, consolidatedTrade.price), 
                low: Math.min(mostRecentBar.low, consolidatedTrade.price),
                close: consolidatedTrade.price 
            })  
            chartVolumeSeries.update({
                time: mostRecentBar.time, 
                value: chartVolumeSeries.data()[chartVolumeSeries.data().length-1].value + consolidatedTrade.size,
                color: mostRecentBar.close > mostRecentBar.open ? '#089981' : '#ef5350'
            }) 
        }
        //new bar
        var timeToRequestForUpdatingAggregate = ESTSecondstoUTCSeconds(mostRecentBar.time as number) * 1000;
        console.log(consolidatedTrade2)
        if (consolidatedTrade2.timestamp !== null){
            console.log("new")
            const data = consolidatedTrade2
            var referenceStartTime = getReferenceStartTimeForDateMilliseconds(data.timestamp, currentChartInstance.extendedHours) // this is in milliseconds 
            var timeDiff = (data.timestamp - referenceStartTime)/1000 // this is in seconds
            var flooredDifference = Math.floor(timeDiff / chartTimeframeInSeconds) * chartTimeframeInSeconds // this is in seconds 
            console.log("Attempted Timestamp", referenceStartTime, flooredDifference, (referenceStartTime/1000 + flooredDifference))
            var newTime = UTCSecondstoESTSeconds((referenceStartTime/1000 + flooredDifference)) as UTCTimestamp
            console.log(chartCandleSeries.data())
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
            console.log(chartCandleSeries.data())
        } else{
            return 
        }
        await new Promise(resolve => setTimeout(resolve, 3000));
        try {
            const barDataList: BarData[] = await privateRequest<BarData[]>("getChartData", {
                securityId:chartSecurityId, 
                timeframe:chartTimeframe, 
                timestamp: timeToRequestForUpdatingAggregate,
                direction:"backward",
                bars:1,
                extendedHours: chartExtendedHours,
                isreplay: (get(replayInfo).status == "active" || get(replayInfo).status == "paused") ? true : false,
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
                if (currentChartInstance.timestamp && get(replayInfo).status == "inactive"){
                    changeChart({timestamp:0})
                }else{
                    chart.timeScale().resetTimeScale()
                }
            }else if (event.key == "Tab" || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                if(get(replayInfo).status == "active" || get(replayInfo).status == "paused") {
                    currentChartInstance.timestamp = 0
                }
                queryInstanceInput("any",currentChartInstance)
                .then((v:Instance)=>{
                    currentChartInstance = v
                    changeChart(v, true)
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
            if (!logicalRange || Date.now() - lastChartRequestTime < chartRequestThrottleDuration) {return;}
            const barsOnScreen = Math.floor(logicalRange.to) - Math.ceil(logicalRange.from)
            const bufferInScreenSizes = .7;
            if(logicalRange.from / barsOnScreen < bufferInScreenSizes) {
                if(chartEarliestDataReached) {return;}
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
                if(replayStream.replayStatus == true) { return;}
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
        function change(newReq:ChartRequest,ignoreId=false){
           if (!ignoreId && chartId !== selectedChartId){return}
           const req = {...currentChartInstance,...newReq}
           if (!req.timeframe){
               req.timeframe = "1d"
           }
          /* if (["active","paused"].includes(get(replayInfo).status)){
               req.timestamp = get(currentTimestamp)
           }*/
            unsubscribe() 
            release()
            unsubscribeQuote()
            releaseQuote()
            hoveredCandleData.set(defaultHoveredCandleData)
            chartEarliestDataReached = false;
            chartLatestDataReached = false; 
            if(!req.securityId || !req.ticker || !req.timeframe) {return}

            chartSecurityId = req.securityId;
            chartTimeframe = req.timeframe;
            currentChartInstance = {...req}
            chartTimeframeInSeconds = timeframeToSeconds(req.timeframe, (req.timestamp == 0 ? Date.now() : req.timestamp) as number);
            chartExtendedHours = req.extendedHours;
            if (req.timeframe?.includes('m') || req.timeframe?.includes('w') || 
                    req.timeframe?.includes('d') || req.timeframe?.includes('q')){
                    chart.applyOptions({timeScale: {timeVisible: false}});
            }else { chart.applyOptions({timeScale: {timeVisible: true}}); }
            backendLoadChartData(req)
        }
       chartQuery.subscribe((req:ChartRequest)=>{
           change(req)
        }) 
       timeEvent.subscribe((e:TimeEvent)=>{
           console.log(e)
           if (!currentChartInstance || !currentChartInstance.securityId) return;
           if (e.event == "replay"){
               currentChartInstance.timestamp = get(currentTimestamp)
                const req: ChartRequest = {
                    ...currentChartInstance,
                    bars: 400,
                    direction: "backward",
                    requestType: "loadNewTicker",
                    includeLastBar: false,
                }
                change(req,true)
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

