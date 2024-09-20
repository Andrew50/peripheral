<!-- chart.svelte-->
<script lang="ts">
    import Legend from './legend.svelte'
    import Shift from './shift.svelte'
    import {privateRequest} from '$lib/core/backend';
    import type {Instance, TradeData,QuoteData} from '$lib/core/types'
    import {setActiveChart,chartQuery, changeChart,selectedChartId} from './interface'
    import type {ShiftOverlay, BarData, ChartRequest} from './interface'
    import { queryInstanceInput } from '$lib/utils/input.svelte'
    import { queryInstanceRightClick } from '$lib/utils/rightClick.svelte'
    import { createChart, ColorType,CrosshairMode} from 'lightweight-charts';
    import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, UTCTimestamp,HistogramStyleOptions, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
    import {calculateSMA,calculateADR,calculateRVOL} from './indicators'
    import type {Writable} from 'svelte/store';
    import {writable, get} from 'svelte/store';
    import { onMount, onDestroy  } from 'svelte';
    import { UTCtoEST, ESTtoUTC, ESTSecondstoUTC, getReferenceStartTimeForDateMilliseconds, timeframeToSeconds} from '$lib/core/timestamp';
	import { getStream, replayStream } from '$lib/utils/stream';
    let bidLine: any
    let askLine: any
    let askPriceLine: any
    let chartCandleSeries: ISeriesApi<"Candlestick", Time, WhitespaceData<Time> | CandlestickData<Time>, CandlestickSeriesOptions, DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>>
    let chartVolumeSeries: ISeriesApi<"Histogram", Time, WhitespaceData<Time> | HistogramData<Time>, HistogramSeriesOptions, DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>>;
    let sma10Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let adr20Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let sma20Series: ISeriesApi<"Line", Time, WhitespaceData<Time> | { time: UTCTimestamp, value: number }, any, any>;
    let chart: IChartApi;
    let latestCrosshairPositionTime: number;
    let chartEarliestDataReached = false;
    let chartLatestDataReached = false;  
    let isLoadingChartData = false    
    let lastChartRequestTime = 0; 
    let queuedLoad: Function | null = null
    let shiftDown = false
    const chartRequestThrottleDuration = 150; 
    const hoveredCandleData = writable({ open: 0, high: 0, low: 0, close: 0, volume: 0, })
    const shiftOverlay: Writable<ShiftOverlay> = writable({ x: 0, y: 0, startX: 0, startY: 0, width: 0, height: 0, isActive: false, startPrice: 0, currentPrice: 0, })
    
    export let chartId: number;
    export let width: number;
    let chartTicker: string;
    let chartSecurityId: number; 
    let chartTimeframe: string; 
    let chartTimeframeInSeconds: number; 
    let chartExtendedHours: boolean;
    let unsubscribe = () => {} 
    let release = () => {}
    let releaseQuote = () => {}
    let unsubscribeQuote = () => {}
    let touchStartX: number;
    let touchStartY: number;
    let chartInstance: Instance;

    const tradeConditionsToCheck = new Set([2, 5, 7, 10, 12, 13, 15, 16, 20, 21, 22, 29, 33, 37, 52, 53])
    function backendLoadChartData(inst:ChartRequest): void{
        console.log(isLoadingChartData, inst)
        if (isLoadingChartData ||!inst.ticker || !inst.timeframe || !inst.securityId) { return; }
        isLoadingChartData = true;
        lastChartRequestTime = Date.now()
        privateRequest<BarData[]>("getChartData", {securityId:inst.securityId, timeframe:inst.timeframe, timestamp:inst.timestamp, direction:inst.direction, bars:inst.bars,extendedhours:inst.extendedHours})
            .then((barDataList: BarData[]) => {
                if (! (Array.isArray(barDataList) && barDataList.length > 0)){ return}
                let newCandleData = barDataList.map((bar) => ({
                  time: UTCtoEST(bar.time as UTCTimestamp) as UTCTimestamp,
                  open: bar.open, 
                  high: bar.high, 
                  low: bar.low, 
                  close: bar.close, 
                }));
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
                } else if(inst.requestType === 'loadNewTicker') {
                    if(inst.includeLastBar == false) {
                        newCandleData = newCandleData.slice(0, newCandleData.length-1)
                        newVolumeData = newVolumeData.slice(0, newVolumeData.length-1)
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
                // Handling the aggregation of the most recent candle 
                if(inst.timestamp == 0) { // IF REAL TIME DATA 

                    var referenceStartTime = getReferenceStartTimeForDateMilliseconds(newCandleData[newCandleData.length-1].time*1000, inst.extendedHours) // this is in milliseconds 
                    var timediff = (Date.now() - referenceStartTime)/1000 // this is in seconds
                    var flooredDifference = Math.floor(timediff/chartTimeframeInSeconds) * chartTimeframeInSeconds // this is in seconds
                    var newTime = referenceStartTime + flooredDifference*1000
                   /* privateRequest<TradeData[]>("getTradeData", {securityId: inst.securityId, time: newTime, lengthOfTime: chartTimeframeInSeconds, extendedHours: inst.extendedHours})
                  .then((res:Array<any>)=> {
                        if (!Array.isArray(res) || res.length === 0) {
                            return 
                        }
                        var aggregateOpen = 0;
                        var aggregateClose = 0;
                        var aggregateHigh = 0;
                        var aggregateLow = 0;
                        for (let i = 0; i < res.length; i++) {
                            if(res[i].size <= 100) {continue;}
                            if(aggregateOpen == 0) {
                                aggregateOpen = res[i].price;
                            }
                            if(res[i].price > aggregateHigh) {
                                aggregateHigh = res[i].price
                            } else if (aggregateLow == 0 || res[i].price < aggregateLow) {
                                aggregateLow = res[i].price
                            }
                            aggregateClose = res[i].price
                        }
                        newCandleData.push({time: UTCtoEST(newTime / 1000) as UTCTimestamp, open: aggregateOpen, high: aggregateHigh, low: aggregateLow, close: aggregateClose});
                        newVolumeData.push({time:UTCtoEST(newTime/1000) as UTCTimestamp, value: res.reduce((acc, trade) => acc + trade.size, 0), color: aggregateClose > aggregateOpen ? '#089981' : '#ef5350',});
                        */
                        queuedLoad = () => {
                        if (inst.direction == "forward") {
                            const visibleRange = chart.timeScale().getVisibleRange()
                            const vrFrom = visibleRange?.from as Time
                            const vrTo = visibleRange?.to as Time
                            chartCandleSeries.setData(newCandleData);
                            chartVolumeSeries.setData(newVolumeData);
                            chart.timeScale().setVisibleRange({from: vrFrom, to: vrTo})
                        }else if (inst.direction == "backward"){
                            console.log(newCandleData)
                            chartCandleSeries.setData(newCandleData);
                            chartVolumeSeries.setData(newVolumeData);
                        }
                        queuedLoad = null
                        sma10Series.setData(calculateSMA(newCandleData, 10));
                        sma20Series.setData(calculateSMA(newCandleData, 20));
                        //adr20Series.setData(calculateADR(newCandleData,20));
                        if (inst.requestType == 'loadNewTicker') {
                            chart.timeScale().fitContent();
                            chart.timeScale().applyOptions({
                            rightOffset: 10
                            });
                        }
                        isLoadingChartData = false; // Ensure this runs after data is loaded
                        }
                        if (inst.direction == "backward" || inst.requestType == "loadNewTicker"){
                            queuedLoad()
                        }
                    //});
                }
                else { // REQUEST IS NOT FOR REAL TIME DATA // IT IS FOR BACK/FRONT LOAD or something else like replay 
                    queuedLoad = () => {
                        if (inst.direction == "forward") {
                            const visibleRange = chart.timeScale().getVisibleRange()
                            const vrFrom = visibleRange?.from as Time
                            const vrTo = visibleRange?.to as Time
                            chartCandleSeries.setData(newCandleData);
                            chartVolumeSeries.setData(newVolumeData);
                            chart.timeScale().setVisibleRange({from: vrFrom, to: vrTo})
                        }else if (inst.direction == "backward"){
                            console.log(newCandleData)
                            chartCandleSeries.setData(newCandleData);
                            chartVolumeSeries.setData(newVolumeData);
                        }
                        queuedLoad = null
                        sma10Series.setData(calculateSMA(newCandleData, 10));
                        sma20Series.setData(calculateSMA(newCandleData, 20));
                        //adr20Series.setData(calculateADR(newCandleData,20));
                        if (inst.requestType == 'loadNewTicker') {
                            chart.timeScale().fitContent();
                            chart.timeScale().applyOptions({
                            rightOffset: 10
                            });
                        }
                        isLoadingChartData = false; // Ensure this runs after data is loaded
                        }
                        if (inst.direction == "backward" || inst.requestType == "loadNewTicker"){
                            queuedLoad()
                        }
                }
            })
            .catch((error: string) => {
                console.error(error)
                isLoadingChartData = false; // Ensure this runs after data is loaded
            });
    }
    
    function updateLatestQuote(data:QuoteData) {
        if (!data.bidPrice || !data.askPrice){return}
        console.log('updating quote')
        
    const candle = chartCandleSeries.data()[chartCandleSeries.data().length - 1]
    if (!candle)return;
    const time = candle.time
    bidLine.setData([
        { time: time, value: data.bidPrice },
    ]);
    askLine.setData([
        { time: time, value: data.askPrice },
    ]);

        /*const tim = Math.floor(Date.now() / 1000) as UTCTimestamp
        if (bidPriceLine) chartCandleSeries.removePriceLine(bidPriceLine);
        if (askPriceLine) chartCandleSeries.removePriceLine(askPriceLine);
        bidPriceLine = chartCandleSeries.createPriceLine({
            price: data.bidPrice,
            color: 'red',
            lineWidth: 2,
            lineStyle: 0,  // Solid line
            axisLabelVisible: true,
            title: 'Bid',
        });

        askPriceLine = chartCandleSeries.createPriceLine({
            price: data.askPrice,
            color: 'green',
            lineWidth: 2,
            lineStyle: 0,  // Solid line
            axisLabelVisible: true,
            title: 'Ask',
        });*/

    }
    function updateLatestChartBar(data:TradeData) {
        if (!data.price || !data.size || !data.timestamp) {return}
        if(chartCandleSeries.data().length == 0 || !chartCandleSeries) {return}
        var mostRecentBar = chartCandleSeries.data()[chartCandleSeries.data().length-1]
        if (UTCtoEST(data.timestamp/1000) < (mostRecentBar.time as number) + chartTimeframeInSeconds) {
            mostRecentBar = chartCandleSeries.data()[chartCandleSeries.data().length-1]
            chartVolumeSeries.update({
                time: mostRecentBar.time, 
                value: chartVolumeSeries.data()[chartVolumeSeries.data().length-1].value + data.size,
                color: mostRecentBar.close > mostRecentBar.open ? '#089981' : '#ef5350'
            })
            if(data.conditions == null || (data.size >= 100 && !data.conditions.some(condition => tradeConditionsToCheck.has(condition)))) {
                chartCandleSeries.update({
                time: mostRecentBar.time, 
                open: mostRecentBar.open, 
                high: Math.max(mostRecentBar.high, data.price), 
                low: Math.min(mostRecentBar.low, data.price),
                close: data.price 
                })  
            }
            return 
        } else  { // if not hourly, daily, weekly, monthly at this point; this updates when a new bar has to be created 
            console.log("Attempted to Update")
            privateRequest<BarData[]>("getChartData", {
                securityId:chartSecurityId, 
                timeframe:chartTimeframe, 
                timestamp:ESTtoUTC(chartCandleSeries.data()[chartCandleSeries.data().length-1].time as number)*1000,
                direction:"backward",
                bars:1,
                extendedHours: chartExtendedHours
            }).then((barDataList : BarData[]) => {
                if (! (Array.isArray(barDataList) && barDataList.length > 0)){ return}
                const bar = barDataList[0];
                console.log(bar)
                chartCandleSeries.update({
                    time: UTCtoEST(bar.time) as UTCTimestamp, 
                    open: bar.open, 
                    high: bar.high,
                    low: bar.low,
                    close: bar.close
                })
                chartVolumeSeries.update({
                    time: UTCtoEST(bar.time) as UTCTimestamp,
                    value: bar.volume,
                    color: bar.close > bar.open ? '#089981' : '#ef5350'
                })
                console.log("Updated with aggregate from polygon")

                if(data.size < 100) {return }

                var referenceStartTime = getReferenceStartTimeForDateMilliseconds(data.timestamp, get(chartQuery).extendedHours) // this is in milliseconds 
                var timeDiff = (data.timestamp - referenceStartTime)/1000 // this is in seconds
                var flooredDifference = Math.floor(timeDiff / chartTimeframeInSeconds) * chartTimeframeInSeconds // this is in seconds 
                var newTime = UTCtoEST((referenceStartTime/1000 + flooredDifference)) as UTCTimestamp

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
                return 
            })

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
            const timestamp = ESTSecondstoUTC(latestCrosshairPositionTime);
            const dt = new Date(timestamp);
            const datePart = dt.toLocaleDateString('en-CA'); // 'en-CA' gives you the yyyy-mm-dd format
            const timePart = dt.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
            const formattedDate = `${datePart} ${timePart}`;
            const ins: Instance = { ...get(chartQuery), timestamp: timestamp}
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
            if (queuedLoad != null){
                queuedLoad()
            }
        })
        chartContainer.addEventListener('mousedown',event  => {
            setActiveChart(chartId)
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
        chartContainer.addEventListener('touchstart',(event) => {
            setActiveChart(chartId)
            const touch = event.touches[0];
            touchStartX = touch.clientX
            touchStartY = touch.clientY
        })
        chartContainer.addEventListener('touchend',(event) => {
            const touch = event.changedTouches[0]
            const distX = Math.abs(touch.clientX - touchStartX)
            const distY = Math.abs(touch.clientY - touchStartY)
            if (distX < 10 && distY << 10){
                queryInstanceInput("any",get(chartQuery))
                .then((v:Instance)=>{
                    changeChart(v, true)
                }).catch()
            }
        })

        chartContainer.addEventListener('keydown', (event) => {
            setActiveChart(chartId)
            if (event.key == "r" && event.altKey){
                chart.timeScale().resetTimeScale()
            }else if (event.key == "Tab" || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                queryInstanceInput("any",get(chartQuery))
                .then((v:Instance)=>{
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
        bidLine = chart.addLineSeries({
            color: 'green',
            lineWidth: 2,
            lastValueVisible: true, // Shows the price on the right
            priceLineVisible: false,
        });
        askLine = chart.addLineSeries({
            color: 'red',
            lineWidth: 2,
            lastValueVisible: true, // Shows the price on the right
            priceLineVisible: false,
        });
        //adr20Series = chart.addLineSeries({ color: 'orange', lineWidth:1,priceLineVisible:false,priceScaleId:'left'});

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
            if(logicalRange.from < 10) {
                //console.log(logicalRange.from, Date.now()-lastChartRequestTime, chartEarliestDataReached)
                if(chartEarliestDataReached) {return;}
                const v = get(chartQuery)
                backendLoadChartData({
                    ...v,
                    timestamp: ESTSecondstoUTC(chartCandleSeries.data()[0].time as UTCTimestamp) as number,
                    bars: 50 - Math.floor(logicalRange.from),
                    direction: "backward",
                    requestType: "loadAdditionalData",
                    includeLastBar: true,
                })
            } else if (logicalRange.to > chartCandleSeries.data().length-10) { // forward load
                if(chartLatestDataReached) {return;}
                if(replayStream.replayStatus == true) { return;}
                const v = get(chartQuery)
                backendLoadChartData({
                    ...v,
                    timestamp: ESTSecondstoUTC(chartCandleSeries.data()[chartCandleSeries.data().length-1].time as UTCTimestamp) as UTCTimestamp,
                    bars:  150 + 2*Math.floor(logicalRange.to) - chartCandleSeries.data().length,
                    direction: "forward",
                    requestType: "loadAdditionalData",
                    includeLastBar: true, 
                })
            }
        })
       chartQuery.subscribe((req:ChartRequest)=>{
           if (chartId !== selectedChartId){return}
            unsubscribe() 
            release()
            unsubscribeQuote()
            releaseQuote()

            chartEarliestDataReached = false;
            chartLatestDataReached = false; 
            if(!req.securityId || !req.ticker || !req.timeframe) {return}

            chartTicker = req.ticker;
            chartSecurityId = req.securityId ;
            chartTimeframe = req.timeframe;
            chartInstance = req
            chartTimeframeInSeconds = timeframeToSeconds(req.timeframe);
            chartExtendedHours = req.extendedHours;
            if (req.timeframe?.includes('m') || req.timeframe?.includes('w') || 
                    req.timeframe?.includes('d') || req.timeframe?.includes('q')){
                    chart.applyOptions({timeScale: {timeVisible: false}});
            }else { chart.applyOptions({timeScale: {timeVisible: true}}); }
            backendLoadChartData(req)

            const [priceStore, r] = getStream<TradeData>(req.ticker, 'fast')
            release = r
            unsubscribe = priceStore.subscribe((v:TradeData) => {
                updateLatestChartBar(v)
            })
            const [quoteStore, rq] = getStream<QuoteData>(req.ticker, 'quote')
            releaseQuote = rq
            unsubscribeQuote = quoteStore.subscribe((v:QuoteData) => {
                updateLatestQuote(v)
            })
            
        }) 
        


    });
</script>

<div autofocus class="chart" id="chart_container-{chartId}" style="width: {width}px" tabindex="-1">
<Legend instance={chartInstance} hoveredCandleData={hoveredCandleData} />
<Shift shiftOverlay={shiftOverlay}/>
</div>
<style>
.chart {
    position:relative;
}
</style>

