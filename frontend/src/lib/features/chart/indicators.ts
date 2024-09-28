import {get} from 'svelte/store'
import {settings} from '$lib/core/stores'
import {privateRequest} from '$lib/core/backend'
import type {BarData} from '$lib/features/chart/interface'
import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, UTCTimestamp,HistogramStyleOptions, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
let dailyVolumeSecurityId = -1
let dailyVolumeDate = -1
let vol:number;
export function calculateSMA(data: CandlestickData[], period: number): { time: UTCTimestamp, value: number }[] {
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

export function calculateSingleADR(data: CandlestickData[]): number {
    const period = get(settings).adrPeriod
    let sum = 0;
    let l = 0
    for (let j = 0; j < period && j < data.length; j++) {
        sum += (( data[j].high / data[j].low - 1) * 100)
        l ++ 
    }
    const average = sum / l
    return average
}


async function getVolMA(securityId:number,timestamp:number):Promise<number>{
    const period = 10
    const data = await privateRequest<BarData[]>("getChartData",{
        securityId: securityId,
        timeframe: '1d', // Assuming daily timeframe
        timestamp: timestamp*1000 - 1,
        direction: "backward",
        bars: period, // Fetch extra bars to account for the moving average calculation
        extendedHours: false,
        isReplay: false,
    });
    let sum = 0
    const dolvol = get(settings).dolvol
    for (let i = 0;i < data.length;i++){
        sum += data[i].volume * (dolvol ? data[i].close : 1)
    }
    return sum / data.length
}
function getStartOfDayTimestamp(timestamp: number): number {
    const date = new Date(timestamp * 1000);
    date.setHours(0, 0, 0, 0);
    return Math.floor(date.getTime() / 1000);
}

export async function calculateRVOL(volumeData: { time: UTCTimestamp; value: number }[],securityId:number): number {
    let volumeSum = 0;
    if(!Array.isArray(volumeData)) {return 0;}
    const dayDate = getStartOfDayTimestamp(volumeData[volumeData.length-1].time)//new Date(volumeData[volumeData.length].time * 1000).toISOString().split('T')[0]; // Get the date part
    if (dayDate != dailyVolumeDate || securityId !== dailyVolumeSecurityId){

        vol = getVolMA(securityId,dayDate)
        dailyVolumeDate = dayDate
        dailyVolumeSecurityId = securityId
    }
    for (let i = volumeData.length - 1; i >= 0; i--) {
        const dataPoint = volumeData[i];
        const dataPointDate = getStartOfDayTimestamp(dataPoint.time)//new Date(dataPoint.time * 1000).toISOString().split('T')[0]; // Get the date part
        if (dataPointDate !== dayDate) {
            break;
        }
        volumeSum += dataPoint.value;
    }
    const volF = await vol
    return (volumeSum  / volF) * 100
}


export function calculateVWAP(data: CandlestickData[], volumeData: { time: UTCTimestamp; volume: number }[]): { time: UTCTimestamp, value: number }[] {
    let vwapData: { time: UTCTimestamp, value: number }[] = [];
    let cumulativeVolume = 0;
    let cumulativePriceVolume = 0;
    let currentDay: string | null = null; // Track the current day
    for (let i = 0; i < data.length; i++) {
        const candle = data[i];
        const volume = volumeData[i]?.value || 0;
        const candleDate = new Date(candle.time * 1000).toISOString().split('T')[0];
        if (candleDate !== currentDay) {
            cumulativeVolume = 0;
            cumulativePriceVolume = 0;
            currentDay = candleDate;
        }
        const typicalPrice = (candle.high + candle.low + candle.close) / 3;
        cumulativeVolume += volume;
        cumulativePriceVolume += typicalPrice * volume;
        if (cumulativeVolume > 0) {
            const vwap = cumulativePriceVolume / cumulativeVolume;
            vwapData.push({ time: candle.time, value: vwap });
        }
    }
    return vwapData;
}

