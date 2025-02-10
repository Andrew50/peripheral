<!-- chart.svelte-->
<script lang="ts">
	import Legend from './legend.svelte';
	import Shift from './shift.svelte';
	import Countdown from './countdown.svelte';
	import DrawingMenu from './drawingMenu.svelte';
	import { privateRequest } from '$lib/core/backend';
	import type { Instance, TradeData, QuoteData } from '$lib/core/types';
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
		HistogramSeriesOptions
	} from 'lightweight-charts';
	import { calculateRVOL, calculateSMA, calculateSingleADR, calculateVWAP } from './indicators';
	import type { Writable } from 'svelte/store';
	import { writable, get } from 'svelte/store';
	import { onMount } from 'svelte';
	import {
		UTCSecondstoESTSeconds,
		ESTSecondstoUTCSeconds,
		ESTSecondstoUTCMillis,
		getReferenceStartTimeForDateMilliseconds,
		timeframeToSeconds,
		getRealTimeTime
	} from '$lib/core/timestamp';
	import { addStream } from '$lib/utils/stream/interface';
	import { ArrowMarkersPaneView, type ArrowMarker } from './arrowMarkers';
	let bidLine: any;
	let askLine: any;
	let currentBarTimestamp: number;
	interface DrawingMenuProps {
		chartCandleSeries: ISeriesApi<'Candlestick'>;
		selectedLine: IPriceLine | null;
		clientX: number;
		clientY: number;
		active: boolean;
		horizontalLines: { price: number; line: IPriceLine; id: number }[];
		isDragging: boolean;
	}
	let drawingMenuProps: Writable<DrawingMenuProps> = writable({
		chartCandleSeries: null,
		selectedLine: null,
		clientX: 0,
		clientY: 0,
		active: false,
		selectedLineId: -1,
		horizontalLines: [],
		isDragging: false
	});

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
	const defaultHoveredCandleData = {
		rvol: 0,
		open: 0,
		high: 0,
		low: 0,
		close: 0,
		volume: 0,
		adr: 0,
		chg: 0,
		chgprct: 0,
		mcap: 0
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
	let currentChartInstance: Instance = { ticker: '', timestamp: 0, timeframe: '' };
	let blockingChartQueryDispatch = {};
	let isPanning = false;
	const excludedConditions = new Set([2, 7, 10, 13, 15, 16, 20, 21, 22, 29, 33, 37]);
	let mouseDownStartX = 0;
	let mouseDownStartY = 0;
	const DRAG_THRESHOLD = 3; // pixels of movement before considered a drag

	// Add new interface for alert lines
	interface AlertLine {
		price: number;
		line: IPriceLine;
		alertId: number;
	}

	// Add new property to track alert lines
	let alertLines: AlertLine[] = [];

	let arrowSeries: any = null;  // Initialize as null

	function extendedHours(timestamp: number): boolean {
		const date = new Date(timestamp);
		const hours = date.getHours();
		const minutes = date.getMinutes();
		const timeInMinutes = hours * 60 + minutes;

		// Regular market hours are 9:30 AM - 4:00 PM EST
		const marketOpenMinutes = 9 * 60 + 30; // 9:30 AM
		const marketCloseMinutes = 16 * 60; // 4:00 PM

		return timeInMinutes < marketOpenMinutes || timeInMinutes >= marketCloseMinutes;
	}

	function backendLoadChartData(inst: ChartQueryDispatch): void {
		if (inst.requestType === 'loadNewTicker') {
			bidLine.setData([]);
			askLine.setData([]);
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
			console.log('adjusting to stream timestamp');
			inst.timestamp = Math.floor($streamInfo.timestamp);
		}
		console.log(inst);
		console.log(inst.extendedHours);
		privateRequest<BarData[]>('getChartData', {
			securityId: inst.securityId,
			timeframe: inst.timeframe,
			timestamp: inst.timestamp,
			direction: inst.direction,
			bars: inst.bars,
			extendedhours: inst.extendedHours,
			isreplay: $streamInfo.replayActive
		})
			.then((barDataList: BarData[]) => {
				blockingChartQueryDispatch = inst;
				if (!(Array.isArray(barDataList) && barDataList.length > 0)) {
					return;
				}
				console.log(barDataList);
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
					for (const line of $drawingMenuProps.horizontalLines) {
						chartCandleSeries.removePriceLine(line.line);
					}
					privateRequest<HorizontalLine[]>('getHorizontalLines', {
						securityId: inst.securityId
					}).then((res: HorizontalLine[]) => {
						if (res !== null && res.length > 0) {
							for (const line of res) {
								//night need to be later
								addHorizontalLine(line.price, line.id); //TO IMPLEMENT
							}
						}
					});
				}

				// Check if we reach end of avaliable data
				if (inst.timestamp == 0) {
					chartLatestDataReached = true;
				}
				if (barDataList.length < inst.bars) {
					if (inst.direction == 'backward') {
						chartEarliestDataReached = true;
					} else if (inst.direction == 'forward') {
						chartLatestDataReached = true;
					}
				}
				queuedLoad = () => {
					if (inst.direction == 'forward') {
						const visibleRange = chart.timeScale().getVisibleRange();
						const vrFrom = visibleRange?.from as Time;
						const vrTo = visibleRange?.to as Time;
						chartCandleSeries.setData(newCandleData);
						chartVolumeSeries.setData(newVolumeData);
						chart.timeScale().setVisibleRange({ from: vrFrom, to: vrTo });
					} else if (inst.direction == 'backward') {
						chartCandleSeries.setData(newCandleData);
						chartVolumeSeries.setData(newVolumeData);
						
						// Add null check before using arrowSeries
						if (arrowSeries && 'entries' in inst || 'exits' in inst) {
							const markers: ArrowMarker[] = [];
							
							// Add entry markers
							if ('entries' in inst) {
								inst.entries.forEach(entry => {
									const entryTime = UTCSecondstoESTSeconds(entry.time / 1000);
									const roundedTime = Math.floor(entryTime / chartTimeframeInSeconds) * chartTimeframeInSeconds;
									markers.push({
										time: roundedTime as UTCTimestamp,
										price: entry.price,
										type: 'entry'
									});
								});
							}
							
							// Add exit markers
							if ('exits' in inst) {
								inst.exits.forEach(exit => {
									const exitTime = UTCSecondstoESTSeconds(exit.time / 1000);
									const roundedTime = Math.floor(exitTime / chartTimeframeInSeconds) * chartTimeframeInSeconds;
									markers.push({
										time: roundedTime as UTCTimestamp,
										price: exit.price,
										type: 'exit'
									});
								});
							}
							console.log(markers);
						
							arrowSeries.setData(markers);
						}
					}
					queuedLoad = null;
					sma10Series.setData(calculateSMA(newCandleData, 10));
					sma20Series.setData(calculateSMA(newCandleData, 20));
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
						releaseFast = addStream(inst, 'fast', updateLatestChartBar);
						releaseQuote = addStream(inst, 'quote', updateLatestQuote);
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
						console.log('1');
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
		const candle = chartCandleSeries.data()[chartCandleSeries.data().length - 1];
		if (!candle) return;
		const time = candle.time;
		bidLine.setData([{ time: time, value: data.bidPrice }]);
		askLine.setData([{ time: time, value: data.askPrice }]);
	}
	// Create a horizontal line at the current crosshair position (Y-coordinate)
	function addHorizontalLine(price: number, id: number = -1) {
		const priceLine = chartCandleSeries.createPriceLine({
			price: price,
			color: 'white',
			lineWidth: 1,
			lineStyle: 0, // Solid line
			axisLabelVisible: false,
			title: `Price: ${price}`
		});
		$drawingMenuProps.horizontalLines.push({
			id,
			price,
			line: priceLine
		});
		if (id == -1) {
			// only add to baceknd if its being added not from a ticker load but from a new added line
			privateRequest<number>('setHorizontalLine', {
				price: price,
				securityId: chartSecurityId
			}).then((res: number) => {
				$drawingMenuProps.horizontalLines[$drawingMenuProps.horizontalLines.length - 1].id = res;
			});
		}
	}
	function handleMouseMove(event: MouseEvent) {
		if (!$drawingMenuProps.isDragging || !$drawingMenuProps.selectedLine) return;

		const newPrice = chartCandleSeries.coordinateToPrice(event.clientY) || 0;
		if (newPrice <= 0) return;

		// Update the line position visually
		$drawingMenuProps.selectedLine.applyOptions({
			price: newPrice
		});

		// Update the stored price in horizontalLines array
		const lineIndex = $drawingMenuProps.horizontalLines.findIndex(
			(line) => line.line === $drawingMenuProps.selectedLine
		);
		if (lineIndex !== -1) {
			$drawingMenuProps.horizontalLines[lineIndex].price = newPrice;
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
		console.log(upperPrice, lowerPrice);

		if (upperPrice == 0 || lowerPrice == 0) return false;

		// Only check regular horizontal lines, not alert lines
		for (const line of $drawingMenuProps.horizontalLines) {
			if (line.price <= upperPrice && line.price >= lowerPrice) {
				drawingMenuProps.update((v: DrawingMenuProps) => ({
					...v,
					chartCandleSeries: chartCandleSeries,
					selectedLine: line.line,
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
		console.log('handleMouseDown');
		if (determineClickedLine(event)) {
			console.log('determineClickedLine');
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
				console.log(deltaX, deltaY);

				if (deltaX <= DRAG_THRESHOLD && deltaY <= DRAG_THRESHOLD) {
					console.log('click');
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
			shiftOverlay.update((v: ShiftOverlay) => {
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
				} else {
					document.removeEventListener('mousemove', shiftOverlayTrack);
				}
				return v;
			});
		}
	}

	function shiftOverlayTrack(event: MouseEvent): void {
		shiftOverlay.update((v: ShiftOverlay) => {
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
			(!currentChartInstance.extendedHours || /^[dwm]/.test(currentChartInstance.timeframe))
		) {
			return;
		}

		const dolvol = get(settings).dolvol;
		const mostRecentBar = chartCandleSeries.data().at(-1);
		if (!mostRecentBar) return;

		currentBarTimestamp = mostRecentBar.time as number;
		const tradeTime = UTCSecondstoESTSeconds(trade.timestamp / 1000);
		const sameBar = tradeTime < currentBarTimestamp + chartTimeframeInSeconds;

		if (sameBar) {
			// Update existing bar
			if (trade.size >= 100) {
				if (!trade.conditions?.some((condition) => excludedConditions.has(condition))) {
					chartCandleSeries.update({
						time: mostRecentBar.time,
						open: mostRecentBar.open,
						high: Math.max(mostRecentBar.high, trade.price),
						low: Math.min(mostRecentBar.low, trade.price),
						close: trade.price
					});
				}
			}

			const lastVolume = chartVolumeSeries.data().at(-1);
			if (lastVolume) {
				chartVolumeSeries.update({
					time: mostRecentBar.time,
					value: lastVolume.value + trade.size,
					color: mostRecentBar.close > mostRecentBar.open ? '#089981' : '#ef5350'
				});
			}
			return;
		}

		// Create new bar
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
			const currentData = chartCandleSeries.data();
			const barIndex = currentData.findIndex(
				(candle) => candle.time === UTCSecondstoESTSeconds(barData.time)
			);

			if (barIndex !== -1) {
				const updatedCandle = {
					time: UTCSecondstoESTSeconds(barData.time) as UTCTimestamp,
					open: barData.open,
					high: barData.high,
					low: barData.low,
					close: barData.close
				};
				currentData[barIndex] = updatedCandle;
				chartCandleSeries.setData(currentData);

				const volumeData = chartVolumeSeries.data();
				volumeData[barIndex] = {
					time: UTCSecondstoESTSeconds(barData.time) as UTCTimestamp,
					value: barData.volume * (dolvol ? barData.close : 1),
					color: barData.close > barData.open ? '#089981' : '#ef5350'
				};
				chartVolumeSeries.setData(volumeData);
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

				alertLines.push({
					price: alert.alertPrice,
					line: priceLine,
					alertId: alert.alertId
				});
			}
		});
	}

	function change(newReq: ChartQueryDispatch) {
		const req = { ...currentChartInstance, ...newReq };
		if (chartId !== req.chartId) {
			return;
		}
		if (!req.timeframe) {
			req.timeframe = '1d';
		}
		if (!req.securityId || !req.ticker || !req.timeframe) {
			return;
		}
		hoveredCandleData.set(defaultHoveredCandleData);
		chartEarliestDataReached = false;
		chartLatestDataReached = false;
		chartSecurityId = req.securityId;
		chartTimeframe = req.timeframe;
		currentChartInstance = { ...req };
		chartTimeframeInSeconds = timeframeToSeconds(
			req.timeframe,
			(req.timestamp == 0 ? Date.now() : req.timestamp) as number
		);
		chartExtendedHours = req.extendedHours ?? false;
		if (
			req.timeframe?.includes('m') ||
			req.timeframe?.includes('w') ||
			req.timeframe?.includes('d') ||
			req.timeframe?.includes('q')
		) {
			chart.applyOptions({ timeScale: { timeVisible: false } });
		} else {
			chart.applyOptions({ timeScale: { timeVisible: true } });
		}
		backendLoadChartData(req);

		// Clear existing alert lines when changing tickers
		alertLines.forEach((line) => {
			chartCandleSeries.removePriceLine(line.line);
		});
		alertLines = [];

		if(arrowSeries) {
			arrowSeries.setData([]);
		}
	}

	chartQueryDispatcher.subscribe((req: ChartQueryDispatch) => {
		change(req);
	});
	chartEventDispatcher.subscribe((e: ChartEventDispatch) => {
		if (!currentChartInstance || !currentChartInstance.securityId) return;
		if (e.event == 'replay') {
			//currentChartInstance.timestamp = $streamInfo.timestamp
			currentChartInstance.timestamp = 0;
			const req: ChartQueryDispatch = {
				...currentChartInstance,
				bars: 400,
				direction: 'backward',
				requestType: 'loadNewTicker',
				includeLastBar: false,
				chartId: chartId
			};
			console.log(req);
			change(req);
		} else if (e.event == 'addHorizontalLine') {
			addHorizontalLine(e.data);
		}
	});

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
			const timestamp = ESTSecondstoUTCMillis(latestCrosshairPositionTime);
			const price = Math.round(chartCandleSeries.coordinateToPrice(event.clientY) * 100) / 100 || 0;
			const ins: Instance = { ...currentChartInstance, timestamp: timestamp, price: price };
			queryInstanceRightClick(event, ins, 'chart');
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
				//alt + r reset view
				if (currentChartInstance.timestamp && !$streamInfo.replayActive) {
					queryChart({ timestamp: 0 });
				} else {
					chart.timeScale().resetTimeScale();
				}
			} else if (event.key == 'h' && event.altKey) {
				addHorizontalLine(chartCandleSeries.coordinateToPrice(latestCrosshairPositionY) || 0);
			} else if (event.key == 'Tab' || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
				// goes to input popup
				if ($streamInfo.replayActive) {
					currentChartInstance.timestamp = 0;
				}
				queryInstanceInput('any', 'any', currentChartInstance)
					.then((v: Instance) => {
						currentChartInstance = v;
						queryChart(v, true);
					})
					.catch();
			} else if (event.key == 'Shift') {
				shiftDown = true;
			} else if (event.key == 'Escape') {
				if (get(shiftOverlay).isActive) {
					shiftOverlay.update((v: ShiftOverlay) => {
						if (v.isActive) {
							v.isActive = false;
							return {
								...v,
								isActive: false
							};
						}
					});
				}
			}
		});
		chart = createChart(chartContainer, chartOptions);
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
		const smaOptions = {
			lineWidth: 1,
			priceLineVisible: false,
			lastValueVisible: false
		} as DeepPartial<LineWidth>;
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
		arrowSeries = chart.addCustomSeries<ArrowMarker, CustomSeriesOptions>(new ArrowMarkersPaneView(),{});

		chart.subscribeCrosshairMove((param) => {
			if (!chartCandleSeries.data().length || !param.point || !currentChartInstance.securityId) {
				return;
			}
			const volumeData = param.seriesData.get(chartVolumeSeries);
			const volume = volumeData ? volumeData.value : 0;
			const allCandleData = chartCandleSeries.data();
			const validCrosshairPoint = !(
				param === undefined ||
				param.time === undefined ||
				param.point.x < 0 ||
				param.point.y < 0
			);
			let bar;
			let cursorBarIndex;
			if (!validCrosshairPoint) {
				if (param.logical < 0) {
					bar = allCandleData[0];
					cursorBarIndex = 0;
				} else {
					cursorBarIndex = allCandleData.length - 1;
					bar = allCandleData[cursorBarIndex];
				}
			} else {
				bar = param.seriesData.get(chartCandleSeries);
				if (!bar) {
					return;
				}
				const cursorTime = bar.time as number;
				cursorBarIndex = allCandleData.findIndex((candle) => candle.time === cursorTime);
			}
			let barsForADR;
			if (cursorBarIndex >= 20) {
				barsForADR = allCandleData.slice(cursorBarIndex - 19, cursorBarIndex + 1);
			} else {
				barsForADR = allCandleData.slice(0, cursorBarIndex + 1);
			}
			let chg = 0;
			let chgprct = 0;
			if (cursorBarIndex > 0) {
				chg = bar.close - allCandleData[cursorBarIndex - 1].close;
				chgprct = (bar.close / allCandleData[cursorBarIndex - 1].close - 1) * 100;
			}
			const mcap = $hoveredCandleData.mcap;
			hoveredCandleData.set({
				open: bar.open,
				high: bar.high,
				low: bar.low,
				close: bar.close,
				volume: volume,
				adr: calculateSingleADR(barsForADR),
				chg: chg,
				chgprct: chgprct,
				rvol: 0,
				mcap: mcap
			});
			if (/^\d+$/.test(currentChartInstance.timeframe)) {
				let barsForRVOL;
				if (cursorBarIndex >= 1000) {
					barsForADR = allCandleData.slice(cursorBarIndex - 1000, cursorBarIndex + 1);
				} else {
					barsForRVOL = chartVolumeSeries.data().slice(0, cursorBarIndex + 1);
				}
				calculateRVOL(barsForRVOL, currentChartInstance.securityId).then((r: any) => {
					hoveredCandleData.update((v) => {
						v.rvol = r;
						return v;
					});
				});
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
				console.log('2');
				backendLoadChartData({
					...currentChartInstance,
					timestamp: ESTSecondstoUTCMillis(
						chartCandleSeries.data()[0].time as UTCTimestamp
					) as number,
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
				console.log('3');
				backendLoadChartData({
					...currentChartInstance,
					timestamp: ESTSecondstoUTCMillis(
						chartCandleSeries.data()[chartCandleSeries.data().length - 1].time as UTCTimestamp
					) as UTCTimestamp,
					bars: Math.floor(bufferInScreenSizes * barsOnScreen) + 100,
					direction: 'forward',
					requestType: 'loadAdditionalData',
					includeLastBar: true
				});
			}
		});
	});
</script>

<div autofocus class="chart" id="chart_container-{chartId}" style="width: {width}px" tabindex="-1">
	<Legend instance={currentChartInstance} {hoveredCandleData} />
	<Shift {shiftOverlay} />
	<Countdown instance={currentChartInstance} {currentBarTimestamp} />
	<DrawingMenu {drawingMenuProps} />
</div>

<style>
	.chart {
		position: relative;
	}
</style>
