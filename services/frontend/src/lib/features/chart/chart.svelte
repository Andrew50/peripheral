<!-- chart.svelte-->
<script lang="ts">
	import Legend from './legend.svelte';
	import Shift from './shift.svelte';
	import DrawingMenu from './drawingMenu.svelte';
	// import WhyMoving from '$lib/components/whyMoving.svelte';

	import { chartRequest, privateRequest, publicRequest } from '$lib/utils/helpers/backend';
	import { type DrawingMenuProps, addHorizontalLine, drawingMenuProps } from './drawingMenu.svelte';
	import type { Instance as CoreInstance, TradeData, QuoteData } from '$lib/utils/types/types';
	import {
		setActiveChart,
		chartQueryDispatcher,
		chartEventDispatcher,
		queryChart,
		showExtendedHoursToggle
	} from './interface';
	import { streamInfo, settings, activeAlerts, isPublicViewing } from '$lib/utils/stores/stores';
	import type { ShiftOverlay, ChartEventDispatch, BarData, ChartQueryDispatch } from './interface';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { queryInstanceRightClick } from '$lib/components/rightClick.svelte';
	import { createChart, ColorType, CrosshairMode } from 'lightweight-charts';
	import type {
		IChartApi,
		ISeriesApi,
		CandlestickData,
		Time,
		WhitespaceData,
		CandlestickSeriesOptions,
		DeepPartial,
		CandlestickStyleOptions,
		CustomSeriesOptions,
		SeriesOptionsCommon,
		UTCTimestamp,
		HistogramStyleOptions,
		HistogramData,
		HistogramSeriesOptions,
		LineStyleOptions,
		LineWidth
	} from 'lightweight-charts';
	import { calculateSingleADR, calculateVWAP, calculateMultipleSMAs } from './indicators';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';
	import { onMount, onDestroy } from 'svelte';
	import {
		UTCSecondstoESTSeconds,
		ESTSecondstoUTCSeconds,
		ESTSecondstoUTCMillis,
		getReferenceStartTimeForDateMilliseconds,
		timeframeToSeconds
	} from '$lib/utils/helpers/timestamp';
	import { addStream } from '$lib/utils/stream/interface';
	import { ArrowMarkersPaneView, type ArrowMarker } from './arrowMarkers';
	import { EventMarkersPaneView, type EventMarker } from './eventMarkers';
	import { adjustEventsToTradingDays, handleScreenshot, extendedHours } from './chartHelpers';
	import { SessionHighlighting, createDefaultSessionHighlighter } from './sessionShade';
	import {
		type FilingContext,
		addFilingToChatContext,
		openChatAndQuery
	} from '$lib/features/chat/interface';

	interface EventValue {
		type?: string;
		url?: string;
		ratio?: string;
		amount?: string;
		exDate?: string;
		payDate?: string;
	}

	interface Trade {
		time: number;
		type: string;
		price: number;
	}

	interface Instance extends CoreInstance {
		securityId: number;
		chartId?: number;
		bars?: number;
		extendedHours?: boolean;
	}

	interface ExtendedInstance extends Instance {
		extendedHours?: boolean;
		inputString?: string;
	}

	interface ChartInstance extends ExtendedInstance {
		bars?: number;
		direction?: 'forward' | 'backward';
		requestType?: 'loadNewTicker' | 'loadAdditionalData';
		includeLastBar?: boolean;
		ticker: string;
		timestamp: number;
		timeframe: string;
		securityId: number;
		price: number;
		trades?: any[]; // Add optional trades property
	}

	let bidLine: any;
	let askLine: any;
	let currentBarTimestamp: number;

	let chartCandleSeries: ISeriesApi<
		'Candlestick',
		Time,
		WhitespaceData<Time> | CandlestickData<Time>,
		CandlestickSeriesOptions,
		DeepPartial<CandlestickStyleOptions & SeriesOptionsCommon>
	>;
	let chartVolumeSeries: ISeriesApi<
		'Histogram',
		Time,
		WhitespaceData<Time> | HistogramData<Time>,
		HistogramSeriesOptions,
		DeepPartial<HistogramStyleOptions & SeriesOptionsCommon>
	>;
	let sma10Series: ISeriesApi<
		'Line',
		Time,
		WhitespaceData<Time> | { time: UTCTimestamp; value: number },
		any,
		any
	>;
	let sma20Series: ISeriesApi<
		'Line',
		Time,
		WhitespaceData<Time> | { time: UTCTimestamp; value: number },
		any,
		any
	>;
	let vwapSeries: ISeriesApi<
		'Line',
		Time,
		WhitespaceData<Time> | { time: UTCTimestamp; value: number },
		any,
		any
	>;
	let chart: IChartApi;
	let latestCrosshairPositionTime: number;
	let latestCrosshairPositionY = 0;
	let chartEarliestDataReached = false;
	let chartLatestDataReached = false;
	let lastChartQueryDispatchTime = 0;
	let queuedLoad: Function | null = null;
	let shiftDown = false;
	const chartRequestThrottleDuration = 150;
	const defaultBarsOnScreen = 100;
	const defaultHoveredCandleData = {
		rvol: 0,
		open: 0,
		high: 0,
		low: 0,
		close: 0,
		volume: 0,
		adr: 0,
		chg: 0,
		chgprct: 0
	};
	const hoveredCandleData = writable(defaultHoveredCandleData);
	const shiftOverlay: Writable<ShiftOverlay> = writable({
		x: 0,
		y: 0,
		startX: 0,
		startY: 0,
		width: 0,
		height: 0,
		isActive: false,
		startPrice: 0,
		currentPrice: 0
	});
	export let chartId: number;
	export let width: number;
	export let defaultChartData: any = null;
	let chartSecurityId: number;
	let chartTimeframe: string;
	let chartTimeframeInSeconds: number;
	let chartExtendedHours: boolean;
	let releaseFast: () => void = () => {};
	let releaseQuote: () => void = () => {};
	let currentChartInstance: ChartInstance = {
		ticker: '',
		timestamp: 0,
		timeframe: '',
		securityId: 0,
		price: 0,
		extendedHours: false
	};
	let isPanning = false;
	const excludedConditions = new Set([2, 7, 10, 13, 15, 16, 20, 21, 22, 29, 33, 37]);
	let mouseDownStartX = 0;
	let mouseDownStartY = 0;
	let lastFetchedSecurityId: number | null = null;
	const DRAG_THRESHOLD = 3; // pixels of movement before considered a drag

	// Type guards moved to a higher scope
	const isCandlestick = (data: any): data is CandlestickData<Time> =>
		data &&
		typeof data === 'object' &&
		'open' in data &&
		'high' in data &&
		'low' in data &&
		'close' in data;
	const isHistogram = (data: any): data is HistogramData<Time> =>
		data && typeof data === 'object' && 'value' in data;

	// Add new interface for alert lines
	interface AlertLine {
		price: number;
		line: any; // Use any for now since we don't have the full IPriceLine type
		alertId: number;
	}

	// Add new property to track alert lines
	let alertLines: AlertLine[] = [];

	// State for quote line visibility
	let isViewingLiveData = true; // Assume true initially
	let lastQuoteData: QuoteData | null = null;

	// Track previous security ID to avoid unnecessary stream changes
	let previousSecurityId: number | null = null;

	// Why Moving popup state (declare early to avoid TDZ)
	let whyMovingTicker: string = '';
	let whyMovingTrigger: number = 0;

	let arrowSeries: any = null; // Initialize as null
	let eventSeries: ISeriesApi<'Custom', Time, EventMarker>;
	let eventMarkerView: EventMarkersPaneView;
	let selectedEvent: {
		events: EventMarker['events'];
		x: number;
		y: number;
		time: number;
	} | null = null;
	let hoveredEvent: {
		events: EventMarker['events'];
		x: number;
		y: number;
	} | null = null;

	let sessionHighlighting: SessionHighlighting;

	// Function to fetch detailed ticker information including logo
	function fetchTickerDetails(securityId: number) {
		if (lastFetchedSecurityId === securityId) return;

		lastFetchedSecurityId = securityId;
		publicRequest<Record<string, any>>('getTickerMenuDetails', {
			securityId: securityId
		})
			.then((details) => {
				if (lastFetchedSecurityId === securityId) {
					// Update currentChartInstance with the detailed information
					currentChartInstance = {
						...currentChartInstance,
						...details
					};
				}
			})
			.catch((error) => {
				console.error('Chart component: Error fetching ticker details:', error);
			});
	}

	// Add throttling variables for chart updates
	let pendingBarUpdate: any = null;
	let pendingVolumeUpdate: any = null;
	let lastUpdateTime = 0;
	const updateThrottleMs = 100;

	let keyBuffer: string[] = []; // This is for catching key presses from the keyboard before the input system is active
	let isInputActive = false; // Track if input window is active/initializing
	let isSwitchingTickers = false; // Track chart switching overlay state
	let isLoadingAdditionalData = false; // Track if back or forward loading
	let latestLoadToken = 0;
	let activeTickerChangeRequestAbort: AbortController | null = null;

	// Add type definitions at the top
	interface Alert {
		alertType: string;
		alertPrice?: number;
		securityId: string | number;
		alertId: number;
	}

	interface IPriceLine {
		price: number;
		color: string;
		lineWidth: number;
		lineStyle: number;
		axisLabelVisible: boolean;
		title: string;
	}

	interface HorizontalLine {
		id: number;
		price: number;
		line: IPriceLine;
		color: string;
		lineWidth: number;
		amount?: number;
	}

	// Helper function to refresh legend with the latest candle data
	function _refreshLegendWithLatestCandleData() {
		const candleData = chartCandleSeries?.data();
		const volumeData = chartVolumeSeries?.data();

		if (!candleData || !volumeData || !candleData.length) {
			hoveredCandleData.set(defaultHoveredCandleData);
			return;
		}

		const allCandles = candleData.filter(isCandlestick) as CandlestickData<Time>[];
		const allVolumes = volumeData.filter(isHistogram) as HistogramData<Time>[];

		if (!allCandles.length) {
			hoveredCandleData.set(defaultHoveredCandleData);
			return;
		}

		const lastCandle = allCandles[allCandles.length - 1];
		const lastCandleIndex = allCandles.length - 1;

		let volumeForLastCandle = 0;
		const lastVolumeEntry = allVolumes.find((v) => v.time === lastCandle.time);
		if (lastVolumeEntry) {
			volumeForLastCandle = lastVolumeEntry.value;
		} else if (allVolumes.length > 0) {
			// Fallback if no direct time match, use the absolute last volume entry
			volumeForLastCandle = allVolumes[allVolumes.length - 1].value;
		}

		let barsForADR;
		if (lastCandleIndex >= 19) {
			barsForADR = allCandles.slice(lastCandleIndex - 19, lastCandleIndex + 1);
		} else {
			barsForADR = allCandles.slice(0, lastCandleIndex + 1);
		}

		let chg = 0;
		let chgprct = 0;
		if (lastCandleIndex > 0) {
			const prevBar = allCandles[lastCandleIndex - 1];
			chg = lastCandle.close - prevBar.close;
			chgprct = (lastCandle.close / prevBar.close - 1) * 100;
		}

		hoveredCandleData.set({
			open: lastCandle.open,
			high: lastCandle.high,
			low: lastCandle.low,
			close: lastCandle.close,
			volume: volumeForLastCandle,
			adr: calculateSingleADR(barsForADR),
			chg: chg,
			chgprct: chgprct,
			rvol: 0 // RVOL calculation is currently commented out
		});
	}

	// Helper function to clear quote lines
	function clearQuoteLines() {
		if (bidLine && askLine) {
			bidLine.setData([]);
			askLine.setData([]);
		}
	}

	// Helper function to apply the last known quote
	function applyLastQuote() {
		// Assumes isViewingLiveData is already true when called
		if (lastQuoteData && bidLine && askLine) {
			const candle = chartCandleSeries?.data()?.at(-1);
			if (candle) {
				const time = candle.time;
				bidLine.setData([{ time: time, value: lastQuoteData.bidPrice }]);
				askLine.setData([{ time: time, value: lastQuoteData.askPrice }]);
			}
		}
	}

	function processChartDataResponse(
		response: { bars: BarData[]; isEarliestData: boolean },
		inst: ChartQueryDispatch,
		visibleRange: any
	): void {
		const barDataList = response.bars;
		if (!(Array.isArray(barDataList) && barDataList.length > 0)) {
			queuedLoad = null;
			return;
		}
		let newCandleData = barDataList.map((bar) => ({
			time: UTCSecondstoESTSeconds(bar.time as UTCTimestamp) as UTCTimestamp,
			open: bar.open,
			high: bar.high,
			low: bar.low,
			close: bar.close
		}));
		let newVolumeData: any;
		if (get(settings).dolvol) {
			newVolumeData = barDataList.map((bar) => ({
				time: UTCSecondstoESTSeconds(bar.time as UTCTimestamp) as UTCTimestamp,
				value: (bar.volume * (bar.close + bar.open)) / 2,
				color: bar.close > bar.open ? '#089981' : '#ef5350'
			}));
		} else {
			newVolumeData = barDataList.map((bar) => ({
				time: UTCSecondstoESTSeconds(bar.time as UTCTimestamp) as UTCTimestamp,
				value: bar.volume,
				color: bar.close > bar.open ? '#089981' : '#ef5350'
			}));
		}
		if (inst.requestType === 'loadAdditionalData' && inst.direction === 'backward') {
			const earliestCandleTime = chartCandleSeries.data()[0]?.time;
			if (
				typeof earliestCandleTime === 'number' &&
				newCandleData[newCandleData.length - 1].time <= earliestCandleTime
			) {
				newCandleData = [...newCandleData.slice(0, -1), ...chartCandleSeries.data()] as any;
				newVolumeData = [...newVolumeData.slice(0, -1), ...chartVolumeSeries.data()] as any;
			}
		} else if (inst.requestType === 'loadAdditionalData' && inst.direction === 'forward') {
			// Forward loading: append new data to existing data
			const existingData = chartCandleSeries.data();
			const existingVolumeData = chartVolumeSeries.data();

			if (existingData.length > 0 && newCandleData.length > 0) {
				const latestCandleTime = existingData[existingData.length - 1]?.time;

				// Find the first new candle that comes after the latest existing candle
				let startIndex = 0;
				for (let i = 0; i < newCandleData.length; i++) {
					if (typeof latestCandleTime === 'number' && newCandleData[i].time > latestCandleTime) {
						startIndex = i;
						break;
					}
				}
				// Only append truly new data
				if (startIndex < newCandleData.length) {
					newCandleData = [...existingData, ...newCandleData.slice(startIndex)] as any;
					newVolumeData = [...existingVolumeData, ...newVolumeData.slice(startIndex)] as any;
				} else {
					newCandleData = existingData as any;
					newVolumeData = existingVolumeData as any;
				}
			}
		} else if (inst.requestType === 'loadNewTicker') {
			if (inst.includeLastBar == false && !$streamInfo.replayActive) {
				newCandleData = newCandleData.slice(0, newCandleData.length - 1);
				newVolumeData = newVolumeData.slice(0, newVolumeData.length - 1);
			}

			// Only release streams if securityId is changing
			const currentSecurityId =
				typeof inst.securityId === 'string' ? parseInt(inst.securityId, 10) : inst.securityId;

			if (previousSecurityId !== currentSecurityId) {
				releaseFast();
				releaseQuote();
			}
			drawingMenuProps.update((v) => ({
				...v,
				chartCandleSeries: chartCandleSeries,
				securityId: Number(inst.securityId)
			}));
			for (const line of $drawingMenuProps.horizontalLines) {
				chartCandleSeries.removePriceLine(line.line);
			}
			if (!$isPublicViewing) {
				privateRequest<HorizontalLine[]>('getHorizontalLines', {
					securityId: inst.securityId
				}).then((res: HorizontalLine[]) => {
					if (res !== null && res.length > 0) {
						for (const line of res) {
							addHorizontalLine(
								line.price,
								currentChartInstance.securityId,
								line.id,
								line.color || '#FFFFFF',
								(line.lineWidth || 1) as LineWidth
							);
						}
					}
				});
			}
		}
		// Check if we reach end of avaliable data
		if (inst.timestamp == 0) {
			chartLatestDataReached = true;
		}
		if (barDataList.length < (inst.bars ?? 0)) {
			if (inst.direction == 'backward') {
				chartEarliestDataReached = response.isEarliestData;
			} else if (inst.direction == 'forward') {
				chartLatestDataReached = true;
			}
		}
		queuedLoad = () => {
			// Add SEC filings request when loading new ticker
			chartCandleSeries.setData(newCandleData);
			chartVolumeSeries.setData(newVolumeData);
			if (inst.direction == 'backward') {
				if (
					arrowSeries &&
					inst &&
					typeof inst === 'object' &&
					'trades' in inst &&
					Array.isArray(inst.trades)
				) {
					const markersByTime = new Map<
						number,
						{
							entries: Array<{ price: number; isLong: boolean }>;
							exits: Array<{ price: number; isLong: boolean }>;
						}
					>();

					// Process all trades
					inst.trades.forEach((trade: Trade) => {
						const tradeTime = UTCSecondstoESTSeconds(trade.time / 1000);
						const roundedTime =
							Math.floor(tradeTime / chartTimeframeInSeconds) * chartTimeframeInSeconds;

						if (!markersByTime.has(roundedTime)) {
							markersByTime.set(roundedTime, { entries: [], exits: [] });
						}

						// Determine if this is an entry or exit based on trade type
						const isEntry = trade.type === 'Buy' || trade.type === 'Short';
						const isLong = trade.type === 'Buy' || trade.type === 'Sell';

						if (isEntry) {
							markersByTime.get(roundedTime)?.entries.push({
								price: trade.price,
								isLong: isLong
							});
						} else {
							markersByTime.get(roundedTime)?.exits.push({
								price: trade.price,
								isLong: isLong
							});
						}
					});

					// Convert to format for ArrowMarkersPaneView
					const markers = Array.from(markersByTime.entries()).map(([time, data]) => ({
						time: time as UTCTimestamp,
						entries: data.entries,
						exits: data.exits
					}));
					// Sort markers by timestamp (time) in ascending order
					markers.sort((a, b) => a.time - b.time);

					arrowSeries.setData(markers);
				}
			}
			try {
				const barsWithEvents = response.bars; // Use the original response with events
				if (barsWithEvents.length > 0 && barsWithEvents) {
					const allEventsRaw: Array<{ timestamp: number; type: string; value: string }> = [];
					barsWithEvents.forEach((bar) => {
						if (bar.events && bar.events.length > 0) {
							allEventsRaw.push(...bar.events);
						}
					});

					// Check if new raw events exist
					if (allEventsRaw && allEventsRaw.length > 0) {
						const eventsByTime = new Map<
							number,
							Array<{
								type: string;
								title: string;
								url?: string;
								value?: string;
								exDate?: string;
								payoutDate?: string;
							}>
						>();

						// Iterate through bars, using bar.time as the key
						barsWithEvents.forEach((bar) => {
							if (bar.events && bar.events.length > 0) {
								const barTime = bar.time as number; // Already EST seconds

								if (!eventsByTime.has(barTime)) {
									eventsByTime.set(barTime, []);
								}

								// Process each event attached to this bar
								bar.events.forEach((event) => {
									// Parse the JSON string in event.value into an object
									let valueObj: EventValue = {};
									try {
										valueObj = JSON.parse(event.value);
									} catch (e) {
										console.error('Failed to parse event value:', e, event.value);
									}

									// Create proper event object based on type and add to the map using barTime
									if (event.type === 'sec_filing') {
										eventsByTime.get(barTime)?.push({
											type: 'sec_filing',
											title: valueObj.type || 'SEC Filing',
											url: valueObj.url
										});
									} else if (event.type === 'split') {
										eventsByTime.get(barTime)?.push({
											type: 'split',
											title: `Split: ${valueObj.ratio || 'unknown'}`,
											value: valueObj.ratio
										});
									} else if (event.type === 'dividend') {
										const amount = typeof valueObj.amount === 'string' ? valueObj.amount : '0.00';
										eventsByTime.get(barTime)?.push({
											type: 'dividend',
											title: `Dividend: $${amount}`,
											value: amount,
											exDate: valueObj.exDate || 'Unknown',
											payoutDate: valueObj.payDate || 'Unknown'
										});
									}
								});
							}
						});

						// Process the newly fetched events
						let newEventData: EventMarker[] = [];
						eventsByTime.forEach((eventsList, time) => {
							newEventData.push({
								time: time as UTCTimestamp,
								events: eventsList
							});
						});

						// Get existing events ONLY if loading additional data
						const existingEventData =
							inst.requestType === 'loadAdditionalData'
								? (eventSeries.data() as EventMarker[])
								: [];

						// Combine using a Map to handle potential overlaps/updates
						const combinedEventsMap = new Map<number, EventMarker>();
						existingEventData.forEach((event) =>
							combinedEventsMap.set(event.time as number, event)
						);
						newEventData.forEach((event) => combinedEventsMap.set(event.time as number, event)); // New events overwrite existing at the same time

						let finalEventData = Array.from(combinedEventsMap.values());

						// Sort the combined data by time
						finalEventData.sort((a, b) => (a.time as number) - (b.time as number));

						// Adjust events to trading days using the combined candle data
						// Ensure newCandleData exists and is an array before spreading
						const candleDataForAdjustment = Array.isArray(newCandleData) ? [...newCandleData] : [];
						finalEventData = adjustEventsToTradingDays(finalEventData, candleDataForAdjustment);

						// Set the final data
						eventSeries.setData(finalEventData);
					}
				}
			} catch (error) {
				console.warn('Failed to process chart events from bars:', error);
				// Avoid clearing events on error during additional load
				if (inst.requestType !== 'loadAdditionalData') {
					eventSeries.setData([]);
				}
			}
			queuedLoad = null;

			// Fix the SMA data type issues
			const smaResults = calculateMultipleSMAs(newCandleData, [10, 20]);
			const sma10Data = smaResults.get(10);
			const sma20Data = smaResults.get(20);

			if (sma10Data) {
				sma10Series.setData([...sma10Data] as Array<
					WhitespaceData<Time> | { time: UTCTimestamp; value: number }
				>);
			}
			if (sma20Data) {
				sma20Series.setData([...sma20Data] as Array<
					WhitespaceData<Time> | { time: UTCTimestamp; value: number }
				>);
			}

			if (/^\d+$/.test(inst.timeframe ?? '')) {
				vwapSeries.setData(calculateVWAP(newCandleData, newVolumeData));
			} else {
				vwapSeries.setData([]);
			}

			// Only set up new streams if the securityId actually changed
			const currentSecurityId =
				typeof inst.securityId === 'string' ? parseInt(inst.securityId, 10) : inst.securityId;

			if (inst.requestType == 'loadNewTicker' && previousSecurityId !== currentSecurityId) {
				releaseFast();
				releaseQuote();
				releaseFast = addStream(inst, 'all', updateLatestChartBar) as () => void;
				releaseQuote = addStream(inst, 'quote', updateLatestQuote) as () => void;
				previousSecurityId = currentSecurityId;
			}
			// Hide chart switching overlay when loading completes
			if (inst.requestType === 'loadNewTicker') {
				isSwitchingTickers = false;
			}
			// Trigger Why Moving popup only for new ticker loads
			if (inst.requestType === 'loadNewTicker') {
				whyMovingTicker = inst.ticker ?? '';
				whyMovingTrigger = Date.now();
			}

			// Apply time scale reset and right offset for new ticker loads after all data is processed
			if (inst.requestType === 'loadNewTicker' && chart && inst.direction === 'backward') {
				// Use requestAnimationFrame to ensure chart has processed the data
				requestAnimationFrame(() => {
					chart.timeScale().resetTimeScale();
					if (inst.timestamp === 0) {
						chart.timeScale().applyOptions({
							rightOffset: 10 // Live data gets right margin
						});
					} else {
						chart.timeScale().applyOptions({
							rightOffset: 0 // Historical data gets no right margin
						});
					}
				});
			}
		};
		if (
			inst.direction == 'backward' ||
			inst.requestType == 'loadNewTicker' ||
			inst.direction == 'forward'
		) {
			queuedLoad();
		}
	}

	function backendLoadChartData(inst: ChartQueryDispatch): void {
		if (!inst.ticker || !inst.timeframe || !inst.securityId) {
			return;
		}
		if (inst.requestType === 'loadNewTicker') {
			latestLoadToken++;
			activeTickerChangeRequestAbort?.abort();
			activeTickerChangeRequestAbort = new AbortController();
			eventSeries.setData([]); // Clear events only when loading new ticker
			pendingBarUpdate = null;
			pendingVolumeUpdate = null;
			bidLine.setData([]);
			askLine.setData([]);
			arrowSeries.setData([]);
		} else if (inst.requestType === 'loadAdditionalData') {
			isLoadingAdditionalData = true;
		}
		const thisRequestTickerIncrementCount = latestLoadToken;
		const signal = activeTickerChangeRequestAbort?.signal;

		console.log('backendLoadChartData', inst);
		const visibleRange = chart.timeScale().getVisibleRange();
		lastChartQueryDispatchTime = Date.now();
		if (
			$streamInfo.replayActive &&
			(inst.timestamp == 0 || (inst.timestamp ?? 0) > $streamInfo.timestamp)
		) {
			inst.timestamp = Math.floor($streamInfo.timestamp);
		}
		chartRequest<{ bars: BarData[]; isEarliestData: boolean }>(
			'getChartData',
			{
				securityId: inst.securityId,
				timeframe: inst.timeframe,
				timestamp: inst.timestamp,
				direction: inst.direction,
				bars: inst.bars,
				extendedhours: inst.extendedHours,
				isreplay: $streamInfo.replayActive,
				includeSECFilings: get(settings).showFilings
			},
			false,
			false,
			signal
		)
			.then((response) => {
				if (thisRequestTickerIncrementCount !== latestLoadToken) return;

				processChartDataResponse(response, inst, visibleRange);
			})
			.catch((error: Error) => {
				if (error?.name === 'AbortError' || thisRequestTickerIncrementCount !== latestLoadToken)
					return;
				console.error(error);
				isLoadingAdditionalData = false;
				isSwitchingTickers = false;
			})
			.finally(() => {
				if (thisRequestTickerIncrementCount === latestLoadToken) {
					isLoadingAdditionalData = false;
					isSwitchingTickers = false;
				}
			});
	}
	function updateLatestQuote(data: QuoteData) {
		if (!data?.bidPrice || !data?.askPrice) {
			return;
		}
		// Always store the latest quote
		lastQuoteData = data;

		// Only update lines if viewing live data
		if (isViewingLiveData) {
			applyLastQuote();
		}
	}
	// Create a horizontal line at the current crosshair position (Y-coordinate)

	function handleMouseMove(event: MouseEvent) {
		if (!chartCandleSeries || !$drawingMenuProps.isDragging || !$drawingMenuProps.selectedLine)
			return;

		const price = chartCandleSeries.coordinateToPrice(event.clientY);
		if (typeof price !== 'number' || price <= 0) return;

		// Update the line position visually
		$drawingMenuProps.selectedLine.applyOptions({
			price: price
		});

		// Update the stored price in horizontalLines array
		const lineIndex = $drawingMenuProps.horizontalLines.findIndex(
			(line) => line.line === $drawingMenuProps.selectedLine
		);
		if (lineIndex !== -1) {
			$drawingMenuProps.horizontalLines[lineIndex].price = price;
		}
	}

	function handleMouseUp() {
		if (!$drawingMenuProps.isDragging || !$drawingMenuProps.selectedLine) return;

		const lineData = $drawingMenuProps.horizontalLines.find(
			(line) => line.line === $drawingMenuProps.selectedLine
		);

		if (lineData) {
			// Update line position in backend
			privateRequest<void>(
				'updateHorizontalLine',
				{
					id: lineData.id,
					price: lineData.price,
					securityId: chartSecurityId
				},
				true
			);
		}

		drawingMenuProps.update((v) => ({ ...v, isDragging: false }));
		document.removeEventListener('mousemove', handleMouseMove);
		document.removeEventListener('mouseup', handleMouseUp);
	}

	function startDragging(event: MouseEvent) {
		if (!$drawingMenuProps.selectedLine) return;

		event.preventDefault();
		event.stopPropagation();

		drawingMenuProps.update((v) => ({ ...v, isDragging: true }));
		document.addEventListener('mousemove', handleMouseMove);
		document.addEventListener('mouseup', handleMouseUp);
	}

	function determineClickedLine(event: MouseEvent) {
		const mouseY = event.clientY;
		const pixelBuffer = 5;

		const upperPrice = chartCandleSeries.coordinateToPrice(mouseY - pixelBuffer) || 0;
		const lowerPrice = chartCandleSeries.coordinateToPrice(mouseY + pixelBuffer) || 0;

		if (upperPrice == 0 || lowerPrice == 0) return false;

		// Only check regular horizontal lines, not alert lines
		for (const line of $drawingMenuProps.horizontalLines) {
			if (line.price <= upperPrice && line.price >= lowerPrice) {
				drawingMenuProps.update((v: DrawingMenuProps) => ({
					...v,
					chartCandleSeries: chartCandleSeries,
					selectedLine: line.line,
					selectedLinePrice: line.price,
					clientX: event.clientX,
					clientY: event.clientY,
					active: false,
					selectedLineId: line.id
				}));

				event.preventDefault();
				event.stopPropagation();
				return true;
			}
		}

		setTimeout(() => {
			drawingMenuProps.update((v: DrawingMenuProps) => ({
				...v,
				selectedLine: null,
				selectedLineId: -1,
				active: false
			}));
		}, 100);
		return false;
	}

	function handleMouseDown(event: MouseEvent) {
		// Ensure chart container has focus for keyboard events
		const chartContainer = document.getElementById(`chart_container-${chartId}`);
		if (chartContainer) {
			chartContainer.focus();
		}

		if (determineClickedLine(event)) {
			('determineClickedLine');
			mouseDownStartX = event.clientX;
			mouseDownStartY = event.clientY;

			// Add mousemove listener to detect drag
			const handleMouseMoveForDrag = (moveEvent: MouseEvent) => {
				const deltaX = Math.abs(moveEvent.clientX - mouseDownStartX);
				const deltaY = Math.abs(moveEvent.clientY - mouseDownStartY);

				if (deltaX > DRAG_THRESHOLD || deltaY > DRAG_THRESHOLD) {
					// It's a drag - start dragging and remove this temporary listener
					document.removeEventListener('mousemove', handleMouseMoveForDrag);
					document.removeEventListener('mouseup', handleMouseUpForClick);
					startDragging(moveEvent);
				}
			};

			// Add mouseup listener to handle click
			const handleMouseUpForClick = (upEvent: MouseEvent) => {
				const deltaX = Math.abs(upEvent.clientX - mouseDownStartX);
				const deltaY = Math.abs(upEvent.clientY - mouseDownStartY);

				if (deltaX <= DRAG_THRESHOLD && deltaY <= DRAG_THRESHOLD) {
					('click');
					// It's a click - show menu
					drawingMenuProps.update((v) => ({
						...v,
						active: true,
						clientX: upEvent.clientX,
						clientY: upEvent.clientY
					}));
				}

				// Clean up listeners
				document.removeEventListener('mousemove', handleMouseMoveForDrag);
				document.removeEventListener('mouseup', handleMouseUpForClick);
			};

			document.addEventListener('mousemove', handleMouseMoveForDrag);
			document.addEventListener('mouseup', handleMouseUpForClick);
			return;
		}

		setActiveChart(chartId, currentChartInstance);

		if (shiftDown || get(shiftOverlay).isActive) {
			shiftOverlay.update((v: ShiftOverlay): ShiftOverlay => {
				v.isActive = !v.isActive;
				if (v.isActive) {
					// Disable chart interactions while drawing
					chart.applyOptions({
						handleScroll: false,
						handleScale: false,
						kineticScroll: {
							mouse: false,
							touch: false
						}
					});

					// Get chart container position for relative coordinates
					const chartContainer = document.getElementById(`chart_container-${chartId}`);
					const rect = chartContainer?.getBoundingClientRect();
					const offsetX = rect ? rect.left : 0;
					const offsetY = rect ? rect.top : 0;

					v.startX = event.clientX - offsetX;
					v.startY = event.clientY - offsetY;
					v.width = 0;
					v.height = 0;
					v.x = v.startX;
					v.y = v.startY;
					v.startPrice = chartCandleSeries.coordinateToPrice(event.clientY - offsetY) || 0;
					document.addEventListener('mousemove', shiftOverlayTrack);
					document.addEventListener('mouseup', handleShiftOverlayEnd);
				} else {
					// Re-enable chart interactions when drawing ends
					chart.applyOptions({
						handleScroll: true,
						handleScale: true,
						kineticScroll: {
							mouse: false,
							touch: false
						}
					});
					document.removeEventListener('mousemove', shiftOverlayTrack);
				}
				return v;
			});
		} else {
			isPanning = true;
		}
	}

	function shiftOverlayTrack(event: MouseEvent): void {
		shiftOverlay.update((v: ShiftOverlay): ShiftOverlay => {
			// Get chart container position for relative coordinates
			const chartContainer = document.getElementById(`chart_container-${chartId}`);
			const rect = chartContainer?.getBoundingClientRect();
			const offsetX = rect ? rect.left : 0;
			const offsetY = rect ? rect.top : 0;

			const relativeX = event.clientX - offsetX;
			const relativeY = event.clientY - offsetY;

			const overlay = {
				...v,
				width: Math.abs(relativeX - v.startX),
				height: Math.abs(relativeY - v.startY),
				x: Math.min(relativeX, v.startX),
				y: Math.min(relativeY, v.startY),
				currentPrice: chartCandleSeries.coordinateToPrice(relativeY) || 0
			};
			return overlay;
		});
	}

	function handleShiftOverlayEnd(event: MouseEvent) {
		shiftOverlay.update((v: ShiftOverlay): ShiftOverlay => {
			if (v.isActive) {
				// Re-enable chart interactions when drawing ends
				chart.applyOptions({
					handleScroll: true,
					handleScale: true,
					kineticScroll: {
						mouse: false,
						touch: false
					}
				});

				return {
					...v,
					isActive: false,
					width: 0,
					height: 0
				};
			}
			return v;
		});
		document.removeEventListener('mousemove', shiftOverlayTrack);
		document.removeEventListener('mouseup', handleShiftOverlayEnd);
	}

	async function updateLatestChartBar(trade: TradeData) {
		// Early returns for invalid data or conditions
		if (
			!trade?.price ||
			!trade?.size ||
			!trade?.timestamp ||
			!chartCandleSeries?.data()?.length ||
			isSwitchingTickers ||
			trade.conditions?.some((condition) => excludedConditions.has(condition))
		) {
			return;
		}

		const isExtendedHoursTrade = extendedHours(trade.timestamp);
		if (
			isExtendedHoursTrade &&
			(!currentChartInstance.extendedHours || /\d[dwm]/.test(currentChartInstance.timeframe))
		) {
			return;
		}
		const dolvol = get(settings).dolvol;
		const allCandleDataLive = chartCandleSeries.data();
		const mostRecentBarRaw = allCandleDataLive.at(-1);

		if (!mostRecentBarRaw || !isCandlestick(mostRecentBarRaw)) return;
		const mostRecentBar = mostRecentBarRaw as CandlestickData<Time>; // Now we know it's a CandlestickData

		let legendIsDisplayingCurrentLastCandle = false;
		const currentLegendData = get(hoveredCandleData);
		// Check if legend is showing the current last bar's data by comparing time and close price (as a heuristic)
		if (
			currentLegendData.close === mostRecentBar.close &&
			latestCrosshairPositionTime === mostRecentBar.time
		) {
			legendIsDisplayingCurrentLastCandle = true;
		}

		currentBarTimestamp = mostRecentBar.time as number;
		const tradeTime = UTCSecondstoESTSeconds(trade.timestamp / 1000);
		const sameBar = tradeTime < currentBarTimestamp + chartTimeframeInSeconds;

		if (sameBar) {
			if (!chartLatestDataReached) return;
			const now = Date.now();

			if (!pendingBarUpdate) {
				pendingBarUpdate = { ...mostRecentBar }; // Create a mutable copy
			}

			if (!pendingVolumeUpdate) {
				const lastVolumeRaw = chartVolumeSeries.data().at(-1);
				if (lastVolumeRaw && isHistogram(lastVolumeRaw)) {
					pendingVolumeUpdate = { ...(lastVolumeRaw as HistogramData<Time>) }; // Create a mutable copy
				}
			}

			pendingBarUpdate.high = Math.max(pendingBarUpdate.high, trade.price);
			pendingBarUpdate.low = Math.min(pendingBarUpdate.low, trade.price);
			pendingBarUpdate.close = trade.price;

			if (pendingVolumeUpdate) {
				pendingVolumeUpdate.value = (pendingVolumeUpdate.value || 0) + trade.size;
				pendingVolumeUpdate.color =
					pendingBarUpdate.close > pendingBarUpdate.open ? '#089981' : '#ef5350';
			}

			if (now - lastUpdateTime >= updateThrottleMs) {
				if (pendingBarUpdate) {
					chartCandleSeries.update(pendingBarUpdate);
					pendingBarUpdate = null;
				}
				if (pendingVolumeUpdate) {
					chartVolumeSeries.update(pendingVolumeUpdate);
					pendingVolumeUpdate = null;
				}
				lastUpdateTime = now;
				if (legendIsDisplayingCurrentLastCandle) {
					_refreshLegendWithLatestCandleData();
				}
			}
			return;
		} else {
			if (!chartLatestDataReached) return;

			if (pendingBarUpdate) {
				chartCandleSeries.update(pendingBarUpdate);
				pendingBarUpdate = null;
			}
			if (pendingVolumeUpdate) {
				chartVolumeSeries.update(pendingVolumeUpdate);
				pendingVolumeUpdate = null;
			}
			lastUpdateTime = Date.now();

			const referenceStartTime = getReferenceStartTimeForDateMilliseconds(
				trade.timestamp,
				currentChartInstance.extendedHours
			);
			const timeDiff = (trade.timestamp - referenceStartTime) / 1000;
			const flooredDifference =
				Math.floor(timeDiff / chartTimeframeInSeconds) * chartTimeframeInSeconds;
			const newTime = UTCSecondstoESTSeconds(
				referenceStartTime / 1000 + flooredDifference
			) as UTCTimestamp;

			chartCandleSeries.update({
				time: newTime,
				open: trade.price,
				high: trade.price,
				low: trade.price,
				close: trade.price
			});

			chartVolumeSeries.update({
				time: newTime,
				value: trade.size,
				color: '#089981'
			});

			if (legendIsDisplayingCurrentLastCandle) {
				_refreshLegendWithLatestCandleData();
				// Crucially, update latestCrosshairPositionTime to the NEW latest bar's time
				const currentCandles = chartCandleSeries
					.data()
					.filter(isCandlestick) as CandlestickData<Time>[];
				if (currentCandles.length > 0) {
					latestCrosshairPositionTime = currentCandles[currentCandles.length - 1].time as number;
				}
			}

			try {
				const timeToRequestForUpdatingAggregate =
					ESTSecondstoUTCSeconds(mostRecentBar.time as number) * 1000;
				const [barData] = await chartRequest<BarData[]>('getChartData', {
					securityId: chartSecurityId,
					timeframe: chartTimeframe,
					timestamp: timeToRequestForUpdatingAggregate,
					direction: 'backward',
					bars: 1,
					extendedHours: chartExtendedHours,
					isreplay: $streamInfo.replayActive
				});

				if (!barData) return;

				// Find and update the matching (previous) bar
				const allCandleDataForUpdate = chartCandleSeries
					.data()
					.filter(isCandlestick) as CandlestickData<Time>[];
				const barIndex = allCandleDataForUpdate.findIndex(
					(candle) => candle.time === UTCSecondstoESTSeconds(barData.time)
				);

				if (barIndex !== -1) {
					const updatedPrevCandle = {
						time: UTCSecondstoESTSeconds(barData.time) as UTCTimestamp,
						open: barData.open,
						high: barData.high,
						low: barData.low,
						close: barData.close
					};
					chartCandleSeries.update(updatedPrevCandle);

					const updatedPrevVolumeBar = {
						time: UTCSecondstoESTSeconds(barData.time) as UTCTimestamp,
						value: barData.volume * (dolvol ? barData.close : 1),
						color: barData.close > barData.open ? '#089981' : '#ef5350'
					};
					chartVolumeSeries.update(updatedPrevVolumeBar);

					// If legend was tracking live, it should still track the NEW latest bar.
					// Refresh its data as ADR might change due to previous bar update.
					if (legendIsDisplayingCurrentLastCandle) {
						_refreshLegendWithLatestCandleData();
						// Ensure latestCrosshairPositionTime still points to the current latest bar
						const currentCandles = chartCandleSeries
							.data()
							.filter(isCandlestick) as CandlestickData<Time>[];
						if (currentCandles.length > 0) {
							latestCrosshairPositionTime = currentCandles[currentCandles.length - 1]
								.time as number;
						}
					}
				}
			} catch (error) {
				console.error('Error fetching historical data for previous bar:', error);
			}
		}
	}

	// Add subscription to activeAlerts store to update alert lines
	$: if ($activeAlerts && chartCandleSeries) {
		// Remove existing alert lines
		alertLines.forEach((line) => {
			chartCandleSeries.removePriceLine(line.line);
		});
		alertLines = [];

		// Add new alert lines for price alerts
		$activeAlerts.forEach((alert) => {
			if (alert.alertType === 'price' && alert.alertPrice && alert.securityId === chartSecurityId) {
				const priceLine = chartCandleSeries.createPriceLine({
					price: alert.alertPrice,
					color: '#FFB74D', // Orange color for alert lines
					lineWidth: 1,
					lineStyle: 1, // Dashed line
					axisLabelVisible: true,
					title: `Alert: ${alert.alertPrice}`
					// Make lines unclickable by not adding any interactive properties
				});

				// Fix the alert ID type
				if (alert.alertId !== undefined) {
					alertLines.push({
						price: alert.alertPrice,
						line: priceLine,
						alertId: alert.alertId
					});
				}
			}
		});
	}

	function change(
		newReq: ChartQueryDispatch,
		preloadedResponse?: { bars: BarData[]; isEarliestData: boolean }
	) {
		// Reset pending updates when changing charts
		pendingBarUpdate = null;
		pendingVolumeUpdate = null;
		lastUpdateTime = 0;

		// Show overlay for new ticker changes
		if (newReq.requestType === 'loadNewTicker') {
			isSwitchingTickers = true;
		}

		const securityId =
			typeof newReq.securityId === 'string'
				? parseInt(newReq.securityId, 10)
				: newReq.securityId || 0;

		const chartId =
			typeof newReq.chartId === 'string' ? parseInt(newReq.chartId, 10) : newReq.chartId;
		const updatedReq: ChartInstance = {
			ticker: newReq.ticker || currentChartInstance.ticker,
			timestamp: newReq.timestamp || 0,
			timeframe: newReq.timeframe || currentChartInstance.timeframe,
			securityId: securityId,
			price: newReq.price ?? currentChartInstance.price,
			chartId: chartId,
			bars: newReq.bars ?? defaultBarsOnScreen,
			direction: newReq.direction,
			requestType: newReq.requestType,
			includeLastBar: newReq.includeLastBar,
			extendedHours: newReq.extendedHours ?? currentChartInstance.extendedHours
		};

		// Update currentChartInstance
		currentChartInstance = {
			...currentChartInstance,
			...updatedReq
		};

		// Update global chart state so other components know about the current chart
		if (typeof chartId === 'number') {
			setActiveChart(chartId, currentChartInstance);
		}

		// Fetch detailed ticker information including logo
		if (updatedReq.securityId) {
			fetchTickerDetails(updatedReq.securityId);
		}

		// Determine if viewing live data based on timestamp
		isViewingLiveData = updatedReq.timestamp === 0;
		// Clear quote lines if not viewing live data initially
		if (!isViewingLiveData) {
			clearQuoteLines();
		} else if (lastQuoteData) {
			// If switching back to live view and we have quote data, apply it (might be applied again later)
			applyLastQuote();
		}

		if (
			typeof chartId === 'number' &&
			typeof updatedReq.chartId === 'number' &&
			chartId !== updatedReq.chartId
		) {
			return;
		}

		if (!updatedReq.securityId || !updatedReq.ticker || !updatedReq.timeframe) {
			return;
		}

		// Rest of the function remains the same
		chartEarliestDataReached = false;
		chartLatestDataReached = false;
		chartSecurityId = updatedReq.securityId;
		chartTimeframe = updatedReq.timeframe;
		chartTimeframeInSeconds = timeframeToSeconds(updatedReq.timeframe);
		chartExtendedHours = updatedReq.extendedHours ?? false;

		// Apply time scale options
		if (chart) {
			if (
				updatedReq.timeframe?.includes('m') ||
				updatedReq.timeframe?.includes('w') ||
				updatedReq.timeframe?.includes('d') ||
				updatedReq.timeframe?.includes('q')
			) {
				chart.applyOptions({ timeScale: { timeVisible: false } });
			} else {
				chart.applyOptions({ timeScale: { timeVisible: true } });
			}
		}

		// Preserve trades data if it exists in newReq
		if ('trades' in newReq && Array.isArray(newReq.trades)) {
			updatedReq.trades = newReq.trades;
		}
		if (arrowSeries) {
			arrowSeries.setData([]);
		}

		// Clear existing alert lines when changing tickers
		if (chartCandleSeries) {
			alertLines.forEach((line) => {
				chartCandleSeries.removePriceLine(line.line);
			});
			alertLines = [];
		}

		// Reset session highlighting when changing securities
		if (sessionHighlighting) {
			chartCandleSeries.detachPrimitive(sessionHighlighting);
			sessionHighlighting = new SessionHighlighting(createDefaultSessionHighlighter());
			chartCandleSeries.attachPrimitive(sessionHighlighting);
		}

		// Use preloaded data (this is from the initial page load on going to /app)
		if (preloadedResponse) {
			processChartDataResponse(preloadedResponse, updatedReq, null);
		} else {
			backendLoadChartData(updatedReq);
		}
	}

	onMount(() => {
		// Keep onMount synchronous
		const chartOptions = {
			autoSize: true,
			crosshair: {
				mode: CrosshairMode.Normal
			},
			layout: {
				textColor: 'white',
				background: {
					type: ColorType.Solid,
					color: '#0f0f0f'
				},
				attributionLogo: false
			},
			grid: {
				vertLines: {
					visible: false
				},
				horzLines: {
					visible: false
				}
			},
			timeScale: {
				timeVisible: true,
				shiftVisibleRangeOnNewBar: false,
				borderColor: 'black'
			},
			rightPriceScale: {
				borderColor: 'black'
			},
			leftPriceScale: {
				borderColor: 'black'
			},
			kineticScroll: {
				mouse: false,
				touch: false
			}
		};
		const chartContainer = document.getElementById(`chart_container-${chartId}`);
		if (!chartContainer) {
			return;
		}
		//init event listeners
		chartContainer.addEventListener('contextmenu', (event: MouseEvent) => {
			event.preventDefault();
			if (!chartCandleSeries) return;

			const price = chartCandleSeries.coordinateToPrice(event.clientY);
			if (typeof price !== 'number') return;

			const timestamp = ESTSecondstoUTCMillis(latestCrosshairPositionTime);
			const roundedPrice = Math.round(price * 100) / 100;

			const inst: CoreInstance = {
				ticker: currentChartInstance.ticker,
				timestamp: timestamp,
				timeframe: currentChartInstance.timeframe,
				securityId: currentChartInstance.securityId,
				price: roundedPrice
			};
			queryInstanceRightClick(event, inst, 'chart');
		});
		chartContainer.addEventListener('keyup', (event) => {
			if (event.key == 'Shift') {
				shiftDown = false;
			}
		});

		// Add global shift key listeners for more robust detection
		const handleGlobalKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Shift') {
				shiftDown = true;
			}
		};

		const handleGlobalKeyUp = (event: KeyboardEvent) => {
			if (event.key === 'Shift') {
				shiftDown = false;
			}
		};

		document.addEventListener('keydown', handleGlobalKeyDown);
		document.addEventListener('keyup', handleGlobalKeyUp);
		chartContainer.addEventListener('mousedown', handleMouseDown);
		chartContainer.addEventListener('mouseup', () => {
			isPanning = false;
			if (queuedLoad != null) {
				queuedLoad();
			}
		});
		chartContainer.addEventListener('keydown', (event) => {
			if (chartId !== undefined) {
				setActiveChart(chartId, currentChartInstance);
			}

			// Handle all Alt key combinations first
			if (event.altKey) {
				// Prevent default behavior for all Alt key combinations
				event.preventDefault();
				event.stopPropagation();

				// Now handle specific Alt key combinations
				if (event.key == 'r') {
					// alt + r reset view
					if (currentChartInstance.timestamp && !$streamInfo.replayActive) {
						queryChart({ timestamp: 0 });
					} else {
						chart.timeScale().resetTimeScale();
					}
					return;
				} else if (event.key == 'h') {
					// IMPORTANT: Prevent default and stop propagation FIRST for Alt+H
					event.stopPropagation();
					event.preventDefault();

					const price = chartCandleSeries.coordinateToPrice(latestCrosshairPositionY);
					if (typeof price !== 'number') return;

					const roundedPrice = Math.round(price * 100) / 100;
					const securityId = currentChartInstance.securityId;
					addHorizontalLine(roundedPrice, securityId);
					return;
				} else if (event.key == 's') {
					// IMPORTANT: Prevent default and stop propagation FIRST for Alt+S
					event.stopPropagation();
					event.preventDefault();

					if (chartId !== undefined) {
						handleScreenshot(chartId.toString());
					}
					return;
				} else if (event.altKey) {
					// Block all other Alt key combinations from triggering the input window
					// This ensures that when Alt is pressed, nothing goes to the input window
					event.stopPropagation();
					event.preventDefault();
					return;
				}

				// For any other Alt key combinations, just return
				// This prevents them from triggering the input window
				return;
			}

			// Handle Tab key to show extended hours toggle
			if (event.key === 'Tab') {
				event.preventDefault();
				event.stopPropagation();
				showExtendedHoursToggle();
				return;
			}

			// Handle non-Alt key combinations
			if (!event.ctrlKey && !event.metaKey && /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
				// Prevent default and stop propagation immediately
				event.preventDefault();
				event.stopPropagation();

				// Add the keypress to our buffer
				keyBuffer.push(event.key);

				// If this is the first key, start the input process
				if (!isInputActive) {
					isInputActive = true;

					// Create the initial instance with the buffer contents so far
					const initialInputString = keyBuffer.join('');
					const partialInstance: ChartInstance = {
						ticker: currentChartInstance.ticker,
						timestamp: currentChartInstance.timestamp,
						timeframe: currentChartInstance.timeframe,
						securityId: ensureNumericSecurityId(currentChartInstance),
						price: currentChartInstance.price,
						chartId: currentChartInstance.chartId,
						extendedHours: currentChartInstance.extendedHours,
						inputString: initialInputString // Pass the buffer as the initial string
					};

					// Initiate the input window with the whole buffer string
					queryInstanceInput('any', 'any', partialInstance)
						.then((value: CoreInstance) => {
							// Handle normal completion
							isInputActive = false;
							keyBuffer = []; // Clear buffer

							const securityId =
								typeof value.securityId === 'string'
									? parseInt(value.securityId, 10)
									: value.securityId || 0;

							const extendedV: ChartInstance = {
								ticker: value.ticker || '',
								timestamp: value.timestamp || 0,
								timeframe: value.timeframe || '',
								securityId: securityId,
								price: value.price || 0,
								chartId: currentChartInstance.chartId,
								extendedHours: value.extendedHours
							};
							currentChartInstance = extendedV;
							queryChart(extendedV, true);
							setTimeout(() => chartContainer.focus(), 0);
						})
						.catch((error) => {
							isInputActive = false;
							keyBuffer = []; // Clear buffer
							// Only log errors that are *not* 'User cancelled input'
							if (error && error.message !== 'User cancelled input') {
								console.error('Input error:', error);
							}
							setTimeout(() => chartContainer.focus(), 0);
						});
				}
			} else if (event.key == 'Shift') {
				shiftDown = true;
			} else if (event.key == 'Escape') {
				// Clear buffer on escape
				keyBuffer = [];
				isInputActive = false;

				if (get(shiftOverlay).isActive) {
					shiftOverlay.update((v: ShiftOverlay): ShiftOverlay => {
						if (v.isActive) {
							return {
								...v,
								isActive: false,
								width: 0,
								height: 0
							};
						}
						return v;
					});
				}
			}
		});
		chart = createChart(chartContainer, chartOptions);

		// Then add your candlestick / volume / etc. after or before.
		chartCandleSeries = chart.addCandlestickSeries({
			priceLineVisible: false,
			upColor: '#089981',
			downColor: '#ef5350',
			borderVisible: false,
			wickUpColor: '#089981',
			wickDownColor: '#ef5350'
		});
		chartVolumeSeries = chart.addHistogramSeries({
			lastValueVisible: false,
			priceLineVisible: false,
			priceFormat: { type: 'volume' },
			priceScaleId: ''
		});
		chartVolumeSeries
			.priceScale()
			.applyOptions({ scaleMargins: { top: 0.8, bottom: 0 }, visible: false });
		chartCandleSeries.priceScale().applyOptions({ scaleMargins: { top: 0.1, bottom: 0.2 } });
		const smaOptions: DeepPartial<LineStyleOptions & SeriesOptionsCommon> = {
			lineWidth: 1,
			priceLineVisible: false,
			lastValueVisible: false
		};
		sma10Series = chart.addLineSeries({ color: 'purple', ...smaOptions });
		sma20Series = chart.addLineSeries({ color: 'blue', ...smaOptions });
		vwapSeries = chart.addLineSeries({ color: 'white', ...smaOptions });
		//rvolSeries = chart.addLineSeries({color:'green',...smaOptions})
		bidLine = chart.addLineSeries({
			color: 'white',
			lineWidth: 2,
			lastValueVisible: true, // Shows the price on the right
			priceLineVisible: false
		});
		askLine = chart.addLineSeries({
			color: 'white',
			lineWidth: 2,
			lastValueVisible: true, // Shows the price on the right
			priceLineVisible: false
		});
		arrowSeries = chart.addCustomSeries<ArrowMarker, CustomSeriesOptions>(
			new ArrowMarkersPaneView(),
			{
				title: '',
				priceScaleId: 'right'
			}
		);
		eventMarkerView = new EventMarkersPaneView();
		eventSeries = chart.addCustomSeries(eventMarkerView, {
			priceFormat: {
				type: 'custom',
				minMove: 0.00000001,
				formatter: (price: any) => {
					return '';
				}
			}
		}) as ISeriesApi<'Custom', Time, EventMarker>;

		chart.subscribeCrosshairMove((param) => {
			// Type guard for CandlestickData (local to this callback)
			const isCandlestickLocal = (data: any): data is CandlestickData<Time> =>
				data &&
				typeof data === 'object' &&
				'open' in data &&
				'high' in data &&
				'low' in data &&
				'close' in data;
			// Type guard for HistogramData (local to this callback)
			const isHistogramLocal = (data: any): data is HistogramData<Time> =>
				data && typeof data === 'object' && 'value' in data;

			const candleSeriesActualData = chartCandleSeries.data(); // This includes WhitespaceData
			const volumeSeriesActualData = chartVolumeSeries.data();

			if (!candleSeriesActualData.length || !currentChartInstance.securityId) {
				hoveredCandleData.set(defaultHoveredCandleData);
				return;
			}

			// Filter out whitespace to get only actual chart data points
			const allCandles = candleSeriesActualData.filter(
				isCandlestickLocal
			) as CandlestickData<Time>[];
			const allVolumes = volumeSeriesActualData.filter(isHistogramLocal) as HistogramData<Time>[];

			if (!allCandles.length) {
				// If, after filtering, there are no valid candles
				hoveredCandleData.set(defaultHoveredCandleData);
				return;
			}

			let barToUse: CandlestickData<Time>;
			let volumeForBar: number = 0;
			let indexOfBarInAllCandles: number;

			const lastCandleInSeries = allCandles[allCandles.length - 1];
			const lastCandleIndex = allCandles.length - 1;

			const hoveredCandleFromParam =
				param && param.seriesData ? param.seriesData.get(chartCandleSeries) : undefined;
			const logicalIdx = param?.logical; // The logical index of the bar under the crosshair

			// Decision logic for which bar's data to display
			if (logicalIdx !== undefined && logicalIdx > lastCandleIndex) {
				// Case 1: Crosshair is logically to the right of the last available candle.
				barToUse = lastCandleInSeries;
				indexOfBarInAllCandles = lastCandleIndex;
			} else if (param && param.time !== undefined && isCandlestickLocal(hoveredCandleFromParam)) {
				// Case 2: Crosshair is over a valid candle data point, and not logically to the right.
				// Ensure this hovered candle is one of our 'allCandles'.
				const potentialBarIndex = allCandles.findIndex(
					(c) => c.time === hoveredCandleFromParam.time
				);
				if (potentialBarIndex !== -1) {
					barToUse = hoveredCandleFromParam;
					indexOfBarInAllCandles = potentialBarIndex;
				} else {
					// Fallback: If hovered candle isn't in our filtered list (should be rare), use last.
					barToUse = lastCandleInSeries;
					indexOfBarInAllCandles = lastCandleIndex;
				}
			} else {
				// Case 3: Crosshair is not over a specific candle (e.g., off chart, between bars) or param is invalid.
				barToUse = lastCandleInSeries;
				indexOfBarInAllCandles = lastCandleIndex;
			}

			// Get volume for the barToUse
			// Try to get volume from param if it matches the barToUse's time (most direct)
			const correspondingVolumeFromParam =
				param && param.seriesData ? param.seriesData.get(chartVolumeSeries) : undefined;
			if (
				isHistogramLocal(correspondingVolumeFromParam) &&
				correspondingVolumeFromParam.time === barToUse.time
			) {
				volumeForBar = correspondingVolumeFromParam.value;
			} else {
				// Fallback: find volume by time from allVolumes array
				const volumeMatch = allVolumes.find((v) => v.time === barToUse.time);
				if (volumeMatch) {
					volumeForBar = volumeMatch.value;
				} else if (barToUse.time === lastCandleInSeries.time && allVolumes.length > 0) {
					// If it's the last candle and no direct time match for volume,
					// try taking the absolute last volume entry as a weaker fallback.
					volumeForBar = allVolumes[allVolumes.length - 1].value;
				}
				// if no volume found, volumeForBar remains 0 (initialized value)
			}

			// Calculations (ADR, CHG) based on barToUse and indexOfBarInAllCandles
			let barsForADR;
			if (indexOfBarInAllCandles >= 19) {
				// Need 20 bars for ADR (current + 19 previous)
				barsForADR = allCandles.slice(indexOfBarInAllCandles - 19, indexOfBarInAllCandles + 1);
			} else {
				barsForADR = allCandles.slice(0, indexOfBarInAllCandles + 1);
			}

			let chg = 0;
			let chgprct = 0;
			if (indexOfBarInAllCandles > 0) {
				const prevBar = allCandles[indexOfBarInAllCandles - 1];
				chg = barToUse.close - prevBar.close;
				chgprct = (barToUse.close / prevBar.close - 1) * 100;
			}
			// If indexOfBarInAllCandles is 0 (first bar), chg and chgprct remain 0, which is correct.

			hoveredCandleData.set({
				open: barToUse.open,
				high: barToUse.high,
				low: barToUse.low,
				close: barToUse.close,
				volume: volumeForBar,
				adr: calculateSingleADR(barsForADR), // calculateSingleADR expects CandlestickData[]
				chg: chg,
				chgprct: chgprct,
				rvol: 0 // RVOL calculation is currently commented out in the original code
			});

			latestCrosshairPositionTime = barToUse.time as number;
			latestCrosshairPositionY = param && param.point ? param.point.y : 0;
		});
		chart.timeScale().subscribeVisibleLogicalRangeChange((logicalRange) => {
			if (selectedEvent) {
				selectedEvent = null; // Close popup on scroll/pan
			}
			if (!logicalRange || Date.now() - lastChartQueryDispatchTime < chartRequestThrottleDuration) {
				return;
			}
			if (isLoadingAdditionalData || isSwitchingTickers) {
				return;
			}
			const barsOnScreen = Math.floor(logicalRange.to) - Math.ceil(logicalRange.from);
			const bufferInScreenSizes = 0.7;

			// Backward loading condition:
			// Original condition: logicalRange.from / barsOnScreen < bufferInScreenSizes
			// Corrected condition: Check if number of bars to the left is less than the buffer
			if (logicalRange.from < 30 && logicalRange.from < bufferInScreenSizes * barsOnScreen) {
				if (!chartEarliestDataReached) {
					// Get the earliest timestamp from current data
					const earliestBar = chartCandleSeries.data()[0];
					if (!earliestBar) return;

					// Convert the earliest time from EST seconds to UTC milliseconds for the API request
					const earliestTimestamp = ESTSecondstoUTCMillis(earliestBar.time as UTCTimestamp);

					// Make sure to include extendedHours in the request
					const inst: CoreInstance & { extendedHours?: boolean } = {
						ticker: currentChartInstance.ticker,
						timestamp: earliestTimestamp,
						timeframe: currentChartInstance.timeframe,
						securityId: currentChartInstance.securityId,
						price: currentChartInstance.price,
						extendedHours: currentChartInstance.extendedHours
					};

					backendLoadChartData({
						...inst,
						bars: Math.floor(bufferInScreenSizes * barsOnScreen) + 100,
						direction: 'backward',
						requestType: 'loadAdditionalData',
						includeLastBar: true
					});
				}
			}
			if (
				(chartCandleSeries.data().length - logicalRange.to) / barsOnScreen <
				bufferInScreenSizes
			) {
				// forward load
				if (chartLatestDataReached) {
					return;
				}
				if ($streamInfo.replayActive) {
					return;
				}
				const lastBar = chartCandleSeries.data().at(-1);
				if (!lastBar) return; // Exit if no data exists

				// Convert the last bar's time from EST seconds to UTC milliseconds for the API request
				const requestTimestamp = ESTSecondstoUTCMillis(lastBar.time as UTCTimestamp);

				const inst: CoreInstance & { extendedHours?: boolean } = {
					ticker: currentChartInstance.ticker,
					timestamp: requestTimestamp, // Correct: Use the last loaded bar's timestamp
					timeframe: currentChartInstance.timeframe,
					securityId: currentChartInstance.securityId,
					price: currentChartInstance.price,
					extendedHours: currentChartInstance.extendedHours
				};
				backendLoadChartData({
					...inst,
					bars: Math.floor(bufferInScreenSizes * barsOnScreen) + 100,
					direction: 'forward',
					requestType: 'loadAdditionalData',
					includeLastBar: true
				});
			}
		});

		// Add mousemove handler for the chart container
		const container = document.getElementById(`chart_container-${chartId}`);
		container?.addEventListener('mousemove', (e) => {
			const rect = container.getBoundingClientRect();
			const x = e.clientX - rect.left;
			const y = e.clientY - rect.top;

			// Pass mouse move to event marker view
			if (eventMarkerView && eventMarkerView.handleMouseMove(x, y)) {
				// If the state changed and we need a redraw, request one
				chart.applyOptions({});
			}
		});

		container?.addEventListener('mouseleave', () => {
			if (eventMarkerView && eventMarkerView.clearHover()) {
				// If we cleared the hover state, request a redraw
				hoveredEvent = null;
				chart.applyOptions({});
			}
		});

		container?.addEventListener('click', (e) => {
			const rect = container.getBoundingClientRect();
			const x = e.clientX - rect.left;
			const y = e.clientY - rect.top;

			if (eventMarkerView.handleClick(x, y)) {
				e.preventDefault();
				e.stopPropagation();
			} else {
				// Click outside filing - close the popup
				selectedEvent = null;
			}
		});

		eventMarkerView.setClickCallback(handleEventClick);
		eventMarkerView.setHoverCallback(handleEventHover);

		// Add subscriptions after chart initialization
		chartQueryDispatcher.subscribe((req: ChartQueryDispatch) => {
			change(req);
		});

		chartEventDispatcher.subscribe((e: ChartEventDispatch) => {
			if (!currentChartInstance || !currentChartInstance.securityId) return;
			if (e.event == 'replay') {
				currentChartInstance.timestamp = 0;
				const req: ChartQueryDispatch = {
					...currentChartInstance,
					bars: 300,
					direction: 'backward',
					requestType: 'loadNewTicker',
					includeLastBar: false,
					chartId: chartId
				};
				change(req);
			} else if (e.event == 'addHorizontalLine') {
				addHorizontalLine(e.data.price, e.data.securityId);
			}
		});

		chartContainer.setAttribute('tabindex', '0'); // Make container focusable
		chartContainer.focus(); // Focus the container

		// Create session highlighting
		if (chart) {
			sessionHighlighting = new SessionHighlighting(createDefaultSessionHighlighter());
			chartCandleSeries.attachPrimitive(sessionHighlighting);
		}

		const loadDefaultChart = async () => {
			// Check if preloaded data is available
			if (defaultChartData) {
				// Use server-side preloaded data
				const chartQueryDispatch: ChartQueryDispatch = {
					ticker: defaultChartData.ticker,
					timestamp: defaultChartData.timestamp,
					timeframe: defaultChartData.timeframe,
					securityId: defaultChartData.securityId,
					price: defaultChartData.price,
					chartId: chartId,
					extendedHours: false,
					bars: defaultChartData.bars,
					direction: 'backward',
					requestType: 'loadNewTicker',
					includeLastBar: true
				};

				const preloadedResponse = {
					bars: defaultChartData.chartData.bars,
					isEarliestData: defaultChartData.chartData.isEarliestData || false
				};

				change(chartQueryDispatch, preloadedResponse);
				return;
			}

			// Load default SPY chart if no current instance
			if (!currentChartInstance || !currentChartInstance.ticker) {
				try {
					type SecurityIdResponse = { securityId?: number };
					const response = await publicRequest<SecurityIdResponse>(
						'getSecurityIDFromTickerTimestamp',
						{
							ticker: 'SPY',
							timestampMs: 0
						}
					);

					const spySecurityId = response?.securityId ?? 0;

					if (spySecurityId !== 0) {
						queryChart({
							ticker: 'SPY',
							timeframe: '1d',
							timestamp: 0,
							securityId: spySecurityId,
							price: 0
						});
					} else {
						console.warn('Could not fetch securityId for default ticker SPY.');
					}
				} catch (error) {
					console.error('Error fetching securityId for default ticker SPY:', error);
				}
			}
		};

		loadDefaultChart();

		return () => {
			// Apply any pending updates before unmounting
			if (pendingBarUpdate) {
				chartCandleSeries.update(pendingBarUpdate);
			}

			if (pendingVolumeUpdate) {
				chartVolumeSeries.update(pendingVolumeUpdate);
			}

			// Clean up global shift key listeners
			document.removeEventListener('keydown', handleGlobalKeyDown);
			document.removeEventListener('keyup', handleGlobalKeyUp);
		};
	});

	// Handle event marker clicks
	function handleEventClick(events: EventMarker['events'], x: number, y: number, time: number) {
		// Check if clicking on the same event that's already selected
		if (
			selectedEvent &&
			Math.abs(selectedEvent.x - x) < 10 &&
			Math.abs(selectedEvent.y - y) < 10 &&
			selectedEvent.events.length === events.length
		) {
			// If clicking on the same event, close the popup
			selectedEvent = null;
		} else {
			// Otherwise, select the new event
			selectedEvent = { events, x, y, time };
			hoveredEvent = null; // Clear hover when clicked
		}
	}

	// Handle event marker hover
	function handleEventHover(events: EventMarker['events'] | null, x: number, y: number) {
		if (events) {
			hoveredEvent = { events, x, y };
		} else {
			hoveredEvent = null;
		}
	}

	// Handle closing the event popup
	function closeEventPopup() {
		selectedEvent = null;
	}

	// Function to calculate optimal position for event-info popup
	function calculateEventInfoPosition(containerWidth: number, containerHeight: number) {
		// Try to get the actual legend element and its dimensions
		const legendElement = document.querySelector(`#chart_container-${chartId} .legend`);
		let legendRight = 300; // Default right edge position
		let legendBottom = 100; // Default bottom position

		if (legendElement) {
			const legendRect = legendElement.getBoundingClientRect();
			const chartContainer = document.querySelector(`#chart_container-${chartId}`);
			const chartRect = chartContainer?.getBoundingClientRect();

			if (chartRect) {
				// Calculate legend's right edge and bottom relative to chart container
				legendRight = legendRect.right - chartRect.left;
				legendBottom = legendRect.bottom - chartRect.top;
			}
		}

		const margin = 10; // Margin from legend and edges
		const minPopupWidth = 200; // Minimum popup width

		// Calculate available space to the right of legend
		const availableWidth = containerWidth - legendRight - margin * 2;

		// Set popup width to available space (with min/max constraints)
		const popupWidth = Math.max(minPopupWidth, Math.min(availableWidth, 350));

		// Position to the right of the legend with margin
		let leftPosition = legendRight + margin;

		// Ensure popup doesn't go beyond right edge
		if (leftPosition + popupWidth > containerWidth - margin) {
			leftPosition = containerWidth - popupWidth - margin;
		}

		// Ensure popup doesn't go beyond left edge (shouldn't happen but safety check)
		leftPosition = Math.max(margin, leftPosition);

		// For vertical positioning, start at top but ensure it doesn't go beyond bottom
		let topPosition = 5;
		const maxHeight = containerHeight - topPosition - margin;

		return {
			left: leftPosition,
			top: topPosition,
			width: popupWidth,
			maxHeight: maxHeight
		};
	}

	// New interface for the data object
	interface ChartData {
		type?: string;
		url?: string;
		ratio?: number;
		amount?: number;
	}

	// Type guard for the data object
	function isChartData(data: unknown): data is ChartData {
		return typeof data === 'object' && data !== null;
	}

	// Use in the code
	const data: ChartData = {};
	if (isChartData(data)) {
		if (data.type && data.url) {
			// Handle type and url
		}
		if (typeof data.ratio === 'number') {
			// Handle ratio
		}
		if (typeof data.amount === 'number') {
			// Handle amount
		}
	}

	function ensureNumericSecurityId(instance: ExtendedInstance | CoreInstance): number {
		const securityId = instance.securityId;
		if (typeof securityId === 'string') {
			return parseInt(securityId, 10);
		}
		if (typeof securityId === 'number') {
			return securityId;
		}
		return 0; // Default value if undefined
	}

	// Function to add SEC filing to chat context
	function addFilingToChat(filingEvent: EventMarker['events'][number]) {
		if (!selectedEvent || !filingEvent) return;

		const timestampMs = selectedEvent.time * 1000; // convert seconds to ms
		const filingContext: FilingContext = {
			ticker: currentChartInstance.ticker,
			securityId: currentChartInstance.securityId,
			timestamp: timestampMs,
			filingType: filingEvent.title,
			link: filingEvent.url || ''
		};
		addFilingToChatContext(filingContext);
	}

	// Define the onSummarize function
	function onSummarize(filingEvent: EventMarker['events'][number]) {
		if (!selectedEvent || !filingEvent || !filingEvent.url) return; // Ensure URL exists for summarization

		const timestampMs = selectedEvent.time * 1000; // convert seconds to ms
		const filingContext: FilingContext = {
			ticker: currentChartInstance.ticker,
			securityId: currentChartInstance.securityId,
			timestamp: timestampMs,
			filingType: filingEvent.title,
			link: filingEvent.url // Use the non-optional URL
		};

		// Call the function from chat interface
		openChatAndQuery(
			filingContext,
			`Summarize the attached filing and any relevant exhibits: ${filingEvent.title}`
		);
	}
</script>

<div
	class="chart"
	id="chart_container-{chartId}"
	style="width: {width}px; position: relative;"
	tabindex="-1"
>
	<Legend instance={currentChartInstance} {hoveredCandleData} {width} />
	<Shift {shiftOverlay} />
	<DrawingMenu {drawingMenuProps} />

	<!-- Chart switching overlay -->
	{#if isSwitchingTickers}
		<div class="chart-switching-overlay"></div>
	{/if}

	<!-- Company Logo positioned at bottom right where axes meet -->
	{#if currentChartInstance?.logo || currentChartInstance?.icon}
		<div class="chart-logo-container">
			<img
				src={currentChartInstance.logo || currentChartInstance.icon}
				alt="{currentChartInstance?.name || 'Company'} logo"
				class="chart-company-logo"
			/>
		</div>
	{:else if currentChartInstance?.ticker}
		<!-- Debug fallback: show ticker letter if no logo/icon available -->
		<div class="chart-logo-container">
			<div class="chart-ticker-fallback">
				{currentChartInstance.ticker.charAt(0)}
			</div>
		</div>
	{/if}
</div>

<!-- Why Moving Popup -->
<!-- <WhyMoving ticker={whyMovingTicker} trigger={whyMovingTrigger} /> -->

<!-- Replace the filing info overlay with a more generic event info overlay -->
{#if selectedEvent}
	{@const chartContainer = document.getElementById(`chart_container-${chartId}`)}
	{@const containerHeight = chartContainer?.clientHeight || 600}
	{@const position = calculateEventInfoPosition(width, containerHeight)}
	<div
		class="event-info"
		style="
            left: {position.left}px;
            top: {position.top}px;
            width: {position.width}px;
            max-height: {position.maxHeight}px;"
	>
		<div class="event-header">
			{#if selectedEvent.events[0]?.type === 'sec_filing'}
				<div class="event-title">SEC Filings</div>
			{:else if selectedEvent.events[0]?.type === 'split'}
				<div class="event-title">Stock Splits</div>
			{:else if selectedEvent.events[0]?.type === 'dividend'}
				<div class="event-title">Dividends</div>
			{:else}
				<div class="event-title">Events</div>
			{/if}
			<button class="close-button" on:click={closeEventPopup}>×</button>
		</div>
		<div class="event-content">
			{#each selectedEvent.events as filing}
				{#if filing.type === 'sec_filing'}
					<div class="event-row filing-row">
						<div class="filing-info">
							<span class="event-type">{filing.title}</span>
						</div>
						<div class="event-actions">
							{#if filing.url}
								<a
									href={filing.url}
									target="_blank"
									rel="noopener noreferrer"
									class="btn btn-primary btn-sm"
									on:click|stopPropagation={() => addFilingToChat(filing)}
								>
									View →
								</a>
							{/if}
							<button
								class="btn btn-secondary btn-sm"
								on:click|stopPropagation={() => addFilingToChat(filing)}
							>
								+ Chat
							</button>
							<button
								class="btn btn-tertiary btn-sm"
								on:click|stopPropagation={() => onSummarize(filing)}
								disabled={!filing.url}
								title={!filing.url ? 'No link available to summarize' : 'Summarize this filing'}
							>
								Summarize
							</button>
						</div>
					</div>
				{:else if filing.type === 'split'}
					<div class="event-row">
						<span class="event-type">{filing.title}</span>
					</div>
				{:else if filing.type === 'dividend'}
					<div class="event-row dividend-row">
						<div class="dividend-details">
							<span class="event-type">{filing.title}</span>
							{#if filing.exDate}
								<span class="dividend-date">Ex-Date: {filing.exDate}</span>
							{/if}
							{#if filing.payoutDate}
								<span class="dividend-date">Payout: {filing.payoutDate}</span>
							{/if}
						</div>
					</div>
				{:else}
					<div class="event-row">
						<span class="event-type">{filing.title}</span>
					</div>
				{/if}
			{/each}
		</div>
	</div>
{/if}

<style>
	.event-info {
		position: absolute;
		background: rgba(37, 37, 37, 0.8);
		border: none;
		border-radius: 8px;
		padding: 8px 10px 10px 10px;
		z-index: 1000;
		/* Width and max-height now set via inline styles for dynamic sizing */
		min-width: 200px;
		overflow-y: auto;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
		transform: scale(0.95);
		transform-origin: top left;
		opacity: 0;
		animation: fadeInDown 0.2s ease-out forwards;
		word-wrap: break-word;
		hyphens: auto;
	}
	@keyframes fadeInDown {
		from {
			opacity: 0;
			transform: scale(0.85);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	.event-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding-bottom: 8px;
		border-bottom: 1px solid #444;
		margin-bottom: 8px;
		position: relative;
	}
	.close-button {
		background: transparent;
		border: none;
		color: #bbb;
		font-size: 16px;
		cursor: pointer;
		padding: 4px;
		transition: color 0.2s;
	}
	.close-button:hover {
		color: #fff;
	}
	.event-row {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		padding: 6px 0;
		border-bottom: 1px solid #444;
		color: #e0e0e0;
		text-decoration: none;
		transition: background 0.2s;
	}
	.event-row:hover {
		background: rgba(255, 255, 255, 0.05);
	}
	.event-type {
		font-size: 0.95rem;
		color: #fff;
		font-weight: 500;
		word-wrap: break-word;
		line-height: 1.3;
	}
	.dividend-date {
		font-size: 0.85rem;
		color: #ccc;
		line-height: 1.2;
	}
	.dividend-details {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}
	.event-actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.4rem;
		align-items: flex-start;
	}
	/* Sleek button and filing styles */
	.btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.4rem 0.8rem;
		border-radius: 4px;
		font-size: 0.85rem;
		font-weight: 600;
		text-decoration: none;
		transition:
			background-color 0.2s ease,
			color 0.2s ease;
	}
	.btn-primary {
		background-color: var(--accent-color, #3a8bf7);
		color: #fff;
		border: none;
	}
	.btn-primary:hover {
		background-color: var(--accent-color-dark, #336ecf);
	}
	.btn-secondary {
		background-color: transparent;
		color: var(--text-primary, #fff);
		border: 1px solid var(--accent-color, #3a8bf7);
	}
	.btn-secondary:hover {
		background-color: var(--accent-color, #3a8bf7);
		color: #fff;
	}
	.btn-tertiary {
		background-color: transparent;
		color: var(--text-secondary, #aaa);
		border: 1px solid var(--ui-border, #444);
	}
	.btn-tertiary:hover {
		background-color: rgba(255, 255, 255, 0.1);
		color: #fff;
	}
	.filing-row {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		padding: 0.5rem 0;
		border-bottom: 1px solid #444;
	}
	.filing-info .event-type {
		font-size: 1rem;
		font-weight: 600;
		color: #fff;
		word-wrap: break-word;
		line-height: 1.2;
	}
	.filing-row .event-actions {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		gap: 0.4rem;
	}
	.btn-sm {
		padding: 0.15rem 0.3rem;
		font-size: 0.7rem;
		height: 1.3rem;
	}

	.chart-switching-overlay {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.4);
		z-index: 500; /* Above chart but below legend and UI elements */
		opacity: 0;
		animation: fadeInOverlay 0.2s ease-out forwards;
		pointer-events: none; /* Allow clicks to pass through */
	}

	@keyframes fadeInOverlay {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	/* Chart logo styles positioned at bottom right where axes meet */
	.chart-logo-container {
		position: absolute;
		bottom: 2px;
		right: 2px;
		z-index: 1000; /* High z-index to appear above chart canvas */
		pointer-events: none;
		opacity: 0.85;
		transition: opacity 0.2s ease;
		padding: 2px;
	}

	.chart-logo-container:hover {
		opacity: 1;
	}

	.chart-company-logo {
		height: 18px;
		max-width: 50px;
		object-fit: contain;
		filter: brightness(0.9) contrast(0.95);
		transition: filter 0.2s ease;
		display: block;
	}

	.chart-company-logo:hover {
		filter: brightness(1) contrast(1);
	}

	/* Fallback ticker display for debugging */
	.chart-ticker-fallback {
		width: 20px;
		height: 18px;
		display: flex;
		align-items: center;
		justify-content: center;
		color: rgba(255, 255, 255, 0.8);
		font-size: 10px;
		font-weight: bold;
		font-family: monospace;
	}
</style>
