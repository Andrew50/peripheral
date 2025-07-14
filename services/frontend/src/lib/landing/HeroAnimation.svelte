<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount, onDestroy } from 'svelte';
	// Pricing preload removed - now handled directly in pricing page
	import { createChart } from 'lightweight-charts';
	import { fade } from 'svelte/transition';
	import type { TimelineEvent } from '$lib/landing/timeline';
	import { createTimelineEvents, sampleQuery, totalScroll } from '$lib/landing/timeline';
	import { timelineProgress } from '$lib/landing/timeline';
	import { get } from 'svelte/store';
	import HeroPlotChunk from '$lib/landing/HeroPlotChunk.svelte';
	import { isPlotData, getPlotData, generatePlotKey } from '$lib/features/chat/plotUtils';
	// Import chat utils for markdown parsing and ticker button functionality
	import { parseMarkdown, handleTickerButtonClick } from '$lib/features/chat/utils';
	// Import table-related types and functionality from chat interface
	import type { SortState } from '$lib/features/chat/interface';
	import type {
		ISeriesApi,
		Time,
		CandlestickData,
		CandlestickSeriesOptions,
		CandlestickStyleOptions,
		SeriesOptionsCommon,
		WhitespaceData,
		DeepPartial,
		HistogramData,
		HistogramSeriesOptions,
		HistogramStyleOptions,
		LineData,
		LineSeriesOptions,
		LineStyleOptions
	} from 'lightweight-charts';
	// ---------------------------------------------
	// Chat message structures (mirrors main chat)
	// ---------------------------------------------
	type TableData = { caption?: string; headers: string[]; rows: any[][] };
	type PlotData = {
		chart_type: 'line' | 'bar' | 'scatter' | 'histogram' | 'heatmap';
		data: any[];
		[key: string]: any;
	};
	type ContentChunk = { type: 'text' | 'table' | 'plot'; content: string | TableData | PlotData };

	interface ChatMessage {
		message_id: string;
		sender: 'user' | 'assistant';
		text?: string;
		contentChunks: ContentChunk[];
	}

	function isTableData(content: any): content is TableData {
		return (
			content &&
			typeof content === 'object' &&
			Array.isArray(content.headers) &&
			Array.isArray(content.rows)
		);
	}

	// Table state management (mirrors chat.svelte)
	let tableSortStates: { [key: string]: SortState } = {};
	let tablePaginationStates: { [key: string]: { currentPage: number; rowsPerPage: number } } = {};
	// Server-injected data (bars for SPY) forwarded from route
	export let defaultKey: string;
	export let chartsByKey: Record<
		string,
		{ chartData: { bars: any[] }; timeframe: string; ticker: string }
	>;

	const chartPool = chartsByKey;
	let activeKey = defaultKey;

	function setChart(keys: string[], chartType: 'candle' | 'line') {
		// Clear all series first
		chartCandleSeries?.setData([]);
		chartVolumeSeries?.setData([]);
		chartLineSeries1?.setData([]);
		chartLineSeries2?.setData([]);
		chartLineSeries3?.setData([]);

		if (chartType === 'line') {
			// For line charts, use the first 3 keys for the 3 line series
			const lineSeries = [chartLineSeries1, chartLineSeries2, chartLineSeries3];

			keys.slice(0, 3).forEach((key, index) => {
				const slice = chartPool[key];
				if (!slice) {
					console.warn('Chart slice not found for key', key);
					return;
				}
				const bars = slice.chartData.bars.map((bar: any) => ({
					time: bar.time as any,
					value: bar.close
				}));
				lineSeries[index]?.setData(bars);
			});
		}

		if (chartType === 'candle') {
			const slice = chartPool[keys[0]];
			if (!slice) {
				console.warn('Chart slice not found for key', keys[0]);
				return;
			}

			activeKey = keys[0];

			// Map backend bars into lightweight-charts format
			const bars = slice.chartData.bars.map((bar: any) => ({
				time: bar.time as any,
				open: bar.open,
				high: bar.high,
				low: bar.low,
				close: bar.close,
				volume: bar.volume ?? bar.v ?? bar.vol ?? 0
			}));
			chartCandleSeries?.setData(bars);
			console.log('bars', bars);
			chartVolumeSeries?.setData(
				bars.map((b: any) => ({
					time: b.time,
					value: b.volume,
					color: b.close >= b.open ? '#26a69a' : '#ef5350'
				}))
			);

			if (bars.length) {
				const last = bars[bars.length - 1];
				legendData = {
					open: last.open,
					high: last.high,
					low: last.low,
					close: last.close,
					volume: last.volume
				} as any;
			}
		}
	}

	// ----- Local state -----
	let isLoaded = false;

	// Animation state management
	let animationPhase: 'initial' | 'typing' | 'submitted' | 'complete' = 'initial';
	let showHeroContent = false;
	let animationInput = '';
	let animationInputRef: HTMLTextAreaElement;
	let typewriterIndex = 0;
	let typewriterInterval: NodeJS.Timeout;
	// Control removal of the animation input bar after transition
	let removeAnimationInput = false;
	let chartContainerRef: HTMLDivElement;

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

	// Add 3 line series
	let chartLineSeries1: ISeriesApi<
		'Line',
		Time,
		WhitespaceData<Time> | LineData<Time>,
		LineSeriesOptions,
		DeepPartial<LineStyleOptions & SeriesOptionsCommon>
	>;
	let chartLineSeries2: ISeriesApi<
		'Line',
		Time,
		WhitespaceData<Time> | LineData<Time>,
		LineSeriesOptions,
		DeepPartial<LineStyleOptions & SeriesOptionsCommon>
	>;
	let chartLineSeries3: ISeriesApi<
		'Line',
		Time,
		WhitespaceData<Time> | LineData<Time>,
		LineSeriesOptions,
		DeepPartial<LineStyleOptions & SeriesOptionsCommon>
	>;

	let chatMessages: ChatMessage[] = [];

	// After the declaration of chatMessages, add a reference for the chat messages element
	let heroChatMessagesRef: HTMLDivElement;

	// Create a wrapper function for the event listener to fix type mismatch
	let tickerButtonHandler: ((event: Event) => void) | null = null;

	/* ------------------------------------------------
	   Timeline & typewriter helpers
    ------------------------------------------------*/
	function addMessage(text: string) {
		const msg: ChatMessage = {
			message_id: 'hero_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7),
			sender: 'user',
			text,
			contentChunks: [{ type: 'text', content: text }]
		};
		chatMessages = [...chatMessages, msg];
	}

	function addAssistantMessage(contentChunks: ContentChunk[]) {
		const msg: ChatMessage = {
			message_id: 'hero_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7),
			sender: 'assistant',
			contentChunks
		};
		chatMessages = [...chatMessages, msg];
	}

	function removeLastMessage() {
		chatMessages = chatMessages.slice(0, -1);
	}

	const timelineEvents: TimelineEvent[] = createTimelineEvents({
		addUserMessage: addMessage,
		addAssistantMessage,
		removeLastMessage,
		highlightEventForward,
		setChart
	});

	let heroWrapper: HTMLElement;

	// Function to safely access table data properties (mirrors chat.svelte)
	function getTableData(content: any): TableData | null {
		if (isTableData(content)) {
			return content;
		}
		return null;
	}

	// Function to navigate to a specific page
	function goToPage(tableKey: string, pageNumber: number, totalPages: number) {
		if (pageNumber >= 1 && pageNumber <= totalPages) {
			tablePaginationStates[tableKey].currentPage = pageNumber;
			tablePaginationStates = { ...tablePaginationStates }; // Trigger reactivity
		}
	}

	// Function to go to next page
	function nextPage(tableKey: string, currentPage: number, totalPages: number) {
		if (currentPage < totalPages) {
			goToPage(tableKey, currentPage + 1, totalPages);
		}
	}

	// Function to go to previous page
	function previousPage(tableKey: string, currentPage: number, totalPages: number) {
		if (currentPage > 1) {
			goToPage(tableKey, currentPage - 1, totalPages);
		}
	}

	// Function to sort table data
	function sortTable(tableKey: string, columnIndex: number, tableData: TableData) {
		const currentSortState = tableSortStates[tableKey] || { columnIndex: null, direction: null };
		let newDirection: 'asc' | 'desc' | null = 'asc';

		if (currentSortState.columnIndex === columnIndex) {
			// Toggle direction if clicking the same column
			newDirection = currentSortState.direction === 'asc' ? 'desc' : 'asc';
		}

		// Update sort state for this table
		tableSortStates[tableKey] = { columnIndex, direction: newDirection };
		tableSortStates = { ...tableSortStates }; // Trigger reactivity

		// Sort the rows
		tableData.rows.sort((a, b) => {
			// Skip sorting if rows are not arrays
			if (!Array.isArray(a) || !Array.isArray(b)) {
				return 0;
			}
			const valA = a[columnIndex];
			const valB = b[columnIndex];

			// Handle null/undefined values
			if (valA == null && valB == null) return 0;
			if (valA == null) return 1;
			if (valB == null) return -1;

			let comparison = 0;

			// Check if both values are already numbers
			if (typeof valA === 'number' && typeof valB === 'number') {
				comparison = valA - valB;
			} else {
				// Convert to strings for comparison
				const strA = String(valA).trim();
				const strB = String(valB).trim();

				// Check if both strings represent numbers (more strict check)
				const numA = parseFloat(strA);
				const numB = parseFloat(strB);

				// Only treat as numbers if the entire string is a valid number
				if (!isNaN(numA) && !isNaN(numB) && strA === numA.toString() && strB === numB.toString()) {
					comparison = numA - numB;
				} else {
					// String comparison (case-insensitive)
					const lowerA = strA.toLowerCase();
					const lowerB = strB.toLowerCase();
					comparison = lowerA.localeCompare(lowerB);
				}
			}

			return newDirection === 'asc' ? comparison : comparison * -1;
		});

		// Reset to page 1 when sorting to avoid confusion
		if (tablePaginationStates[tableKey]) {
			tablePaginationStates[tableKey].currentPage = 1;
		}

		// Find the message containing this table and update its content_chunks
		// This is necessary because tableData is a copy within the #each loop
		chatMessages = chatMessages.map((msg) => {
			if (msg.contentChunks) {
				msg.contentChunks = msg.contentChunks.map((chunk, idx) => {
					const currentTableKey = msg.message_id + '-' + idx;
					if (
						currentTableKey === tableKey &&
						chunk.type === 'table' &&
						isTableData(chunk.content)
					) {
						// Return a new chunk object with the sorted rows
						return {
							...chunk,
							content: {
								...chunk.content,
								rows: [...tableData.rows] // Ensure a new array reference
							}
						};
					}
					return chunk;
				});
			}
			return msg;
		});
	}

	function updateHeroProgress() {
		if (!heroWrapper) return;
		const rect = heroWrapper.getBoundingClientRect();
		const travelled = Math.min(Math.max(-rect.top, 0), totalScroll);
		const newProgress = travelled / totalScroll;

		// Timeline progress for events
		timelineProgress.set(newProgress);
		evaluateTimeline();

		// Synchronize the chat scroll position with overall hero progress
		updateChatScroll();
	}

	// Control chat auto-sync (progress → scroll) – disabled once we start manual scrolling
	let chatAutoSync = true;

	// Programmatically scroll the chat messages pane based on global timeline progress
	function updateChatScroll() {
		if (!chatAutoSync) return;
		if (!heroChatMessagesRef) return;
		requestAnimationFrame(() => {
			if (!heroChatMessagesRef) return;
			const maxScroll = heroChatMessagesRef.scrollHeight - heroChatMessagesRef.clientHeight;
			if (maxScroll <= 0) return;
			heroChatMessagesRef.scrollTop = maxScroll * get(timelineProgress);
		});
	}

	function evaluateTimeline() {
		const progress = get(timelineProgress);
		for (const evt of timelineEvents) {
			if (!evt.fired && progress >= evt.trigger) {
				evt.forward();
				evt.fired = true;
			} else if (evt.fired && progress < evt.trigger) {
				evt.backward?.();
				evt.fired = false;
			}
		}
	}

	function startTypewriterEffect() {
		animationPhase = 'typing';
		typewriterInterval = setInterval(() => {
			if (typewriterIndex < sampleQuery.length) {
				animationInput = sampleQuery.slice(0, typewriterIndex + 1);
				typewriterIndex++;
			} else {
				clearInterval(typewriterInterval);
				setTimeout(() => submitAnimationQuery(), 100);
			}
		}, 22);
	}

	function submitAnimationQuery() {
		animationPhase = 'submitted';
		// After CSS transition remove the input element
		setTimeout(() => {
			removeAnimationInput = true;
			animationPhase = 'complete';
		}, 600);
		setTimeout(() => {
			animationPhase = 'complete';
			showHeroContent = true;
		}, 800);
	}
	// ------- Hero chart legend state ---------
	const currentTicker = 'QQQ';
	interface LegendData {
		open: number;
		high: number;
		low: number;
		close: number;
		volume: number;
	}
	let legendData: LegendData = { open: 0, high: 0, low: 0, close: 0, volume: 0 };

	// Immediately after hero chat state variables
	let selectedRowKey: string | null = null;
	// References to chart & series for later updates (used for legend updates etc.)
	let chartInstance: any;

	function highlightTableRow(tableKey: string, rowIndex: number) {
		// Retrieve table context (first table only for hero demo)
		const tableMsg = chatMessages.find((m) => m.contentChunks?.some((c) => c.type === 'table'));
		if (!tableMsg) return;
		const tableIdx = tableMsg.contentChunks.findIndex((c) => c.type === 'table');
		const tableData: any = tableMsg.contentChunks[tableIdx].content;
		const totalRows: number = tableData?.rows?.length ?? 0;
		if (totalRows === 0) return;
		const clampedRowIdx = Math.min(Math.max(rowIndex, 0), totalRows - 1);

		// Ensure correct pagination page so the row is rendered
		const pagState = tablePaginationStates[tableKey] || { currentPage: 1, rowsPerPage: 5 };
		const { rowsPerPage } = pagState;
		const totalPages = Math.ceil(totalRows / rowsPerPage);
		const requiredPage = Math.floor(clampedRowIdx / rowsPerPage) + 1;
		if (requiredPage > totalPages) return; // invalid

		if (pagState.currentPage !== requiredPage) {
			tablePaginationStates[tableKey] = { ...pagState, currentPage: requiredPage } as any;
			tablePaginationStates = { ...tablePaginationStates }; // trigger reactivity
		}
		rowIndex = clampedRowIdx;

		selectedRowKey = `${tableKey}-${rowIndex}`;

		// Wait for DOM to update, then scroll
		setTimeout(() => {
			const rowEl = document.querySelector(`tr[data-row-key="${selectedRowKey}"]`);
			if (!rowEl || !heroChatMessagesRef) return;

			const container = heroChatMessagesRef;
			const containerRect = container.getBoundingClientRect();
			const rowRect = (rowEl as HTMLElement).getBoundingClientRect();

			const isAbove = rowRect.top < containerRect.top;
			const isBelow = rowRect.bottom > containerRect.bottom;

			if (isAbove || isBelow) {
				scrollElementIntoChat(rowEl as HTMLElement);
			}
		}, 50);

		// Stop automatic scroll syncing after first explicit highlight
		chatAutoSync = false;
	}

	// Smoothly scroll chat container so `el` is near the top
	function scrollElementIntoChat(el: HTMLElement) {
		if (!heroChatMessagesRef) return;
		const container = heroChatMessagesRef;
		const offset =
			el.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
		container.scrollTo({ top: Math.max(offset - 8, 0), behavior: 'smooth' });
		chatAutoSync = false; // stop auto sync after manual scroll
	}

	// Ensure the table as a whole is visible in the chat pane
	function scrollTableIntoView() {
		if (!heroChatMessagesRef) return;
		const tableContainer = heroChatMessagesRef.querySelector('.chunk-table-container');
		if (tableContainer) {
			scrollElementIntoChat(tableContainer as HTMLElement);
		}
	}

	// Ensure the plot as a whole is visible in the chat pane
	function scrollPlotIntoView() {
		if (!heroChatMessagesRef) return;
		const plotContainer = heroChatMessagesRef.querySelector('.hero-plot-container');
		if (plotContainer) {
			scrollElementIntoChat(plotContainer as HTMLElement);
		}
	}

	// New: extracted highlighter so it’s in scope before reactive block
	export function highlightEventForward(rowIndex: number = -1, attempts = 6) {
		if (rowIndex === -2) {
			// Special case: scroll plot into view
			setTimeout(() => {
				scrollPlotIntoView();
				chatAutoSync = false; // disable auto sync after manual scroll
			}, 100);
			return;
		}

		// Ensure table container is visible
		scrollTableIntoView();

		if (rowIndex < 0) return; // just scroll table, auto-sync already disabled

		// Locate table row; if not yet in DOM (async rendering), retry a few times
		const tryHighlight = () => {
			const tableMsg = chatMessages.find((m) => m.contentChunks?.some((c) => c.type === 'table'));
			if (!tableMsg) {
				if (attempts > 0) setTimeout(() => highlightEventForward(rowIndex, attempts - 1), 50);
				return;
			}
			const tableIdx = tableMsg.contentChunks.findIndex((c) => c.type === 'table');
			const tableKey = `${tableMsg.message_id}-${tableIdx}`;
			highlightTableRow(tableKey, rowIndex);
		};

		// Initial slight delay to allow DOM update, then attempt highlight
		setTimeout(tryHighlight, 60);
	}

	// Chart & scroll setup
	onMount(() => {
		if (!browser) return;
		document.documentElement.style.setProperty('--hero-total-scroll', `${totalScroll}px`);
		updateHeroProgress();
		window.addEventListener('scroll', updateHeroProgress, { passive: true });
		// Pricing preload removed - now handled directly in pricing page

		// Set loaded flag for fade-in
		isLoaded = true;
		setTimeout(() => startTypewriterEffect(), 0);
		evaluateTimeline();

		// Add delegated event listener for ticker buttons in hero chat
		let heroChatContainer: HTMLDivElement;
		const heroChatEl = document.querySelector('.hero-chat-container');
		if (heroChatEl) {
			heroChatContainer = heroChatEl as HTMLDivElement;
			tickerButtonHandler = (event: Event) => handleTickerButtonClick(event as MouseEvent);
			heroChatContainer.addEventListener('click', tickerButtonHandler);
		}

		if (!chartContainerRef) return;

		/* ----------------------------
		   Initialise lightweight-chart
		   ---------------------------- */
		const chart = createChart(chartContainerRef, {
			width: chartContainerRef.clientWidth,
			height: chartContainerRef.clientHeight,
			layout: {
				background: { color: 'transparent' },
				textColor: '#0B2E33',
				attributionLogo: false
			},
			grid: {
				vertLines: { visible: true, color: 'rgba(11, 46, 51, 0.15)', style: 1 },
				horzLines: { visible: true, color: 'rgba(11, 46, 51, 0.15)', style: 1 }
			},
			timeScale: { visible: true },
			handleScroll: {
				mouseWheel: false,
				pressedMouseMove: true,
				horzTouchDrag: true,
				vertTouchDrag: true
			},
			handleScale: { mouseWheel: false, pinch: true }
		});

		// Candle series
		const candleSeries = chart.addCandlestickSeries({
			upColor: '#26a69a',
			downColor: '#ef5350',
			borderVisible: false,
			wickUpColor: '#26a69a',
			wickDownColor: '#ef5350'
		});

		// Volume histogram – matches chart.svelte settings
		const volumeSeries = chart.addHistogramSeries({
			lastValueVisible: false,
			priceLineVisible: false,
			priceFormat: { type: 'volume' },
			priceScaleId: ''
		});
		volumeSeries
			.priceScale()
			.applyOptions({ scaleMargins: { top: 0.9, bottom: 0 }, visible: false });

		// Add 3 line series with different colors
		const lineSeries1 = chart.addLineSeries({
			color: '#2196F3',
			lineWidth: 2,
			title: 'Line 1'
		});
		const lineSeries2 = chart.addLineSeries({
			color: '#FF9800',
			lineWidth: 2,
			title: 'Line 2'
		});
		const lineSeries3 = chart.addLineSeries({
			color: '#4CAF50',
			lineWidth: 2,
			title: 'Line 3'
		});

		// Subscribe to crosshair for reactive legend updates
		chart.subscribeCrosshairMove((param) => {
			if (!param || !param.seriesData) return;
			const bar = param.seriesData.get(candleSeries);
			const volBar = param.seriesData.get(volumeSeries);
			if (bar && typeof bar === 'object' && 'open' in bar) {
				const volumeVal =
					volBar && typeof volBar === 'object' && 'value' in volBar
						? (volBar as any).value
						: legendData.volume;
				legendData = {
					open: (bar as any).open,
					high: (bar as any).high,
					low: (bar as any).low,
					close: (bar as any).close,
					volume: volumeVal
				} as any;
			}
		});

		// In onMount chart init, assign refs
		chartInstance = chart;
		chartCandleSeries = candleSeries;
		chartVolumeSeries = volumeSeries;
		chartLineSeries1 = lineSeries1;
		chartLineSeries2 = lineSeries2;
		chartLineSeries3 = lineSeries3;

		new ResizeObserver(() => {
			chart.applyOptions({
				width: chartContainerRef!.clientWidth,
				height: chartContainerRef!.clientHeight
			});
		}).observe(chartContainerRef);

		// Inject server-preloaded data for the default key (first slice)
		console.log('activeKey', activeKey);
		setChart([activeKey], 'candle');
	});

	onDestroy(() => {
		if (!browser) return;
		window.removeEventListener('scroll', updateHeroProgress);

		// Clean up ticker button event listener
		const heroChatEl = document.querySelector('.hero-chat-container');
		if (heroChatEl) {
			if (tickerButtonHandler) {
				heroChatEl.removeEventListener('click', tickerButtonHandler);
			}
		}
	});
</script>

<section bind:this={heroWrapper} class="hero-wrapper">
	<div class="hero-pin">
		<!-- Hero Section -->
		<div class="hero-section" class:loaded={isLoaded}>
			<!-- Hero Header - Always Visible -->
			<div class="hero-header">
				<h1 class="hero-title">
					The <span class="gradient-text">best</span> way to trade.
				</h1>
				<p class="hero-subtitle">
					Peripheral is the terminal to envision and execute your trading ideas.<br />
				</p>
			</div>

			<!-- Animation Input Bar -->
			{#if !removeAnimationInput}
				<div
					class="animation-input-container"
					class:typing={animationPhase === 'typing'}
					class:submitted={animationPhase === 'submitted'}
					class:complete={animationPhase === 'complete'}
				>
					<div class="animation-input-wrapper">
						<textarea
							class="animation-input"
							bind:value={animationInput}
							bind:this={animationInputRef}
							readonly
							rows="1"
							class:typing-cursor={animationPhase === 'typing'}
						></textarea>
						<button class="animation-send" class:pulse={animationPhase === 'submitted'}>
							<svg viewBox="0 0 18 18" class="send-icon">
								<path
									d="M7.99992 14.9993V5.41334L4.70696 8.70631C4.31643 9.09683 3.68342 9.09683 3.29289 8.70631C2.90237 8.31578 2.90237 7.68277 3.29289 7.29225L8.29289 2.29225L8.36906 2.22389C8.76184 1.90354 9.34084 1.92613 9.70696 2.29225L14.707 7.29225L14.7753 7.36842C15.0957 7.76119 15.0731 8.34019 14.707 8.70631C14.3408 9.07242 13.7618 9.09502 13.3691 8.77467L13.2929 8.70631L9.99992 5.41334V14.9993C9.99992 15.5516 9.55221 15.9993 8.99992 15.9993C8.44764 15.9993 7.99993 15.5516 7.99992 14.9993Z"
								/>
							</svg>
						</button>
					</div>
				</div>
			{/if}

			<!-- Hero Actions - Revealed after animation -->
			<div class="hero-actions" class:show={showHeroContent} style="margin-top: 0;">
				<div class="hero-chat-container hero-widget" class:has-messages={chatMessages.length > 0}>
					<div
						class="hero-chat-messages"
						class:has-messages={chatMessages.length > 0}
						bind:this={heroChatMessagesRef}
					>
						{#if chatMessages.length !== 0}
							{#each chatMessages as msg (msg.message_id)}
								<div
									in:fade={{ duration: 200 }}
									out:fade={{ duration: 200 }}
									class="message-wrapper {msg.sender}"
								>
									{#if msg.sender === 'user'}
										<div class="message user">
											<div class="message-content">
												<div class="chunk-text">
													{@html parseMarkdown(msg.text || '')}
												</div>
											</div>
										</div>
									{:else if msg.contentChunks && msg.contentChunks.length > 0}
										<div class="assistant-message">
											{#each msg.contentChunks as chunk, idx}
												{#if chunk.type === 'text'}
													<div class="chunk-text">
														{@html parseMarkdown(
															typeof chunk.content === 'string'
																? chunk.content
																: String(chunk.content)
														)}
													</div>
												{:else if chunk.type === 'table'}
													{#if isTableData(chunk.content)}
														{@const tableData = getTableData(chunk.content)}
														{@const tableKey = msg.message_id + '-' + idx}
														{@const currentSort = tableSortStates[tableKey] || {
															columnIndex: null,
															direction: null
														}}

														{#if tableData}
															{@const paginationState =
																tablePaginationStates[tableKey] ||
																(tablePaginationStates[tableKey] = {
																	currentPage: 1,
																	rowsPerPage: 5
																})}
															{@const currentPage = paginationState.currentPage}
															{@const rowsPerPage = paginationState.rowsPerPage}
															{@const totalRows = tableData.rows.length}
															{@const totalPages = Math.ceil(totalRows / rowsPerPage)}
															{@const startIndex = (currentPage - 1) * rowsPerPage}
															{@const endIndex = Math.min(startIndex + rowsPerPage, totalRows)}
															{@const displayedRows = tableData.rows.slice(startIndex, endIndex)}

															<div class="chunk-table-container">
																{#if tableData.caption}
																	<div class="table-caption">
																		{@html parseMarkdown(tableData.caption)}
																	</div>
																{/if}
																<div class="chunk-table">
																	<table>
																		<thead>
																			<tr>
																				{#each tableData.headers as header, colIndex}
																					<th
																						on:click={() =>
																							sortTable(
																								tableKey,
																								colIndex,
																								JSON.parse(JSON.stringify(tableData))
																							)}
																						class:sortable={true}
																						class:sorted={currentSort.columnIndex === colIndex}
																						class:asc={currentSort.columnIndex === colIndex &&
																							currentSort.direction === 'asc'}
																						class:desc={currentSort.columnIndex === colIndex &&
																							currentSort.direction === 'desc'}
																					>
																						{@html parseMarkdown(
																							typeof header === 'string' ? header : String(header)
																						)}
																						{#if currentSort.columnIndex === colIndex}
																							<span class="sort-indicator">
																								{currentSort.direction === 'asc' ? '▲' : '▼'}
																							</span>
																						{/if}
																					</th>
																				{/each}
																			</tr>
																		</thead>
																		<tbody>
																			{#each displayedRows as row, rowIndex}
																				<tr
																					data-row-key={`${tableKey}-${startIndex + rowIndex}`}
																					class:selected-row={selectedRowKey ===
																						`${tableKey}-${startIndex + rowIndex}`}
																				>
																					{#if Array.isArray(row)}
																						{#each row as cell}
																							<td
																								>{@html parseMarkdown(
																									typeof cell === 'string' ? cell : String(cell)
																								)}</td
																							>
																						{/each}
																					{:else}
																						<td colspan={tableData.headers.length}
																							>Invalid row data: {typeof row === 'string'
																								? row
																								: String(row)}</td
																						>
																					{/if}
																				</tr>
																			{/each}
																		</tbody>
																	</table>
																</div>

																{#if totalPages > 1}
																	<div class="table-pagination">
																		<div class="pagination-controls">
																			<button
																				class="pagination-btn glass glass--small {1 === currentPage
																					? 'active'
																					: ''}"
																				on:click={() => goToPage(tableKey, 1, totalPages)}
																				title="First page"
																			>
																				<svg
																					viewBox="0 0 24 24"
																					width="14"
																					height="14"
																					fill="currentColor"
																				>
																					<path
																						d="M18.41,16.59L13.82,12L18.41,7.41L17,6L11,12L17,18L18.41,16.59M6,6H8V18H6V6Z"
																					/>
																				</svg>
																			</button>
																			<button
																				class="pagination-btn glass glass--small"
																				on:click={() =>
																					previousPage(tableKey, currentPage, totalPages)}
																				disabled={currentPage === 1}
																				title="Previous page"
																			>
																				<svg
																					viewBox="0 0 24 24"
																					width="14"
																					height="14"
																					fill="currentColor"
																				>
																					<path
																						d="M15.41,16.59L10.83,12L15.41,7.41L14,6L8,12L14,18L15.41,16.59Z"
																					/>
																				</svg>
																			</button>
																			<button
																				class="pagination-btn glass glass--small"
																				on:click={() => nextPage(tableKey, currentPage, totalPages)}
																				disabled={currentPage === totalPages}
																				title="Next page"
																			>
																				<svg
																					viewBox="0 0 24 24"
																					width="14"
																					height="14"
																					fill="currentColor"
																				>
																					<path
																						d="M8.59,16.59L13.17,12L8.59,7.41L10,6L16,12L10,18L8.59,16.59Z"
																					/>
																				</svg>
																			</button>
																			<button
																				class="pagination-btn glass glass--small {totalPages ===
																				currentPage
																					? 'active'
																					: ''}"
																				on:click={() => goToPage(tableKey, totalPages, totalPages)}
																				title="Last page"
																			>
																				<svg
																					viewBox="0 0 24 24"
																					width="14"
																					height="14"
																					fill="currentColor"
																				>
																					<path
																						d="M5.59,7.41L10.18,12L5.59,16.59L7,18L13,12L7,6L5.59,7.41M16,6H18V18H16V6Z"
																					/>
																				</svg>
																			</button>
																		</div>
																		<div class="pagination-info">
																			Page {currentPage} of {totalPages}
																		</div>
																	</div>
																{/if}
															</div>
														{:else}
															<div class="chunk-error">Invalid table data structure</div>
														{/if}
													{:else}
														<div class="chunk-error">Invalid table data format</div>
													{/if}
												{:else if chunk.type === 'plot'}
													{#if isPlotData(chunk.content)}
														{@const plotData = getPlotData(chunk.content)}
														{#if plotData}
															<HeroPlotChunk
																{plotData}
																plotKey={generatePlotKey(msg.message_id, idx)}
															/>
														{:else}
															<p>Invalid plot data structure</p>
														{/if}
													{:else}
														<p>Invalid plot data</p>
													{/if}
												{/if}
											{/each}
										</div>
									{:else}
										<div class="assistant-message">
											{@html parseMarkdown(msg.text || '')}
										</div>
									{/if}
								</div>
							{/each}
						{/if}
					</div>
				</div>
				<div class="hero-chart-container hero-widget" bind:this={chartContainerRef}>
					<!-- Legend overlay -->
					<div class="hero-chart-legend">
						<span class="ticker">{currentTicker}</span>
						<span>O {legendData.open?.toFixed(2)}</span>
						<span>H {legendData.high?.toFixed(2)}</span>
						<span>L {legendData.low?.toFixed(2)}</span>
						<span>C {legendData.close?.toFixed(2)}</span>
						<span>V {legendData.volume?.toLocaleString()}</span>
					</div>
				</div>
			</div>
		</div>
	</div>
</section>

<style>
	:root {
		--hero-total-scroll: 0;
	}
	/* Animation Input Bar - Mobile */
	.animation-input-container {
		position: relative;
		width: 90vw;
		max-width: 800px;
		opacity: 1;
		transition: all 1.5s cubic-bezier(0.4, 0, 0.2, 1);
		z-index: 20;
		margin-bottom: 1.5rem;
	}

	.animation-input-wrapper {
		padding: 0.75rem 1rem;
		gap: 0.75rem;
	}
	.animation-input-wrapper {
		position: relative;
		display: flex;
		align-items: center;
		gap: 1rem;
		background: rgba(255, 255, 255, 0.95);
		border: 2px solid rgba(79, 124, 130, 0.3);
		border-radius: 28px;
		padding: 1rem 1.5rem;
		backdrop-filter: blur(20px);
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
		transition: all 0.3s ease;
	}
	.animation-input-wrapper:focus-within {
		border-color: var(--color-primary);
		box-shadow: 0 12px 48px rgba(79, 124, 130, 0.2);
	}

	.animation-input {
		flex: 1;
		background: none;
		border: none;
		outline: none;
		color: var(--color-dark);
		font-size: 1.1rem;
		line-height: 1.5;
		resize: none;
		font-family: inherit;
		font-weight: 500;
		padding: 0;
		min-height: 28px;
	}

	.animation-send {
		background: var(--color-primary);
		border: none;
		border-radius: 50%;
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition: all 0.3s ease;
		flex-shrink: 0;
	}
	.animation-send:hover {
		background: var(--color-dark);
		transform: scale(1.05);
	}
	.animation-send.pulse {
		animation: pulse 0.6s ease-in-out;
	}

	@keyframes pulse {
		0% {
			transform: scale(1);
		}
		50% {
			transform: scale(1.2);
			background: #10b981;
		}
		100% {
			transform: scale(1);
		}
	}

	.animation-send .send-icon {
		width: 18px;
		height: 18px;
		fill: white;
	}

	.animation-send .send-icon {
		width: 16px;
		height: 16px;
	}
	.animation-input-container.complete {
		/* Fade out instead of dropping down */
		opacity: 1;
		transform: none;
		pointer-events: none;
		animation: containerFadeOut 0.4s 0.2s forwards ease;
	}

	.animation-input-container.submitted::after {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: 28px; /* match input wrapper radius */
		pointer-events: none;
		border: 2px solid var(--color-primary);
		box-shadow: 0 0 0 0 rgba(79, 124, 130, 0.6);
		animation: ringPulse 1s ease-out forwards;
	}
	@keyframes ringPulse {
		/* Faster expansion (first 0.2s), longer hold (0.5s), then fade */
		0% {
			box-shadow: 0 0 0 0 rgba(79, 124, 130, 0.6);
			opacity: 1;
		}
		20% {
			box-shadow: 0 0 0 8px rgba(79, 124, 130, 0.6);
			opacity: 1;
		}
		70% {
			box-shadow: 0 0 0 8px rgba(79, 124, 130, 0.6);
			opacity: 1;
		}
		100% {
			box-shadow: 0 0 0 8px rgba(79, 124, 130, 0);
			opacity: 0;
		}
	}

	/* Fade the entire container out once the ring pulse finishes */
	@keyframes containerFadeOut {
		0% {
			opacity: 1;
			transform: scale(1);
		}
		100% {
			opacity: 0;
			transform: scale(0.95);
		}
	}

	/* hero stuff */
	.hero-title {
		font-size: clamp(2.7rem, 4vw, 5rem);
		font-weight: 800;
		margin: 0 0 1.5rem 0;
		letter-spacing: -0.02em;
		line-height: 1.1;
		color: #f5f9ff;
		text-shadow:
			0 2px 12px rgba(0, 0, 0, 0.2),
			0 1px 0 rgba(255, 255, 255, 0.01);
		padding-top: var(--header-h);
	}
	.gradient-text {
		background: linear-gradient(
			135deg,
			#3b82f6 0%,
			#6366f1 25%,
			#8b5cf6 50%,
			#ec4899 75%,
			#f59e0b 100%
		);
		background-size: 200% 200%;
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
		animation: gradient-shift 8s ease infinite;
	}

	@keyframes gradient-shift {
		0%,
		100% {
			background-position: 0% 50%;
		}
		25% {
			background-position: 100% 50%;
		}
		50% {
			background-position: 100% 100%;
		}
		75% {
			background-position: 0% 100%;
		}
	}
	/* Shift hero title & subtitle downward */
	.hero-header {
		margin-top: 6rem; /* adjust as needed */
	}
	.hero-subtitle {
		font-size: clamp(1.1rem, 3vw, 1.5rem);
		color: rgba(245, 249, 255, 0.85);
		margin-bottom: 1.5rem;
		line-height: 1.6;
		margin-top: 0;
		font-weight: 400;
	}

	/* Hero Section */
	.hero-section {
		position: relative;
		z-index: 10;
		min-height: 100vh;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		align-items: center;
		padding: 2rem 2rem 4rem;
		padding-top: calc(var(--header-h) - 1rem);
		text-align: center;
		width: 100%;
		flex-shrink: 0;
		isolation: isolate;
		border-radius: 4.5rem;
	}
	@media (max-width: 768px) {
		.hero-section {
			padding: 1rem 1rem 3rem;
			padding-top: calc(var(--header-h) + 1rem);
		}

		.hero-actions {
			flex-direction: column;
			align-items: center;
		}
		:root {
			--hero-widget-h: 220px;
		}
		.hero-chat-container {
			max-width: 100%;
			min-height: 220px;
			max-height: 260px;
		}
		.hero-chart-container {
			max-width: 100%;
			height: var(--hero-widget-h);
		}
		/* Hero Header - Always Visible */
		.hero-header {
			text-align: center;
			opacity: 1;
			transform: translateY(0);
			margin-top: 2rem; /* mobile: slightly less offset */
		}
	}
	/* Hero section halo */
	.hero-section::before {
		content: '';
		position: absolute;
		inset: 0;
		pointer-events: none;
		z-index: -1;
		/* Brighter hue – using primary brand colour */
		--halo-rgb: 79, 124, 130;
		/* Inner colour wash */
		background: radial-gradient(
			ellipse at 50% 50%,
			rgba(var(--halo-rgb), 0.55) 0%,
			rgba(var(--halo-rgb), 0.25) 45%,
			rgba(var(--halo-rgb), 0) 70%
		);
		/* Concentric steps */
		box-shadow:
			0 0 0 48px rgba(var(--halo-rgb), 0.15),
			0 0 0 96px rgba(var(--halo-rgb), 0.1),
			0 0 0 144px rgba(var(--halo-rgb), 0.07),
			0 0 0 192px rgba(var(--halo-rgb), 0.04),
			0 0 0 240px rgba(var(--halo-rgb), 0.02);
		/* Slightly crisper blur */
		filter: blur(28px);
		border-radius: 28px; /* match parent radius */
	}

	.animation-input::placeholder {
		color: rgba(11, 46, 51, 0.5);
	}

	.animation-input.typing-cursor::after {
		content: '|';
		animation: blink 1s infinite;
		margin-left: 2px;
	}

	@keyframes blink {
		0%,
		50% {
			opacity: 1;
		}
		51%,
		100% {
			opacity: 0;
		}
	}

	/* Hero Actions - Initially hidden */
	.hero-actions {
		display: flex;
		gap: 1rem;
		justify-content: center;
		flex-wrap: wrap;
		opacity: 0;
		transform: translateY(50px);
		transition: all 1.2s ease;
		pointer-events: none;
		width: 100%;
		max-width: 800px;
		margin-top: 0;
		/* Allow hero-actions to fill remaining vertical space */
		flex: 0 1 auto;
		height: auto;
	}

	.hero-actions.show {
		opacity: 1;
		transform: translateY(0);
		pointer-events: auto;
	}

	/* Hero Chat Interface */
	.hero-chat-container {
		flex: 1;
		width: 100%;
		max-width: 500px;
		display: flex;
		flex-direction: column;
		gap: 0; /* Remove gap since there's only the messages pane */
		margin: 0;
		/* Fill available vertical space */
		min-height: 280px;
		height: var(--hero-widget-h);
		max-height: none;
	}

	.hero-chat-messages {
		background: var(--hero-widget-background-color);
		border: 1px solid rgba(255, 255, 255, 1);
		border-radius: 16px;
		padding: 1.5rem;
		overflow-y: hidden; /* Disable manual scrolling, controlled programmatically */
		display: flex;
		flex-direction: column;
		gap: 1rem;
		min-height: 120px;
		flex: 1;
		justify-content: center;
		align-items: center;
		position: relative;
	}

	.hero-chat-placeholder {
		text-align: center;
		color: rgba(255, 255, 255, 0.7);
		font-size: 1rem;
		line-height: 1.5;
		width: 100%;
		max-width: 90%;
	}

	/* Mini chart next to chat */
	.hero-chart-container {
		flex: 1;
		width: 100%;
		max-width: 400px;
		height: var(--hero-widget-h);
		max-height: none;
		border-radius: 16px;
		background: var(--hero-widget-background-color);
		border: 1px solid var(--hero-widget-background-color);
		position: relative;
	}

	/* Responsive adjustments for hero chat */
	@media (max-width: 768px) {
		.hero-chat-container {
			max-width: 100%;
			min-height: 220px;
			max-height: 260px;
		}

		.hero-chat-messages {
			padding: 1rem;
		}

		.hero-chart-container {
			max-width: 100%;
			height: var(--hero-widget-h);
			max-height: 260px;
		}
	}

	@media (max-width: 480px) {
		:root {
			--hero-widget-h: 220px;
		}
		.hero-chat-container {
			min-height: 160px;
			max-height: 220px;
		}

		.hero-chat-messages {
			max-height: 120px;
			padding: 0.75rem;
		}

		.hero-chart-container {
			height: var(--hero-widget-h);
			max-height: 220px;
		}
	}
	/* ================================================
        Desktop layout: split chat & chart 50/50
        ================================================ */
	@media (min-width: 1024px) {
		.hero-actions {
			max-width: 75vw;
			display: grid;
			grid-template-columns: 40% 60%;
			gap: 2rem;
			justify-content: center;
		}

		.hero-chat-container {
			width: 100%; /* full width of its grid cell */
			max-width: none;
			max-height: none;
			min-height: 280px;
			height: var(--hero-widget-h);
		}

		.hero-chart-container {
			width: 100%;
			max-width: none;
			max-height: none;
			min-height: 280px;
			height: var(--hero-widget-h);
		}
	}

	/* When messages are present, align them to the top/left */
	.hero-chat-messages.has-messages {
		justify-content: flex-start;
		align-items: stretch;
		text-align: left;
	}

	/* Basic bubble styling mirroring chat.svelte */
	.message-wrapper {
		display: flex;
		flex-direction: column;
		width: 100%;
		align-items: flex-end; /* user alignment */
	}

	.message.user {
		background: linear-gradient(135deg, #0a84ff 0%, #007aff 100%);
		color: #ffffff;
		padding: 0.6rem 1rem;
		border-radius: 1rem;
		max-width: 80%;
		font-size: 0.9rem;
		border-bottom-right-radius: 0.25rem;
		box-shadow: 0 1px 4px rgba(0, 0, 0, 0.15);
		text-align: left;
	}

	.message-wrapper.assistant {
		align-items: flex-start;
	}

	.assistant-message {
		margin: 0;
		font-size: 0.9rem;
		color: var(--color-dark);
		width: 100%;
		max-width: 100%;
		text-align: left;
	}

	/* Force Inter font inside chat */
	.hero-chat-container,
	.hero-chat-container * {
		font-family: 'Geist';
	}

	/* Ticker button styles */
	.hero-chat-container :global(.ticker-button) {
		background: rgba(79, 124, 130, 0.1);
		color: #4f7c82;
		border: 1px solid rgba(79, 124, 130, 0.3);
		border-radius: 0.25rem;
		padding: 0.125rem 0.375rem;
		font-size: 0.75rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.2s ease;
		display: inline-block;
		margin: 0 0.125rem;
		font-family: 'Geist', monospace;
	}

	.hero-chat-container :global(.ticker-button:hover) {
		background: rgba(79, 124, 130, 0.2);
		border-color: #4f7c82;
		transform: translateY(-1px);
	}

	.hero-chat-container :global(.ticker-button:active) {
		transform: translateY(0);
	}

	.hero-chat-container :global(.ticker-button:disabled) {
		opacity: 0.6;
		cursor: not-allowed;
		transform: none;
	}

	/* Content chunk styling */
	.hero-chat-container :global(.chunk-text) {
		margin: 0;
	}

	.hero-chat-container :global(.chunk-text h1),
	.hero-chat-container :global(.chunk-text h2),
	.hero-chat-container :global(.chunk-text h3),
	.hero-chat-container :global(.chunk-text h4),
	.hero-chat-container :global(.chunk-text h5),
	.hero-chat-container :global(.chunk-text h6) {
		margin: 0.5rem 0;
		font-weight: 600;
	}

	.hero-chat-container :global(.chunk-text p) {
		margin: 0.5rem 0;
		line-height: 1.5;
	}

	.hero-chat-container :global(.chunk-text ul),
	.hero-chat-container :global(.chunk-text ol) {
		margin: 0.5rem 0;
		padding-left: 1.5rem;
	}

	/* Table formatting - compact version for hero */
	.hero-chat-container :global(.chunk-table-container) {
		margin: 0.5rem 0;
		border-radius: 0.375rem;
		background: rgba(255, 255, 255, 0.05);
		border: 1px solid rgba(255, 255, 255, 0.1);
		overflow: hidden;
		font-size: 0.75rem;
	}

	.hero-chat-container :global(.table-caption) {
		padding: 0.5rem;
		text-align: left;
		font-weight: 600;
		color: var(--color-dark);
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		background: rgba(255, 255, 255, 0.1);
		font-size: 0.85rem;
	}

	.hero-chat-container :global(.chunk-table) {
		overflow-x: auto;
		max-width: 100%;
	}

	.hero-chat-container :global(.chunk-table table) {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.8rem;
		line-height: 1.2rem;
	}

	.hero-chat-container :global(.chunk-table th) {
		padding: 0.375rem 0.5rem;
		text-align: left;
		font-weight: 600;
		color: var(--color-dark);
		background: rgba(255, 255, 255, 0.1);
		border-bottom: 1px solid rgba(255, 255, 255, 0.2);
		white-space: nowrap;
		cursor: pointer;
		user-select: none;
		position: relative;
		font-size: 0.75rem;
		vertical-align: middle;
	}

	.hero-chat-container :global(.chunk-table th.sortable:hover) {
		background: rgba(255, 255, 255, 0.15);
	}

	.hero-chat-container :global(.chunk-table th .sort-indicator) {
		margin-left: 0.15rem;
		font-size: 0.7rem;
		color: var(--color-primary);
	}

	.hero-chat-container :global(.chunk-table td) {
		padding: 0.375rem 0.5rem;
		color: var(--color-dark);
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		white-space: nowrap;
		font-size: 0.75rem;
		vertical-align: middle;
	}

	.hero-chat-container :global(.chunk-table tbody tr:last-child td) {
		border-bottom: none;
	}

	.hero-chat-container :global(.chunk-table tbody tr:hover) {
		background: rgba(255, 255, 255, 0.05);
	}

	/* Table pagination - compact version */
	.hero-chat-container :global(.table-pagination) {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.5rem;
		background: rgba(255, 255, 255, 0.05);
		border-top: 1px solid rgba(255, 255, 255, 0.1);
	}

	.hero-chat-container :global(.pagination-controls) {
		display: flex;
		gap: 0.15rem;
	}

	.hero-chat-container :global(.pagination-btn) {
		padding: 0.25rem;
		border: 1px solid rgba(255, 255, 255, 0.2);
		background: rgba(255, 255, 255, 0.1);
		color: var(--color-dark);
		border-radius: 0.2rem;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all 0.2s ease;
		width: 28px;
		height: 28px;
	}

	.hero-chat-container :global(.pagination-btn:hover:not(:disabled)) {
		background: rgba(255, 255, 255, 0.2);
		border-color: rgba(255, 255, 255, 0.3);
	}

	.hero-chat-container :global(.pagination-btn:disabled) {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.hero-chat-container :global(.pagination-btn.active) {
		background: var(--color-primary);
		border-color: var(--color-primary);
		color: white;
	}

	.hero-chat-container :global(.pagination-info) {
		font-size: 0.8rem;
		color: var(--color-dark);
		opacity: 0.8;
	}

	/* Error state styling */
	.hero-chat-container :global(.chunk-error) {
		padding: 1rem;
		color: #ef4444;
		background: rgba(239, 68, 68, 0.1);
		border: 1px solid rgba(239, 68, 68, 0.2);
		border-radius: 0.5rem;
		font-size: 0.875rem;
	}

	/* Responsive table design - compact but readable for hero */
	@media (max-width: 768px) {
		.hero-chat-container :global(.chunk-table table) {
			font-size: 0.7rem;
		}

		.hero-chat-container :global(.chunk-table th),
		.hero-chat-container :global(.chunk-table td) {
			padding: 0.25rem 0.375rem;
			font-size: 0.65rem;
		}

		.hero-chat-container :global(.table-pagination) {
			flex-direction: column;
			gap: 0.375rem;
			padding: 0.375rem;
		}

		.hero-chat-container :global(.pagination-btn) {
			width: 24px;
			height: 24px;
		}

		.hero-chat-container :global(.pagination-info) {
			font-size: 0.7rem;
		}
	}

	.hero-chart-legend {
		position: absolute;
		top: 6px;
		left: 6px;
		background: none;
		color: #000000;
		padding: 4px 6px;
		border-radius: 4px;
		font-size: 0.8rem;
		display: flex;
		gap: 0.4rem;
		pointer-events: none;
	}
	.hero-chart-legend .ticker {
		font-weight: 700;
	}

	/* Shared widget class */
	.hero-widget {
		/* Already uses variable – class mainly for semantics now */
		height: var(--hero-widget-h);
	}
	.hero-wrapper {
		height: calc(100vh + var(--hero-total-scroll)); /* 100 vh +  totalScroll (1500 px) */
		position: relative;
	}
	.hero-pin {
		position: sticky; /*  or 'fixed: top:0; left:0; width:100%' */
		top: 0;
		height: 100vh; /*  fills the viewport while pinned  */
	}

	.hero-chat-container :global(tr.selected-row) {
		background: rgba(79, 124, 130, 0.2) !important;
	}
</style>
