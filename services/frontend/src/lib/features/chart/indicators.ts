import { get } from 'svelte/store';
import { settings } from '$lib/utils/stores/stores';
import { privateRequest } from '$lib/utils/helpers/backend';
import type { BarData } from '$lib/features/chart/interface';
import type {
	CandlestickData,
	UTCTimestamp
} from 'lightweight-charts';
let dailyVolumeSecurityId = -1;
let dailyVolumeDate = -1;
let vol: number;



export function calculateSMA(
	data: CandlestickData[],
	period: number
): { time: UTCTimestamp; value: number }[] {
	if (!Array.isArray(data) || period <= 0 || data.length < period) {
		return [];
	}

	const smaData: { time: UTCTimestamp; value: number }[] = [];
	let sum = 0;

	// Calculate initial sum for first period
	for (let i = 0; i < period; i++) {
		sum += data[i].close;
	}

	// First SMA value
	smaData.push({
		time: data[period - 1].time as UTCTimestamp,
		value: sum / period
	});

	// Use sliding window for remaining values
	for (let i = period; i < data.length; i++) {
		sum = sum - data[i - period].close + data[i].close;
		smaData.push({
			time: data[i].time as UTCTimestamp,
			value: sum / period
		});
	}

	return smaData;
}

export function calculateMultipleSMAs(
	data: CandlestickData[],
	periods: number[]
): Map<number, { time: UTCTimestamp; value: number }[]> {
	const results = new Map();

	// Sort periods in ascending order for optimal calculation
	const sortedPeriods = [...periods].sort((a, b) => a - b);

	for (const period of sortedPeriods) {
		results.set(period, calculateSMA(data, period));
	}

	return results;
}

// For real-time updates - only calculate the latest point
export function updateSMAPoint(
	prevClose: number[], // Array of previous closing prices
	newClose: number, // New closing price
	period: number
): number | null {
	if (prevClose.length < period - 1) {
		return null;
	}

	// Calculate SMA using the last 'period' values
	const values = [...prevClose.slice(-(period - 1)), newClose];
	return values.reduce((sum, val) => sum + val, 0) / period;
}

export function calculateSingleADR(data: CandlestickData[]): number {
	const period = get(settings).adrPeriod;
	let sum = 0;
	let l = 0;
	for (let j = 0; j < period && j < data.length; j++) {
		sum += (data[j].high / data[j].low - 1) * 100;
		l++;
	}
	const average = sum / l;
	return average;
}

async function getVolMA(securityId: number, timestamp: number): Promise<number> {
	const period = 10;
	const data = await privateRequest<BarData[]>('getChartData', {
		securityId: securityId,
		timeframe: '1d', // Assuming daily timeframe
		timestamp: timestamp * 1000 - 1,
		direction: 'backward',
		bars: period, // Fetch extra bars to account for the moving average calculation
		extendedHours: false,
		isReplay: false
	});
	let sum = 0;
	const dolvol = get(settings).dolvol;
	for (let i = 0; i < data.length; i++) {
		sum += data[i].volume * (dolvol ? data[i].close : 1);
	}
	return sum / data.length;
}
function getStartOfDayTimestamp(timestamp: number): number {
	const date = new Date(timestamp * 1000);
	date.setHours(0, 0, 0, 0);
	return Math.floor(date.getTime() / 1000);
}

export async function calculateRVOL(
	volumeData: { time: UTCTimestamp; value: number }[],
	securityId: number
): Promise<number> {
	let volumeSum = 0;
	if (!Array.isArray(volumeData)) {
		return 0;
	}
	const dayDate = getStartOfDayTimestamp(volumeData[volumeData.length - 1].time as number);
	if (dayDate != dailyVolumeDate || securityId !== dailyVolumeSecurityId) {
		vol = await getVolMA(securityId, dayDate);
		dailyVolumeDate = dayDate;
		dailyVolumeSecurityId = securityId;
	}
	for (let i = volumeData.length - 1; i >= 0; i--) {
		const dataPoint = volumeData[i];
		const dataPointDate = getStartOfDayTimestamp(dataPoint.time as number);
		if (dataPointDate !== dayDate) {
			break;
		}
		volumeSum += dataPoint.value;
	}
	return (volumeSum / vol) * 100;
}

export function calculateVWAP(
	data: CandlestickData[],
	volumeData: { time: UTCTimestamp; volume: number }[]
): { time: UTCTimestamp; value: number }[] {
	const vwapData: { time: UTCTimestamp; value: number }[] = [];
	let cumulativeVolume = 0;
	let cumulativePriceVolume = 0;
	let currentDay: string | null = null; // Track the current day
	for (let i = 0; i < data.length; i++) {
		const candle = data[i];
		const volume = volumeData[i]?.volume || 0;
		const candleDate = new Date((candle.time as number) * 1000).toISOString().split('T')[0];
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
			vwapData.push({ time: candle.time as UTCTimestamp, value: vwap });
		}
	}
	return vwapData;
}
