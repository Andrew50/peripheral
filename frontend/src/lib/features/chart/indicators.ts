import {get} from 'svelte/store'
import {settings} from '$lib/core/stores'
import type {IChartApi, ISeriesApi, CandlestickData, Time, WhitespaceData, CandlestickSeriesOptions, DeepPartial, CandlestickStyleOptions, SeriesOptionsCommon, UTCTimestamp,HistogramStyleOptions, HistogramData, HistogramSeriesOptions} from 'lightweight-charts';
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
    for (let j = 0; j < period && j < data.length; j++) {
        sum += (( data[j].high / data[j].low - 1) * 100)
    }
    const average = sum / period
    return average
}


export function calculateRVOL(data: CandlestickData[],volume:VolumeData){ }
