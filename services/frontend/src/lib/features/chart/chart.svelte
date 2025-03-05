<!-- chart.svelte-->
<script lang="ts">
	import Legend from './legend.svelte';
	import Shift from './shift.svelte';
	import DrawingMenu from './drawingMenu.svelte';
	import { privateRequest } from '$lib/core/backend';
	import { type DrawingMenuProps, addHorizontalLine, drawingMenuProps } from './drawingMenu.svelte';
	import type { Instance as CoreInstance, TradeData, QuoteData } from '$lib/core/types';
	import {
		setActiveChart,
		chartQueryDispatcher,
		chartEventDispatcher,
		queryChart
	} from './interface';
	import { streamInfo, settings, activeAlerts } from '$lib/core/stores';
	import type { ShiftOverlay, ChartEventDispatch, BarData, ChartQueryDispatch } from './interface';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { queryInstanceRightClick } from '$lib/utils/popups/rightClick.svelte';
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
	import {
		calculateRVOL,
		calculateSingleADR,
		calculateVWAP,
		calculateMultipleSMAs
	} from './indicators';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';
	import { onMount } from 'svelte';
	import {
		UTCSecondstoESTSeconds,
		ESTSecondstoUTCSeconds,
		ESTSecondstoUTCMillis,
		getReferenceStartTimeForDateMilliseconds,
		timeframeToSeconds
	} from '$lib/core/timestamp';
	import { addStream } from '$lib/utils/stream/interface';
	import { ArrowMarkersPaneView, type ArrowMarker } from './arrowMarkers';
	import { EventMarkersPaneView, type EventMarker } from './eventMarkers';
	import { adjustEventsToTradingDays, handleScreenshot } from './chartHelpers';
	import { SessionHighlighting, createDefaultSessionHighlighter } from './sessionShade';

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
	let isLoadingChartData = false;
	let lastChartQueryDispatchTime = 0;
	let queuedLoad: Function | null = null;
	let shiftDown = false;
	const chartRequestThrottleDuration = 150;
	const bufferInScreenSizes = 2;
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
	let blockingChartQueryDispatch = {};
	let isPanning = false;
	const excludedConditions = new Set([2, 7, 10, 13, 15, 16, 20, 21, 22, 29, 33, 37]);
	let mouseDownStartX = 0;
	let mouseDownStartY = 0;
	const DRAG_THRESHOLD = 3; // pixels of movement before considered a drag

	// Add new interface for alert lines
	interface AlertLine {
		price: number;
		line: any; // Use any for now since we don't have the full IPriceLine type
		alertId: number;
	}

	// Add new property to track alert lines
	let alertLines: AlertLine[] = [];

	let arrowSeries: any = null; // Initialize as null
	let eventSeries: ISeriesApi<'Custom', Time, EventMarker>;
	let eventMarkerView: EventMarkersPaneView;
	let selectedEvent: {
		events: EventMarker['events'];
		x: number;
		y: number;
	} | null = null;
	let hoveredEvent: {
		events: EventMarker['events'];
		x: number;
		y: number;
	} | null = null;

	let sessionHighlighting: SessionHighlighting;

	// Add throttling variables for chart updates
	let pendingBarUpdate: any = null;
	let pendingVolumeUpdate: any = null;
	let lastUpdateTime = 0;
	const updateThrottleMs = 100;

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

	interface ScreenshotOptions {
		type: string;
		url: string;
		ratio: number;
	}

	type StreamReleaseFunction = () => void;

	function extendedHours(timestamp: number): boolean {
		const date = new Date(timestamp);
		const minutes = date.getHours() * 60 + date.getMinutes();
		return minutes < 570 || minutes >= 960; // 9:30 AM - 4:00 PM EST
	}

	function backendLoadChartData(inst: ChartQueryDispatch): void {
		console.log(inst);
		eventSeries.setData([]);
		if (inst.requestType === 'loadNewTicker') {
			bidLine.setData([]);
			askLine.setData([]);
			arrowSeries.setData([]);
		}
		if (isLoadingChartData || !inst.ticker || !inst.timeframe || !inst.securityId) {
			return;
		}
		isLoadingChartData = true;
		lastChartQueryDispatchTime = Date.now();
		if (
			$streamInfo.replayActive &&
			(inst.timestamp == 0 || (inst.timestamp ?? 0) > $streamInfo.timestamp)
		) {
			('adjusting to stream timestamp');
			inst.timestamp = Math.floor($streamInfo.timestamp);
		}
		inst;
		inst.extendedHours;
		privateRequest<{ bars: BarData[]; isEarliestData: boolean }>('getChartData', {
			securityId: inst.securityId,
			timeframe: inst.timeframe,
			timestamp: inst.timestamp,
			direction: inst.direction,
			bars: inst.bars,
			extendedhours: inst.extendedHours,
			isreplay: $streamInfo.replayActive
		})
			.then((response) => {
				const barDataList = response.bars;
				console.log(response);
				blockingChartQueryDispatch = inst;
				if (!(Array.isArray(barDataList) && barDataList.length > 0)) {
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
				} else if (inst.requestType === 'loadAdditionalData') {
					const latestCandleTime =
						chartCandleSeries.data()[chartCandleSeries.data().length - 1]?.time;
					if (typeof latestCandleTime === 'number' && newCandleData[0].time >= latestCandleTime) {
						newCandleData = [...chartCandleSeries.data(), ...newCandleData.slice(1)] as any;
						newVolumeData = [...chartVolumeSeries.data(), ...newVolumeData.slice(1)] as any;
					}
				} else if (inst.requestType === 'loadNewTicker') {
					if (inst.includeLastBar == false && !$streamInfo.replayActive) {
						newCandleData = newCandleData.slice(0, newCandleData.length - 1);
						newVolumeData = newVolumeData.slice(0, newVolumeData.length - 1);
					}
					releaseFast();
					releaseQuote();
					/*privateRequest<number>('getMarketCap', { ticker: inst.ticker }).then(
						(res: { marketCap: number }) => {
							hoveredCandleData.update((v: typeof defaultHoveredCandleData) => {
								v.mcap = res.marketCap;
								return v;
							});
						}
					);*/
					drawingMenuProps.update((v) => ({
						...v,
						chartCandleSeries: chartCandleSeries,
						securityId: Number(inst.securityId)
					}));
					for (const line of $drawingMenuProps.horizontalLines) {
						chartCandleSeries.removePriceLine(line.line);
					}
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

				// Check if we reach end of avaliable data
				if (inst.timestamp == 0) {
					chartLatestDataReached = true;
				}
				if (barDataList.length < (inst.bars ?? 0)) {
					if (inst.direction == 'backward') {
						chartEarliestDataReached = response.isEarliestData;
					} else if (inst.direction == 'forward') {
						('chartLatestDataReached');
						chartLatestDataReached = true;
					}
				}
				queuedLoad = () => {
					// Add SEC filings request when loading new ticker
					try {
						const bars = chartCandleSeries.data();
						if (bars.length > 0) {
							const firstBar = bars[0];
							const lastBar = bars[bars.length - 1];

							const fromTime = ESTSecondstoUTCMillis(firstBar.time as UTCTimestamp) as number;
							const toTime = ESTSecondstoUTCMillis(lastBar.time as UTCTimestamp) as number;

							privateRequest<any[]>('getChartEvents', {
								securityId: inst.securityId,
								from: fromTime,
								to: toTime,
								includeSECFilings: get(settings).showFilings
							}).then((events) => {
								// Check if events is empty or undefined
								if (!events || events.length === 0) {
									// Clear any existing events data
									eventSeries.setData([]);
									return; // Exit early
								}

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

								events.forEach((event) => {
									// Convert timestamp from UTC milliseconds to EST seconds
									event.timestamp = UTCSecondstoESTSeconds(event.timestamp / 1000) as UTCTimestamp;

									// Round to the nearest timeframe
									const roundedTime =
										Math.floor(event.timestamp / chartTimeframeInSeconds) * chartTimeframeInSeconds;

									if (!eventsByTime.has(roundedTime)) {
										eventsByTime.set(roundedTime, []);
									}

									// Parse the JSON string into an object
									let valueObj: EventValue = {};
									try {
										valueObj = JSON.parse(event.value);
									} catch (e) {
										console.error('Failed to parse event value:', e, event.value);
									}

									// Create proper event object based on type
									if (event.type === 'sec_filing') {
										eventsByTime.get(roundedTime)?.push({
											type: 'sec_filing',
											title: valueObj.type || 'SEC Filing',
											url: valueObj.url
										});
									} else if (event.type === 'split') {
										eventsByTime.get(roundedTime)?.push({
											type: 'split',
											title: `Split: ${valueObj.ratio || 'unknown'}`,
											value: valueObj.ratio
										});
									} else if (event.type === 'dividend') {
										const amount = typeof valueObj.amount === 'string' ? valueObj.amount : '0.00';
										eventsByTime.get(roundedTime)?.push({
											type: 'dividend',
											title: `Dividend: $${amount}`,
											value: amount,
											exDate: valueObj.exDate || 'Unknown',
											payoutDate: valueObj.payDate || 'Unknown'
										});
									}
								});

								// Fix the event data handling
								let eventData: EventMarker[] = [];
								eventsByTime.forEach((events, time) => {
									eventData.push({
										time: time as UTCTimestamp,
										events: events
									});
								});

								// Cast the data array to any to avoid readonly issues
								eventData = adjustEventsToTradingDays(eventData, [...chartCandleSeries.data()]);

								// Set the data on the event markers series
								eventSeries.setData(eventData);
							});
						}
					} catch (error) {
						console.warn('Failed to fetch chart events:', error);
					}

					if (inst.direction == 'forward') {
						const visibleRange = chart.timeScale().getVisibleRange();
						const vrFrom = visibleRange?.from;
						const vrTo = visibleRange?.to;

						// Only set visible range if both from and to values are valid
						chartCandleSeries.setData(newCandleData);
						chartVolumeSeries.setData(newVolumeData);
						if (vrFrom && vrTo && typeof vrFrom === 'number' && typeof vrTo === 'number') {
							chart.timeScale().setVisibleRange({
								from: vrFrom,
								to: vrTo
							});
						}
					} else if (inst.direction == 'backward') {
						chartCandleSeries.setData(newCandleData);
						chartVolumeSeries.setData(newVolumeData);
						if (arrowSeries && 'trades' in inst && Array.isArray(inst.trades)) {
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
					if (inst.requestType == 'loadNewTicker') {
						chart.timeScale().resetTimeScale();
						//chart.timeScale().fitContent();
						if (currentChartInstance.timestamp === 0) {
							chart.timeScale().applyOptions({
								rightOffset: 10
							});
						} else {
							chart.timeScale().applyOptions({
								rightOffset: 0
							});
						}
						releaseFast = addStream(inst, 'all', updateLatestChartBar) as () => void;
						releaseQuote = addStream(inst, 'quote', updateLatestQuote) as () => void;
					}
					isLoadingChartData = false; // Ensure this runs after data is loaded
				};
				if (
					inst.direction == 'backward' ||
					inst.requestType == 'loadNewTicker' ||
					(inst.direction == 'forward' && !isPanning)
				) {
					queuedLoad();
					if (
						inst.requestType === 'loadNewTicker' &&
						!chartLatestDataReached &&
						!$streamInfo.replayActive
					) {
						('1');
						backendLoadChartData({
							...currentChartInstance,
							timestamp: ESTSecondstoUTCMillis(
								chartCandleSeries.data()[chartCandleSeries.data().length - 1].time as UTCTimestamp
							) as UTCTimestamp,
							bars: 150, //+ 2*Math.floor(chart.getLogicalRange.to) - chartCandleSeries.data().length,
							direction: 'forward',
							requestType: 'loadAdditionalData',
							includeLastBar: true
						});
					}
				}
			})
			.catch((error: string) => {
				console.error(error);

				isLoadingChartData = false; // Ensure this runs after data is loaded
			});
	}
	function updateLatestQuote(data: QuoteData) {
		if (!data?.bidPrice || !data?.askPrice) {
			return;
		}
		const candle = chartCandleSeries.data().at(-1);
		if (!candle) return;
		const time = candle.time;
		bidLine.setData([{ time: time, value: data.bidPrice }]);
		askLine.setData([{ time: time, value: data.askPrice }]);
	}
	// Create a horizontal line at the current crosshair position (Y-coordinate)

	function handleMouseMove(event: MouseEvent) {
		if (!chartCandleSeries || !$drawingMenuProps.isDragging || !$drawingMenuProps.selectedLine)
			return;

		const price = chartCandleSeries.coordinateToPrice(event.clientY);
		if (typeof price !== 'number' || price <= 0) return;

		// Update the line position visually
		$drawingMenuProps.selectedLine.applyOptions({
			price: price,
			title: `Price: ${price.toFixed(2)}`
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
		isPanning = true;
		if (shiftDown || get(shiftOverlay).isActive) {
			shiftOverlay.update((v: ShiftOverlay): ShiftOverlay => {
				v.isActive = !v.isActive;
				if (v.isActive) {
					v.startX = event.clientX;
					v.startY = event.clientY;
					v.width = 0;
					v.height = 0;
					v.x = v.startX;
					v.y = v.startY;
					v.startPrice = chartCandleSeries.coordinateToPrice(v.startY) || 0;
					document.addEventListener('mousemove', shiftOverlayTrack);
					document.addEventListener('mouseup', handleShiftOverlayEnd);
				} else {
					document.removeEventListener('mousemove', shiftOverlayTrack);
				}
				return v;
			});
		}
	}

	function shiftOverlayTrack(event: MouseEvent): void {
		shiftOverlay.update((v: ShiftOverlay): ShiftOverlay => {
			const god = {
				...v,
				width: Math.abs(event.clientX - v.startX),
				height: Math.abs(event.clientY - v.startY),
				x: Math.min(event.clientX, v.startX),
				y: Math.min(event.clientY, v.startY),
				currentPrice: chartCandleSeries.coordinateToPrice(event.clientY) || 0
			};
			return god;
		});
	}

	async function updateLatestChartBar(trade: TradeData) {
		// Early returns for invalid data
		if (
			!trade?.price ||
			!trade?.size ||
			!trade?.timestamp ||
			!chartCandleSeries?.data()?.length ||
			isLoadingChartData
		) {
			return;
		}
		// Check excluded conditions early
		if (trade.conditions?.some((condition) => excludedConditions.has(condition))) {
			return;
		}

		// Check extended hours early
		const isExtendedHours = extendedHours(trade.timestamp);
		if (
			isExtendedHours &&
			(!currentChartInstance.extendedHours || /^[dwm]/.test(currentChartInstance.timeframe || ''))
		) {
			return;
		}

		const dolvol = get(settings).dolvol;
		const mostRecentBar = chartCandleSeries.data().at(-1);
		if (!mostRecentBar) return;

		// Type guard for CandlestickData
		const isCandlestick = (data: any): data is CandlestickData<Time> =>
			'open' in data && 'high' in data && 'low' in data && 'close' in data;

		// Type guard for HistogramData
		const isHistogram = (data: any): data is HistogramData<Time> => 'value' in data;

		if (!isCandlestick(mostRecentBar)) return;

		currentBarTimestamp = mostRecentBar.time as number;
		const tradeTime = UTCSecondstoESTSeconds(trade.timestamp / 1000);
		const sameBar = tradeTime < currentBarTimestamp + chartTimeframeInSeconds;

		if (sameBar) {
			// Use throttled updates
			const now = Date.now();

			// Initialize pendingBarUpdate if this is the first update in this throttle window
			if (!pendingBarUpdate) {
				pendingBarUpdate = {
					time: mostRecentBar.time,
					open: mostRecentBar.open,
					high: mostRecentBar.high,
					low: mostRecentBar.low,
					close: mostRecentBar.close
				};
			}

			// Initialize pendingVolumeUpdate
			if (!pendingVolumeUpdate) {
				const lastVolume = chartVolumeSeries.data().at(-1);
				if (lastVolume && isHistogram(lastVolume)) {
					pendingVolumeUpdate = {
						time: mostRecentBar.time,
						value: lastVolume.value,
						color: mostRecentBar.close > mostRecentBar.open ? '#089981' : '#ef5350'
					};
				}
			}

			// Update pending data with new trade
			if (
				trade.size >= 100 &&
				!trade.conditions?.some((condition) => excludedConditions.has(condition))
			) {
				pendingBarUpdate.high = Math.max(pendingBarUpdate.high, trade.price);
				pendingBarUpdate.low = Math.min(pendingBarUpdate.low, trade.price);
				pendingBarUpdate.close = trade.price;
			}
			if (pendingVolumeUpdate) {
				pendingVolumeUpdate.value += trade.size;
				pendingVolumeUpdate.color =
					pendingBarUpdate.close > pendingBarUpdate.open ? '#089981' : '#ef5350';
			}

			// Apply updates only if throttle time has passed
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
			}

			return;
		}

		// Create new bar - immediate update for new bars
		// Reset throttling state for new bars
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

		// Update with new bar
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
			color: '#089981' // Default to green for new bars
		});

		// Fetch and update historical data
		try {
			const timeToRequestForUpdatingAggregate =
				ESTSecondstoUTCSeconds(mostRecentBar.time as number) * 1000;
			console.log('timeToRequestForUpdatingAggregate:', timeToRequestForUpdatingAggregate);
			const [barData] = await privateRequest<BarData[]>('getChartData', {
				securityId: chartSecurityId,
				timeframe: chartTimeframe,
				timestamp: timeToRequestForUpdatingAggregate,
				direction: 'backward',
				bars: 1,
				extendedHours: chartExtendedHours,
				isreplay: $streamInfo.replayActive
			});

			if (!barData) return;

			// Find and update the matching bar
			const allCandleData = [...chartCandleSeries.data()] as Array<CandlestickData<Time>>;
			const barIndex = allCandleData.findIndex(
				(candle) => candle.time === UTCSecondstoESTSeconds(barData.time)
			);

			if (barIndex !== -1) {
				// Create safe mutable copies for data updates
				function createMutableCopy<T>(data: readonly T[]): T[] {
					return [...data];
				}

				// Update bar data with safe copies
				const updatedCandle = {
					time: UTCSecondstoESTSeconds(barData.time) as UTCTimestamp,
					open: barData.open,
					high: barData.high,
					low: barData.low,
					close: barData.close
				};

				// Create a new mutable copy of the data array before updating it
				const updatedCandleData = createMutableCopy(allCandleData);
				updatedCandleData[barIndex] = updatedCandle;
				chartCandleSeries.setData(updatedCandleData);

				// Create a new mutable copy of the volume data array before updating it
				const updatedVolumeData = createMutableCopy(chartVolumeSeries.data());
				updatedVolumeData[barIndex] = {
					time: UTCSecondstoESTSeconds(barData.time) as UTCTimestamp,
					value: barData.volume * (dolvol ? barData.close : 1),
					color: barData.close > barData.open ? '#089981' : '#ef5350'
				};
				chartVolumeSeries.setData([...updatedVolumeData] as Array<HistogramData<Time>>);
			}
		} catch (error) {
			console.error('Error fetching historical data:', error);
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

	function change(newReq: ChartQueryDispatch) {
		const securityId =
			typeof newReq.securityId === 'string'
				? parseInt(newReq.securityId, 10)
				: newReq.securityId || 0;

		const chartId =
			typeof newReq.chartId === 'string' ? parseInt(newReq.chartId, 10) : newReq.chartId;

		const updatedReq: ChartInstance = {
			ticker: newReq.ticker || currentChartInstance.ticker,
			timestamp: newReq.timestamp ?? currentChartInstance.timestamp,
			timeframe: newReq.timeframe || '1d',
			securityId: securityId,
			price: newReq.price ?? currentChartInstance.price,
			chartId: chartId,
			bars: newReq.bars ?? defaultBarsOnScreen,
			direction: newReq.direction,
			requestType: newReq.requestType,
			includeLastBar: newReq.includeLastBar,
			extendedHours: newReq.extendedHours
		};

		// Update currentChartInstance
		currentChartInstance = {
			...currentChartInstance,
			...updatedReq
		};

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

		backendLoadChartData(updatedReq);

		// Clear existing alert lines when changing tickers
		if (chartCandleSeries) {
			alertLines.forEach((line) => {
				chartCandleSeries.removePriceLine(line.line);
			});
			alertLines = [];
		}

		if (eventSeries) {
			eventSeries.setData([]);
		}
		if (arrowSeries) {
			arrowSeries.setData([]);
		}

		// Reset session highlighting when changing securities
		if (sessionHighlighting) {
			chartCandleSeries.detachPrimitive(sessionHighlighting);
			sessionHighlighting = new SessionHighlighting(createDefaultSessionHighlighter());
			chartCandleSeries.attachPrimitive(sessionHighlighting);
		}
	}

	onMount(() => {
		const chartOptions = {
			autoSize: true,
			crosshair: {
				mode: CrosshairMode.Normal
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
					visible: false
				},
				horzLines: {
					visible: false
				}
			},
			timeScale: {
				timeVisible: true
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
				timestamp: currentChartInstance.timestamp,
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
		chartContainer.addEventListener('mousedown', handleMouseDown);
		chartContainer.addEventListener('mouseup', () => {
			isPanning = false;
			if (queuedLoad != null) {
				queuedLoad();
			}
		});
		chartContainer.addEventListener('keydown', (event) => {
			setActiveChart(chartId, currentChartInstance);
			if (event.key == 'r' && event.altKey) {
				// alt + r reset view
				if (currentChartInstance.timestamp && !$streamInfo.replayActive) {
					queryChart({ timestamp: 0 });
				} else {
					chart.timeScale().resetTimeScale();
				}

				// IMPORTANT: Prevent fall-through so Alt+R doesn't open the input window
				event.stopPropagation();
				event.preventDefault();
				return;
			} else if (event.key == 'h' && event.altKey) {
				const price = chartCandleSeries.coordinateToPrice(latestCrosshairPositionY);
				if (typeof price !== 'number') return;

				const roundedPrice = Math.round(price * 100) / 100;
				const securityId = currentChartInstance.securityId;
				addHorizontalLine(roundedPrice, securityId);
			} else if (event.key == 's' && event.altKey) {
				handleScreenshot(chartId.toString());
			} else if (
				event.key === 'Tab' ||
				(!event.ctrlKey &&
					!event.altKey &&
					!event.metaKey &&
					/^[a-zA-Z0-9]$/.test(event.key.toLowerCase()))
			) {
				// goes to input popup
				if ($streamInfo.replayActive) {
					currentChartInstance.timestamp = 0;
				}

				// Store the first keystroke in the instance to pass it to the input component
				// Don't lowercase - pass the original key case to allow input component to handle it
				const firstKey = event.key === 'Tab' ? '' : event.key;

				// Prevent default and stop propagation immediately
				event.preventDefault();
				event.stopPropagation();

				// Pass the first key as inputString to the query input
				handleKeyboardInput(event, chartContainer);
			} else if (event.key == 'Shift') {
				shiftDown = true;
			} else if (event.key == 'Escape') {
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
			lastValueVisible: true,
			priceLineVisible: false,
			priceFormat: { type: 'volume' },
			priceScaleId: ''
		});
		chartVolumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0 } });
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
			if (!chartCandleSeries.data().length || !param.point || !currentChartInstance.securityId) {
				return;
			}
			const volumeData = param.seriesData.get(chartVolumeSeries);
			const volume =
				volumeData && 'value' in volumeData ? (volumeData as HistogramData<Time>).value : 0;
			const allCandleData = [...chartCandleSeries.data()] as Array<CandlestickData<Time>>;
			const validCrosshairPoint = !(
				param === undefined ||
				param.time === undefined ||
				param.point.x < 0 ||
				param.point.y < 0
			);
			let bar;
			let cursorBarIndex: number | undefined;
			if (!validCrosshairPoint) {
				if (param?.logical !== undefined && param.logical < 0) {
					bar = allCandleData[0];
				}
			} else {
				bar = param.seriesData.get(chartCandleSeries);
				if (!bar) {
					return;
				}
			}

			// Type guard to check if bar is CandlestickData

			const isCandlestick = (data: any): data is CandlestickData<Time> => {
				if (!data) {
					return false;
				}
				return 'open' in data && 'high' in data && 'low' in data && 'close' in data;
			};

			if (!isCandlestick(bar)) {
				return; // Skip if the bar is not CandlestickData
			}

			// Get cursor bar index if it wasn't set in the validCrosshairPoint block
			if (validCrosshairPoint && cursorBarIndex === undefined) {
				const cursorTime = bar.time as number;
				cursorBarIndex = allCandleData.findIndex((candle) => candle.time === cursorTime);
			}

			// Ensure cursorBarIndex is defined before using it
			if (cursorBarIndex === undefined) return;

			let barsForADR;
			if (cursorBarIndex >= 20) {
				barsForADR = allCandleData.slice(cursorBarIndex - 19, cursorBarIndex + 1);
			} else {
				barsForADR = allCandleData.slice(0, cursorBarIndex + 1);
			}
			let chg = 0;
			let chgprct = 0;
			if (cursorBarIndex > 0) {
				const prevBar = allCandleData[cursorBarIndex - 1];

				if (isCandlestick(prevBar)) {
					chg = bar.close - prevBar.close;
					chgprct = (bar.close / prevBar.close - 1) * 100;
				}
			}

			hoveredCandleData.set({
				open: bar.open,
				high: bar.high,
				low: bar.low,
				close: bar.close,
				volume: volume,
				adr: calculateSingleADR(
					barsForADR.filter(
						(candle) => 'open' in candle && 'high' in candle && 'low' in candle && 'close' in candle
					) as CandlestickData<Time>[]
				),
				chg: chg,
				chgprct: chgprct,
				rvol: 0
			});
			if (currentChartInstance.timeframe && /^\d+$/.test(currentChartInstance.timeframe)) {
				let barsForRVOL;
				if (cursorBarIndex !== undefined && cursorBarIndex >= 1000) {
					barsForADR = allCandleData.slice(cursorBarIndex - 1000, cursorBarIndex + 1);
				} else if (cursorBarIndex !== undefined) {
					// Transform the histogram data to the format expected by calculateRVOL
					const volumeData = chartVolumeSeries.data().slice(0, cursorBarIndex + 1);
					barsForRVOL = volumeData
						.filter((bar) => 'value' in bar) // Filter to ensure only HistogramData is included
						.map((bar) => ({
							time: bar.time as UTCTimestamp,
							value: (bar as HistogramData<Time>).value || 0
						}));
				}
				// Only call calculateRVOL if barsForRVOL is defined
				if (barsForRVOL && barsForRVOL.length > 0) {
					calculateRVOL(barsForRVOL, currentChartInstance.securityId).then((r: any) => {
						hoveredCandleData.update((v) => {
							v.rvol = r;
							return v;
						});
					});
				}
			}
			latestCrosshairPositionTime = bar.time as number;
			latestCrosshairPositionY = param.point.y as number; //inccorect
		});
		chart.timeScale().subscribeVisibleLogicalRangeChange((logicalRange) => {
			if (!logicalRange || Date.now() - lastChartQueryDispatchTime < chartRequestThrottleDuration) {
				return;
			}
			const barsOnScreen = Math.floor(logicalRange.to) - Math.ceil(logicalRange.from);
			const bufferInScreenSizes = 0.7;
			if (logicalRange.from / barsOnScreen < bufferInScreenSizes) {
				if (chartEarliestDataReached) {
					return;
				}
				('2');
				const inst: CoreInstance = {
					ticker: currentChartInstance.ticker,
					timestamp: currentChartInstance.timestamp,
					timeframe: currentChartInstance.timeframe,
					securityId: currentChartInstance.securityId,
					price: currentChartInstance.price
				};
				backendLoadChartData({
					...inst,
					bars: Math.floor(bufferInScreenSizes * barsOnScreen) + 100,
					direction: 'backward',
					requestType: 'loadAdditionalData',
					includeLastBar: true
				});
			} else if (
				(chartCandleSeries.data().length - logicalRange.to) / barsOnScreen <
				bufferInScreenSizes
			) {
				// forward loa
				if (chartLatestDataReached) {
					return;
				}
				if ($streamInfo.replayActive) {
					return;
				}
				('3');
				const inst: CoreInstance = {
					ticker: currentChartInstance.ticker,
					timestamp: currentChartInstance.timestamp,
					timeframe: currentChartInstance.timeframe,
					securityId: currentChartInstance.securityId,
					price: currentChartInstance.price
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

		return () => {
			// Apply any pending updates before unmounting
			if (pendingBarUpdate) {
				chartCandleSeries.update(pendingBarUpdate);
			}

			if (pendingVolumeUpdate) {
				chartVolumeSeries.update(pendingVolumeUpdate);
			}

			// ... any other cleanup code ...
		};
	});

	// Handle event marker clicks
	function handleEventClick(events: EventMarker['events'], x: number, y: number) {
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
			selectedEvent = { events, x, y };
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

	function handleShiftOverlayEnd(event: MouseEvent) {
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
		document.removeEventListener('mousemove', shiftOverlayTrack);
		document.removeEventListener('mouseup', handleShiftOverlayEnd);
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

	// Handle Instance type
	const createInstance = (chartInstance: Partial<CoreInstance>): CoreInstance => ({
		ticker: chartInstance.ticker || '',
		timestamp: chartInstance.timestamp || 0,
		timeframe: chartInstance.timeframe || '',
		securityId:
			typeof chartInstance.securityId === 'string'
				? parseInt(chartInstance.securityId, 10)
				: chartInstance.securityId || 0,
		price: chartInstance.price || 0
	});

	// Use in the code where Instance is needed
	const instance = createInstance(currentChartInstance);

	// Handle bars calculation safely
	const calculateBars = (instance: Partial<ChartInstance>): number => {
		return instance.bars || Math.floor(bufferInScreenSizes * defaultBarsOnScreen) + 100;
	};

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

	// Update the queryInstanceInput call
	function handleKeyboardInput(event: KeyboardEvent, chartContainer: HTMLElement) {
		if ($streamInfo.replayActive) {
			currentChartInstance.timestamp = 0;
		}

		const firstKey = event.key === 'Tab' ? '' : event.key;
		event.preventDefault();
		event.stopPropagation();

		const inputInstance: ChartInstance = {
			ticker: currentChartInstance.ticker,
			timestamp: currentChartInstance.timestamp,
			timeframe: currentChartInstance.timeframe,
			securityId: ensureNumericSecurityId(currentChartInstance),
			price: currentChartInstance.price,
			chartId: currentChartInstance.chartId,
			inputString: firstKey
		};

		queryInstanceInput('any', 'any', inputInstance)
			.then((value: CoreInstance) => {
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
					extendedHours: currentChartInstance.extendedHours
				};
				currentChartInstance = extendedV;
				queryChart(extendedV, true);
				setTimeout(() => chartContainer.focus(), 0);
			})
			.catch((error) => {
				console.error('Input error:', error);
				setTimeout(() => chartContainer.focus(), 0);
			});
	}
</script>

<div class="chart" id="chart_container-{chartId}" style="width: {width}px" tabindex="-1">
	<Legend instance={currentChartInstance} {hoveredCandleData} {width} />
	<Shift {shiftOverlay} />
	<DrawingMenu {drawingMenuProps} />
</div>

<!-- Replace the filing info overlay with a more generic event info overlay -->
{#if selectedEvent}
	<div
		class="event-info"
		style="
            left: {selectedEvent.x - 100}px; 
            top: {selectedEvent.y - (80 + selectedEvent.events.length * 40)}px"
	>
		<div class="event-header">
			{#if selectedEvent.events[0]?.type === 'sec_filing'}
				<div class="event-icon" style="color: #9C27B0;">📄</div>
				<div class="event-title">SEC Filings</div>
			{:else if selectedEvent.events[0]?.type === 'split'}
				<div class="event-icon" style="color: #FFD700;">📊</div>
				<div class="event-title">Stock Splits</div>
			{:else if selectedEvent.events[0]?.type === 'dividend'}
				<div class="event-icon" style="color: #2196F3;">💰</div>
				<div class="event-title">Dividends</div>
			{:else}
				<div class="event-icon">📅</div>
				<div class="event-title">Events</div>
			{/if}
			<button class="close-button" on:click={closeEventPopup}>×</button>
		</div>
		<div class="event-content">
			{#each selectedEvent.events as event}
				{#if event.type === 'sec_filing'}
					<a
						href={event.url}
						target="_blank"
						rel="noopener noreferrer"
						class="event-row filing-link"
					>
						<span class="event-type">{event.title}</span>
						<span class="event-link">View →</span>
					</a>
				{:else if event.type === 'split'}
					<div class="event-row">
						<span class="event-type">{event.title}</span>
					</div>
				{:else if event.type === 'dividend'}
					<div class="event-row dividend-row">
						<div class="dividend-details">
							<span class="event-type">{event.title}</span>
							{#if event.exDate}
								<span class="dividend-date">Ex-Date: {event.exDate}</span>
							{/if}
							{#if event.payoutDate}
								<span class="dividend-date">Payout: {event.payoutDate}</span>
							{/if}
						</div>
					</div>
				{:else}
					<div class="event-row">
						<span class="event-type">{event.title}</span>
					</div>
				{/if}
			{/each}
		</div>
	</div>
{/if}

<style>
	.event-info {
		position: absolute;
		background: #1e1e1e;
		border: 1px solid #333;
		border-radius: 4px;
		padding: 8px;
		z-index: 1000;
		width: 220px; /* Increased width to accommodate more content */
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
		transform: translateX(0); /* Center popup above marker */
	}

	.event-header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding-bottom: 8px;
		border-bottom: 1px solid #333;
		margin-bottom: 8px;
		position: relative;
	}

	.close-button {
		position: absolute;
		right: 0;
		top: 0;
		background: none;
		border: none;
		color: #888;
		font-size: 18px;
		cursor: pointer;
		padding: 0 4px;
	}

	.close-button:hover {
		color: #fff;
	}

	.event-icon {
		font-size: 1.2rem;
	}

	.event-title {
		font-weight: bold;
	}

	.event-content {
		max-height: 150px;
		overflow-y: auto;
	}

	.event-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 4px 0;
		border-bottom: 1px solid #333;
		color: #fff;
		text-decoration: none;
	}

	.event-row:last-child {
		border-bottom: none;
	}

	.event-type {
		font-size: 0.9rem;
	}

	.event-link {
		color: #4caf50;
		font-size: 0.8rem;
	}

	.filing-link {
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.filing-link:hover {
		background-color: #333;
	}

	.dividend-row {
		flex-direction: column;
		align-items: flex-start;
	}

	.dividend-details {
		display: flex;
		flex-direction: column;
		width: 100%;
	}

	.dividend-date {
		font-size: 0.8rem;
		color: #bbb;
		margin-top: 2px;
	}
</style>
