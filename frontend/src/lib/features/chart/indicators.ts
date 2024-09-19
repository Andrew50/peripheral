
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

export function calculateADR(data: CandlestickData[],period: number): {time: UTCTimestamp, value: number}[] {
    let adrData: {time:UTCTimestamp, value: number}[] = [];
    for (let i = 0; i < data.length; i ++) {
        if (i >= period - 1){
            let sum = 0;
            for (let j = 0; j < period; j++) {
                sum += (( data[i-j].high / data[i-j].low - 1) * 100)
            }
            const average = sum / period
            adrData.push({time:data[i].time,value:average});
        }
    }
    return adrData
}


export function calculateRVOL(data: CandlestickData[],volume:VolumeData){



}
