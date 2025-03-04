import type { Instance } from '$lib/core/types';
import { writable, get, type Writable } from 'svelte/store';
import { streamInfo } from '$lib/core/stores';
import { privateRequest } from '$lib/core/backend';
import type { UTCTimestamp } from 'lightweight-charts';

export interface ShiftOverlay {
	startX: number;
	startY: number;
	x: number;
	y: number;
	width: number;
	height: number;
	isActive: boolean;
	startPrice: number;
	currentPrice: number;
}

export type ChartId = number;
export type ChartEvent = string | number;

let selectedChartId: ChartId = 0;

export interface ChartQueryDispatch extends Instance {
	bars?: number;
	direction?: 'forward' | 'backward';
	requestType?: 'loadNewTicker' | 'loadAdditionalData';
	includeLastBar?: boolean;
	chartId?: number;
	timestamp?: number;
}

export interface ChartEventDispatch {
	event: ChartEvent;
	chartId: ChartId;
	data: any;
}

export const activeChartInstance = writable<Instance | null>(null);

export function setActiveChart(chartId: ChartId, currentChartInstance: Instance) {
	selectedChartId = chartId;
	// Create a new instance object to ensure reactivity
	const updatedInstance = {
		...currentChartInstance,
		ticker: currentChartInstance.ticker,
		securityId: currentChartInstance.securityId,
		timeframe: currentChartInstance.timeframe,
		extendedHours: currentChartInstance.extendedHours ?? false,
		timestamp: currentChartInstance.timestamp ?? 0
	};
	console.log('interface.ts: Setting active chart with instance:', updatedInstance);
	// Force a new object reference to trigger store updates
	activeChartInstance.set(updatedInstance);
}

export const chartQueryDispatcher = writable<ChartQueryDispatch>({
	ticker: '',
	timeframe: '1d',
	timestamp: 0,
	extendedHours: false,
	bars: 400,
	direction: 'backward',
	requestType: 'loadNewTicker',
	includeLastBar: true,
	chartId: 0
});

export const chartEventDispatcher = writable<ChartEventDispatch>({
	event: '',
	chartId: 0,
	data: null
});

export function eventChart(event: ChartEvent, chartId: ChartId = 0, data: any = null) {
	chartEventDispatcher.set({ event, chartId, data });
}

export function addHorizontalLine(price: number) {
	chartEventDispatcher.set({ event: 'addHorizontalLine', chartId: selectedChartId, data: price });
}

export function queryChart(newInstance: Instance, includeLast: boolean = true): void {
	console.log('interface.ts: Query chart called with instance:', newInstance);
	const queryDispatch: ChartQueryDispatch = {
		...newInstance,
		bars: 400,
		direction: 'backward',
		requestType: 'loadNewTicker',
		includeLastBar: includeLast,
		chartId: selectedChartId
	};

	if (get(streamInfo).replayActive) {
		queryDispatch.timestamp = get(streamInfo).timestamp;
	}

	// Ensure we have all necessary instance properties
	if (!newInstance.name && newInstance.securityId) {
		console.log('interface.ts: Fetching details for security ID:', newInstance.securityId);
		console.log('interface.ts: Type of securityId:', typeof newInstance.securityId);
		console.log('interface.ts: Full instance object:', JSON.stringify(newInstance, null, 2));
		privateRequest<Record<string, any>>(
			'getTickerMenuDetails',
			{ securityId: newInstance.securityId },
			true
		)
			.then((details) => {
				console.log('interface.ts: Received details:', details);
				const updatedDispatch: ChartQueryDispatch = {
					...queryDispatch,
					...details
				};
				chartQueryDispatcher.set(updatedDispatch);
				setActiveChart(selectedChartId, updatedDispatch);
			})
			.catch((error) => {
				console.error('interface.ts: Error fetching details:', error);
			});
	} else {
		chartQueryDispatcher.set(queryDispatch);
		setActiveChart(selectedChartId, queryDispatch);
	}
}

export interface BarData {
	time: UTCTimestamp;
	open: number;
	high: number;
	low: number;
	close: number;
	volume: number;
}

export interface SecurityDateBounds {
	minDate: number;
	maxDate: number;
}
