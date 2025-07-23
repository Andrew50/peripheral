<script lang="ts">
	import type { TimelineEvent } from '../interface';
	import { agentStatusStore } from '$lib/utils/stream/socket';
	import { browser } from '$app/environment';
	import { onDestroy } from 'svelte';
	import { createChart } from 'lightweight-charts';
	import type {
		IChartApi,
		ISeriesApi,
		Time,
		CandlestickData,
		UTCTimestamp,
		CandlestickSeriesOptions,
		CandlestickStyleOptions,
		SeriesOptionsCommon,
		WhitespaceData,
	} from 'lightweight-charts';



	export let isProcessingMessage: boolean = false;

	// Internal state
	let showTimelineDropdown: boolean = false;
	let timeline: TimelineEvent[] = [];
	let lastStatusMessage = '';
	let streamedMessages: Map<number, string> = new Map();
	let streamingTimeout: NodeJS.Timeout | null = null;
	let lastAnimatedIndex: number = -1;
	let animatedCitationCounts: Map<number, { current: number; target: number; isAnimating: boolean }> = new Map();

	// Chart instances management
	let chartInstances: Map<number, IChartApi> = new Map();

	// Debug reactive statements
	$: {
		console.log('🔄 ThinkingTrace DEBUG - isProcessingMessage:', isProcessingMessage);
		console.log('🔄 ThinkingTrace DEBUG - timeline.length:', timeline.length);
		console.log('🔄 ThinkingTrace DEBUG - timeline:', timeline);
	}

	// Derive current status from latest timeline message
	$: currentHeadline = timeline.length > 0 ? timeline[timeline.length - 1].headline || 'Thinking' : '';

	// Stream out function update messages when timeline changes
	$: if (timeline.length > 0 && browser) {
		const lastEvent = timeline[timeline.length - 1];
		if (lastEvent.type === 'FunctionUpdate' && lastEvent.data) {
			streamOutMessage(timeline.length - 1, lastEvent.data);
		}
	}

	function streamOutMessage(eventIndex: number, text: string) {
		// If there's already a streaming message, quickly finish it first
		if (streamingTimeout) {
			clearTimeout(streamingTimeout);
			// Find the currently streaming message and complete it instantly
			for (let [index, partialText] of streamedMessages.entries()) {
				const originalEvent = timeline[index];
				if (originalEvent && originalEvent.type === 'FunctionUpdate' && originalEvent.data) {
					if (partialText.length < originalEvent.data.length) {
						// Complete the previous message instantly
						streamedMessages.set(index, originalEvent.data);
					}
				}
			}
			streamedMessages = streamedMessages; // Trigger reactivity
		}
		
		// Reset streamed message for this event
		streamedMessages.delete(eventIndex);
		streamedMessages = streamedMessages; // Trigger reactivity
		
		// If no text, just return
		if (!text) return;
		
		// Split text into words for faster streaming
		const words = text.split(' ');
		let currentWordIndex = 0;
		
		function typeNextWordChunk() {
			if (currentWordIndex < words.length) {
				// Stream 1-2 words at a time for fast but visible effect
				const wordsToAdd = Math.min(3, words.length - currentWordIndex);
				const endIndex = currentWordIndex + wordsToAdd;
				const partialText = words.slice(0, endIndex).join(' ');
				
				streamedMessages.set(eventIndex, partialText);
				streamedMessages = streamedMessages; // Trigger reactivity
				currentWordIndex = endIndex;
				streamingTimeout = setTimeout(typeNextWordChunk, 15); // 15ms per word chunk
			} else {
				// Clear timeout when done
				streamingTimeout = null;
			}
		}
		
		typeNextWordChunk();
	}

	// Show dropdown toggle if there are timeline items to show
	$: showDropdownToggle = timeline.length > 0;
	

	// Check if there's an active web search
	$: hasActiveWebSearch = timeline.some(event => 
		event.type === 'webSearchQuery' && 
		!timeline.some(laterEvent => 
			laterEvent.type === 'webSearchCitations' && 
			laterEvent.timestamp > event.timestamp
		)
	);
	$: hasActiveBacktest = currentHeadline && 
		typeof currentHeadline === 'string' && 
		currentHeadline.toLowerCase().includes('backtest');

	// Timeline display logic: show all items when dropdown is expanded, otherwise show only the last item
	$: displayedTimelineItems = showTimelineDropdown 
		? timeline 
		: timeline.slice(-1); // Show only the last timeline item when collapsed



	// Reset timeline when processing starts/stops
	$: if (!isProcessingMessage) {
		timeline = [];
		lastStatusMessage = '';
		showTimelineDropdown = false;
		streamedMessages.clear();
		streamedMessages = streamedMessages; // Trigger reactivity
		lastAnimatedIndex = -1;
		animatedCitationCounts.clear();
		// Clear chart instances
		chartInstances.forEach(chart => {
			try {
				chart.remove();
			} catch (e) {
				console.warn('Error removing chart:', e);
			}
		});
		chartInstances.clear();
		if (streamingTimeout) {
			clearTimeout(streamingTimeout);
			streamingTimeout = null;
		}
	}
	$: if (isProcessingMessage && timeline.length == 0) {
		currentHeadline = 'Thinking';
	}
	// Listen to agent status store and build timeline
	$: if ($agentStatusStore && browser && isProcessingMessage) {
		const statusUpdate = $agentStatusStore;

		if (statusUpdate.type === 'FunctionUpdate' && statusUpdate.data && statusUpdate.data !== lastStatusMessage) {
			// Add function update message to timeline
			lastStatusMessage = statusUpdate.headline;
			timeline = [
				...timeline,
				{
					headline: statusUpdate.headline,
					timestamp: new Date(),
					type: 'FunctionUpdate',
					data: statusUpdate.data
				}
			];
		} else if (statusUpdate.type === 'WebSearchQuery' && statusUpdate.data?.query) {
			// Add web search event to timeline in chronological order
			lastStatusMessage = statusUpdate.headline;
			timeline = [
				...timeline,
				{
					headline: `Searching the web...`,
					timestamp: new Date(),
					type: 'webSearchQuery',
					data: statusUpdate.data
				}
			];
		} else if (statusUpdate.type === 'WebSearchCitations' && statusUpdate.data?.citations) {
			// Add web search citations to timeline
			if (timeline.length > 0 && timeline[timeline.length - 1].type === 'webSearchCitations') {
				timeline[timeline.length - 1].data.citations = [...timeline[timeline.length - 1].data.citations, ...statusUpdate.data.citations];
				lastAnimatedIndex = timeline.length - 2; //reset last animated index to the previous item
			} else {
				timeline = [
					...timeline,
					{
						headline: ``,
						timestamp: new Date(),
						type: 'webSearchCitations',
						data: statusUpdate.data
					}
				];
			}
		} else if (statusUpdate.type === 'getWatchlistItems' && statusUpdate.data) {
			// Add watchlist data to timeline
			lastStatusMessage = statusUpdate.headline;
			timeline = [
				...timeline,
				{
					headline: statusUpdate.headline,
					timestamp: new Date(),
					type: 'getWatchlistItems',
					data: statusUpdate.data
				}
			];
		} else if (statusUpdate.type === "getDailySnapshot" && statusUpdate.data) {
			// Add daily snapshot data to timeline
			lastStatusMessage = statusUpdate.headline;
			timeline = [
				...timeline,
				{
					headline: statusUpdate.headline,
					timestamp: new Date(),
					type: 'getDailySnapshot',
					data: statusUpdate.data
				}
			];
		}
	}

	// Internal toggle function
	function toggleDropdown() {
		showTimelineDropdown = !showTimelineDropdown;
	}

	// Function to check if timeline item should animate for the first time
	function shouldAnimateTimelineItem(timelineIndex: number): boolean {
		const actualIndex = showTimelineDropdown ? timelineIndex : timeline.length - 1;
		if (actualIndex > lastAnimatedIndex) {
			lastAnimatedIndex = actualIndex;
			return true;
		}
		return false;
	}

	// Function to start citation count animation
	function startCitationCountAnimation(timelineIndex: number, targetCount: number): void {
		const actualIndex = showTimelineDropdown ? timelineIndex : timeline.length - 1;
		
		// Don't animate if already exists
		if (animatedCitationCounts.has(actualIndex)) {
			return;
		}
		
		animatedCitationCounts.set(actualIndex, { current: 1, target: targetCount, isAnimating: true });
		
		// Start the counting animation with a 200ms delay
		if (targetCount > 1) {
			setTimeout(() => {
				const startTime = Date.now();
				const duration = 750; // Always 0.75 seconds
				
				// Ease-out function that slows down towards the end
				function easeOut(t: number): number {
					return 1 - Math.pow(1 - t, 2);
				}
				
				function updateCount() {
					const elapsed = Date.now() - startTime;
					const linearProgress = Math.min(elapsed / duration, 1);
					const easedProgress = easeOut(linearProgress);
					const current = Math.floor(1 + (targetCount - 1) * easedProgress);
					
					const countData = animatedCitationCounts.get(actualIndex);
					if (countData) {
						countData.current = current;
						countData.isAnimating = linearProgress < 1;
						animatedCitationCounts = animatedCitationCounts; // Trigger reactivity
						
						if (linearProgress < 1) {
							requestAnimationFrame(updateCount);
						}
					}
				}
				requestAnimationFrame(updateCount);
			}, 200);
		}
	}

	// Reactive statement to trigger citation animations
	$: if (browser && displayedTimelineItems.length > 0) {
		displayedTimelineItems.forEach((event, index) => {
			if (event.type === 'webSearchCitations' && event.data?.citations) {
				const actualIndex = showTimelineDropdown ? index : timeline.length - 1;
				if (actualIndex > lastAnimatedIndex && !animatedCitationCounts.has(actualIndex)) {
					startCitationCountAnimation(index, event.data.citations.length);
				}
			}
		});
	}



	// Create charts for getDailySnapshot events
	$: if (browser && displayedTimelineItems.length > 0) {
		displayedTimelineItems.forEach((event, index) => {
			if (event.type === 'getDailySnapshot' && event.data?.chartData) {
				// Find the original timeline index for this event
				const originalTimelineIndex = showTimelineDropdown 
					? index 
					: timeline.findIndex(item => item === event);
				setTimeout(() => createThinkingTraceChart(originalTimelineIndex, event.data.chartData), 50);
			}
		});
	}

	// Function to create chart for getDailySnapshot
	function createThinkingTraceChart(chartIndex: number, chartData: any[]) {
		const container = document.getElementById(`thinkingtrace-chart-${chartIndex}`);
		if (!container) return;

		// If chart instance already exists, remove it to force recreation
		// This ensures charts are recreated when timeline state changes
		if (chartInstances.has(chartIndex)) {
			const existingChart = chartInstances.get(chartIndex);
			try {
				existingChart?.remove();
			} catch (e) {
				console.warn('Error removing existing chart:', e);
			}
			chartInstances.delete(chartIndex);
		}
		
		try {
			const chart = createChart(container, {
				width: container.clientWidth,
				height: container.clientHeight,
				layout: {
					background: { color: 'transparent' },
					textColor: '#ffffff',
					attributionLogo: false
				},
				grid: {
					vertLines: { visible: true, color: 'rgba(255, 255, 255, 0.1)', style: 1 },
					horzLines: { visible: true, color: 'rgba(255, 255, 255, 0.1)', style: 1 }
				},
				timeScale: { 
					visible: false,
					timeVisible: false,
					secondsVisible: false
				},
				handleScroll: {
					mouseWheel: false,
					pressedMouseMove: false,
					horzTouchDrag: false,
					vertTouchDrag: false
				},
				handleScale: { 
					mouseWheel: false, 
					pinch: false 
				}
			});

			// Create candlestick series
			const candleSeries = chart.addCandlestickSeries({
				upColor: '#26a69a',
				downColor: '#ef5350',
				borderVisible: false,
				wickUpColor: '#26a69a',
				wickDownColor: '#ef5350'
			});

			// Convert backend data to lightweight-charts format
			const convertedData: CandlestickData[] = chartData.map((bar: any) => ({
				time: bar.timestamp as UTCTimestamp,
				open: bar.open,
				high: bar.high,
				low: bar.low,
				close: bar.close
			}));

			candleSeries.setData(convertedData);
			chart.timeScale().fitContent();

			// Store chart instance
			chartInstances.set(chartIndex, chart);

			// Handle resize
			const resizeObserver = new ResizeObserver(() => {
				chart.applyOptions({
					width: container.clientWidth,
					height: container.clientHeight
				});
			});
			resizeObserver.observe(container);

		} catch (error) {
			console.error('Error creating chart:', error);
		}
	}

	// Helper functions for watchlist formatting
	function formatPrice(price: number): string {
		if (typeof price !== 'number') return '--';
		return price.toFixed(2);
	}

	function formatChange(change: number): string {
		if (typeof change !== 'number') return '--';
		const sign = change >= 0 ? '+' : '';
		return `${sign}${change.toFixed(2)}`;
	}

	function formatChangePct(changePct: number): string {
		if (typeof changePct !== 'number') return '--';
		const sign = changePct >= 0 ? '+' : '';
		return `${sign}${changePct.toFixed(2)}%`;
	}

	function getChangeClass(value: number): string {
		if (typeof value !== 'number') return '';
		return value >= 0 ? 'positive' : 'negative';
	}
	function formatVolume(volume: number): string {
		if (volume < 1000000) {
			return (volume/1000).toFixed(1) + 'K';
		} else if (volume < 1000000000) {
			return (volume/1000000).toFixed(1) + 'M';
		} else {
			return (volume/1000000000).toFixed(1) + 'B';
		}
	}

	// Cleanup timeout on component destroy
	onDestroy(() => {
		if (streamingTimeout) {
			clearTimeout(streamingTimeout);
		}
		// Clear chart instances on component destroy
		chartInstances.forEach(chart => {
			try {
				chart.remove();
			} catch (e) {
				console.warn('Error removing chart on destroy:', e);
			}
		});
		chartInstances.clear();
	});
</script>

{#if isProcessingMessage}
	<div class="thinking-trace" class:no-timeline={timeline.length === 0}>
		<div class="status-header">
			<div class="current-status">
				<div class="status-icon" class:visible={timeline.length > 0}>
					<svg width="24" height="24" viewBox="0 0 20 20" fill={hasActiveBacktest ? 'none' : 'currentColor'} stroke={hasActiveBacktest ? 'currentColor' : 'none'} stroke-width={hasActiveBacktest ? '1.2' : '0'} xmlns="http://www.w3.org/2000/svg" class={hasActiveWebSearch ? 'globe-spinner' : ''}>
						{#if hasActiveWebSearch}
							<path d="M10 2.125C14.3492 2.125 17.875 5.65076 17.875 10C17.875 14.3492 14.3492 17.875 10 17.875C5.65076 17.875 2.125 14.3492 2.125 10C2.125 5.65076 5.65076 2.125 10 2.125ZM7.88672 10.625C7.94334 12.3161 8.22547 13.8134 8.63965 14.9053C8.87263 15.5194 9.1351 15.9733 9.39453 16.2627C9.65437 16.5524 9.86039 16.625 10 16.625C10.1396 16.625 10.3456 16.5524 10.6055 16.2627C10.8649 15.9733 11.1274 15.5194 11.3604 14.9053C11.7745 13.8134 12.0567 12.3161 12.1133 10.625H7.88672ZM3.40527 10.625C3.65313 13.2734 5.45957 15.4667 7.89844 16.2822C7.7409 15.997 7.5977 15.6834 7.4707 15.3486C6.99415 14.0923 6.69362 12.439 6.63672 10.625H3.40527ZM13.3633 10.625C13.3064 12.439 13.0059 14.0923 12.5293 15.3486C12.4022 15.6836 12.2582 15.9969 12.1006 16.2822C14.5399 15.467 16.3468 13.2737 16.5947 10.625H13.3633ZM12.1006 3.7168C12.2584 4.00235 12.4021 4.31613 12.5293 4.65137C13.0059 5.90775 13.3064 7.56102 13.3633 9.375H16.5947C16.3468 6.72615 14.54 4.53199 12.1006 3.7168ZM10 3.375C9.86039 3.375 9.65437 3.44756 9.39453 3.7373C9.1351 4.02672 8.87263 4.48057 8.63965 5.09473C8.22547 6.18664 7.94334 7.68388 7.88672 9.375H12.1133C12.0567 7.68388 11.7745 6.18664 11.3604 5.09473C11.1274 4.48057 10.8649 4.02672 10.6055 3.7373C10.3456 3.44756 10.1396 3.375 10 3.375ZM7.89844 3.7168C5.45942 4.53222 3.65314 6.72647 3.40527 9.375H6.63672C6.69362 7.56102 6.99415 5.90775 7.4707 4.65137C7.59781 4.31629 7.74073 4.00224 7.89844 3.7168Z"></path>
						{:else if hasActiveBacktest}
							<path stroke-linecap="round" stroke-linejoin="round" d="M8.62 3.28c.07-.45.46-.78.92-.78h.91c.46 0 .85.33.92.78l.12.74c.06.35.32.64.65.77.33.14.71.12 1-.09l.61-.44c.37-.27.88-.22 1.2.1l.64.65c.32.32.37.83.1 1.2l-.44.61c-.21.29-.23.67-.09 1 .14.33.42.59.77.65l.74.12c.45.07.78.46.78.92v.91c0 .46-.33.85-.78.92l-.74.12c-.35.06-.63.32-.77.65-.14.33-.12.71.09 1l.44.61c.27.37.22.88-.1 1.2l-.65.64c-.32.32-.83.37-1.2.1l-.61-.44c-.29-.21-.67-.23-1-.09-.33.14-.59.42-.65.77l-.12.74c-.07.45-.46.78-.92.78h-.91c-.46 0-.85-.33-.92-.78l-.12-.74c-.06-.35-.32-.64-.65-.77-.33-.14-.71-.12-1 .09l-.61.44c-.37.27-.88.22-1.2-.1l-.64-.65c-.32-.32-.37-.83-.1-1.2l.44-.61c.21-.29.23-.67.09-1-.14-.33-.42-.59-.77-.65l-.74-.12c-.45-.07-.78-.46-.78-.92v-.91c0-.46.33-.85.78-.92l.74-.12c.35-.06.63-.32.77-.65.14-.33.12-.71-.09-1l-.44-.61c-.27-.37-.22-.88.1-1.2l.65-.64c.32-.32.83-.37 1.2-.1l.61.44c.29.21.67.23 1 .09.33-.14.59-.42.65-.77l.12-.74Z"></path>
							<path stroke-linecap="round" stroke-linejoin="round" d="M12.5 10a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0Z"></path>
						{:else}
							<path d="M15.1687 8.0855C15.1687 5.21138 12.8509 2.88726 9.99976 2.88726C7.14872 2.88744 4.83179 5.21149 4.83179 8.0855C4.8318 9.91374 5.7711 11.5187 7.19019 12.4459H12.8103C14.2293 11.5187 15.1687 9.91365 15.1687 8.0855ZM8.47046 16.1099C8.72749 16.6999 9.31515 17.1127 9.99976 17.1128C10.6844 17.1128 11.2719 16.6999 11.5291 16.1099H8.47046ZM7.65894 14.7798H12.3416V13.7759H7.65894V14.7798ZM16.4988 8.0855C16.4988 10.3216 15.3777 12.2942 13.6716 13.4703V15.4449C13.6714 15.8119 13.3736 16.1098 13.0066 16.1099H12.9216C12.6187 17.4453 11.4268 18.4429 9.99976 18.4429C8.57283 18.4428 7.3807 17.4453 7.07788 16.1099H6.9939C6.62677 16.1099 6.32909 15.8119 6.32886 15.4449V13.4703C4.62271 12.2942 3.50172 10.3217 3.50171 8.0855C3.50171 4.48337 6.40777 1.55736 9.99976 1.55718C13.5919 1.55718 16.4988 4.48326 16.4988 8.0855Z"></path>
						{/if}
					</svg>	
				</div>
				{currentHeadline}
				{#if showDropdownToggle}
					<button
						class="timeline-dropdown-toggle"
						on:click={toggleDropdown}
						aria-label={showTimelineDropdown ? 'Hide timeline' : 'Show timeline'}
					>
						<svg
							width="18"
							height="18"
							viewBox="0 0 20 20"
							fill="currentColor"
							xmlns="http://www.w3.org/2000/svg"
							class="chevron-icon {showTimelineDropdown ? 'expanded' : ''}"
						>
							<path d="M7.52925 3.7793C7.75652 3.55203 8.10803 3.52383 8.36616 3.69434L8.47065 3.7793L14.2207 9.5293C14.4804 9.789 14.4804 10.211 14.2207 10.4707L8.47065 16.2207C8.21095 16.4804 7.78895 16.4804 7.52925 16.2207C7.26955 15.961 7.26955 15.539 7.52925 15.2793L12.8085 10L7.52925 4.7207L7.44429 4.61621C7.27378 4.35808 7.30198 4.00657 7.52925 3.7793Z"></path>
						</svg>
					</button>
				{/if}
			</div>
		</div>

		{#if displayedTimelineItems.length > 0}
			<div class="timeline-items" class:collapsed={!showTimelineDropdown}>
				{#each displayedTimelineItems as event, index}
					{@const actualTimelineIndex = showTimelineDropdown ? index : timeline.length - 1}
					{@const shouldAnimate = shouldAnimateTimelineItem(index)}
					<div class="timeline-item">
						<div class="timeline-dot"></div>
						<div class="timeline-content">
							{#if event.type === 'FunctionUpdate' && event.data}
								<div class="timeline-message">
									{streamedMessages.get(actualTimelineIndex) || event.data}
								</div>
							{:else if event.type === 'webSearchQuery' && event.data?.query}
								<div class="timeline-websearch">
									<div class="web-search-chip" class:animate-fade-in={shouldAnimate}>
										<svg class="search-icon" viewBox="0 0 24 24" width="18" height="18" fill="none">
											<path
												d="M21 21L16.514 16.506L21 21ZM19 10.5C19 15.194 15.194 19 10.5 19C5.806 19 2 15.194 2 10.5C2 5.806 5.806 2 10.5 2C15.194 2 19 5.806 19 10.5Z"
												stroke="currentColor"
												stroke-width="2"
												stroke-linecap="round"
												stroke-linejoin="round"
											/>
										</svg>
										<span class="search-query">{event.data.query}</span>
									</div>
								</div>
							{:else if event.type === 'webSearchCitations' && event.data?.citations}
								{@const actualIndex = showTimelineDropdown ? index : timeline.length - 1}
								{@const countData = animatedCitationCounts.get(actualIndex)}
								<div class="timeline-citations">
									<div class="citations-header">

										<span>Reading sources · {countData?.current || event.data.citations.length} </span>
									</div>
									<div class="citations-container">
										{#each event.data.citations as citation, index}
											<div class="citation-item" class:animate-fade-in={shouldAnimate}
												on:click={() => {
													if (citation.url) {
														window.open(citation.url, '_blank');
													}
												}} 
												on:keydown={(e) => {
													if (e.key === 'Enter' && citation.url) {
														window.open(citation.url, '_blank');
													}
												}} 
												role="button" 
												tabindex="0">
												{#if citation.urlIcon}
													<img 
														src={citation.urlIcon} 
														alt="Site icon" 
														class="citation-favicon"
														on:error={(e) => {
															const img = e.target;
															if (img instanceof HTMLImageElement) {
																img.style.display = 'none';
															}
														}}
													/>
												{:else}
													<div class="citation-favicon-placeholder"></div>
												{/if}
												<span class="citation-title">{citation.title || 'Untitled'}</span>
												<span class="citation-url">{citation.url ? citation.url.replace(/^https?:\/\//, '').split('/')[0].split('.').slice(0, -1).join('.') : 'Unknown URL'}</span>
											</div>
										{/each}
									</div>
								</div>
							{:else if event.type === 'getDailySnapshot' && event.data?.chartData}
								{@const originalTimelineIndex = showTimelineDropdown ? index : timeline.findIndex(item => item === event)}
								{@const ticker = event.data.ticker || ''}
								<div class="timeline-chart" class:animate-fade-in={shouldAnimate}>
									<div class="chart-container" id={`thinkingtrace-chart-${originalTimelineIndex}`}>
										<div class="chart-legend">
											<span class="ticker">{ticker}</span>
											<span>{event.data.chartData[event.data.chartData.length - 1].close.toFixed(2)}</span>
											<span>Vol: {formatVolume(event.data.chartData[event.data.chartData.length - 1].volume)}</span>
										</div>
									</div>
								</div>
							{:else if event.type === 'getWatchlistItems' && event.data}
								<div class="timeline-watchlist">
									{#if event.data && Array.isArray(event.data.tickers)}
										{@const watchlistData = event.data.tickers}
										<div class="watchlist-header">
											<svg class="watchlist-icon" viewBox="0 0 24 24" width="16" height="16" fill="none">
												<path
													d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
													stroke="currentColor"
													stroke-width="2"
													stroke-linecap="round"
													stroke-linejoin="round"
												/>
											</svg>
											<span>Reading {event.data.watchlistName || 'Watchlist'}</span>
										</div>
										<div class="watchlist-container" class:watchlist-container-animate={shouldAnimate}>
											<div class="watchlist-table">
												<div class="watchlist-table-header">
													<div class="watchlist-header-cell ticker">Ticker</div>
													<div class="watchlist-header-cell price">Price</div>
													<div class="watchlist-header-cell change">Change</div>
													<div class="watchlist-header-cell change-pct">Change %</div>
												</div>
												<div class="watchlist-table-body">
													{#each watchlistData as item, index}
														<div class="watchlist-row" class:watchlist-row-reveal={shouldAnimate} style="animation-delay: {index * 10}ms;">
															<div class="watchlist-cell ticker">
																{#if item.icon}
																	<img
																		src={item.icon}
																		alt={`${item.ticker} icon`}
																		class="watchlist-ticker-icon"
																	/>
																{:else if item.ticker}
																	<span class="watchlist-default-icon">
																		{item.ticker.charAt(0).toUpperCase()}
																	</span>
																{/if}
																<span class="watchlist-ticker-name">{item.ticker || '--'}</span>
															</div>
															<div class="watchlist-cell price">${formatPrice(item.price)}</div>
															<div class="watchlist-cell change {getChangeClass(item.change)}">
																{formatChange(item.change)}
															</div>
															<div class="watchlist-cell change-pct {getChangeClass(item.changePercent)}">
																{formatChangePct(item.changePercent)}
															</div>
														</div>
													{/each}
												</div>
											</div>
										</div>
									{:else if Array.isArray(event.data)}
										{@const watchlistData = event.data}
										<div class="watchlist-header">
											<svg class="watchlist-icon" viewBox="0 0 24 24" width="16" height="16" fill="none">
												<path
													d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
													stroke="currentColor"
													stroke-width="2"
													stroke-linecap="round"
													stroke-linejoin="round"
												/>
											</svg>
											<span>Watchlist data · {watchlistData.length} items</span>
										</div>
										<div class="watchlist-container" class:watchlist-container-animate={shouldAnimate}>
											<div class="watchlist-table">
												<div class="watchlist-table-header">
													<div class="watchlist-header-cell ticker">Ticker</div>
													<div class="watchlist-header-cell price">Price</div>
													<div class="watchlist-header-cell change">Change</div>
													<div class="watchlist-header-cell change-pct">Change %</div>
												</div>
												<div class="watchlist-table-body">
													{#each watchlistData as item, index}
														<div class="watchlist-row" class:watchlist-row-reveal={shouldAnimate} style="animation-delay: {index * 10}ms;">
															<div class="watchlist-cell ticker">
																{#if item.icon}
																	<img
																		src={item.icon}
																		alt={`${item.ticker} icon`}
																		class="watchlist-ticker-icon"
																	/>
																{:else if item.ticker}
																	<span class="watchlist-default-icon">
																		{item.ticker.charAt(0).toUpperCase()}
																	</span>
																{/if}
																<span class="watchlist-ticker-name">{item.ticker || '--'}</span>
															</div>
															<div class="watchlist-cell price">${formatPrice(item.price)}</div>
															<div class="watchlist-cell change {getChangeClass(item.change)}">
																{formatChange(item.change)}
															</div>
															<div class="watchlist-cell change-pct {getChangeClass(item.changePct)}">
																{formatChangePct(item.changePct)}
															</div>
														</div>
													{/each}
												</div>
											</div>
										</div>
									{:else if event.data && Array.isArray(event.data.items)}
										{@const watchlistData = event.data.items}
										<div class="watchlist-header">
											<svg class="watchlist-icon" viewBox="0 0 24 24" width="16" height="16" fill="none">
												<path
													d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
													stroke="currentColor"
													stroke-width="2"
													stroke-linecap="round"
													stroke-linejoin="round"
												/>
											</svg>
											<span>{event.data.name || 'Watchlist'} · {watchlistData.length} items</span>
										</div>
										<div class="watchlist-container" class:watchlist-container-animate={shouldAnimate}>
											<div class="watchlist-table">
												<div class="watchlist-table-header">
													<div class="watchlist-header-cell ticker">Ticker</div>
													<div class="watchlist-header-cell price">Price</div>
													<div class="watchlist-header-cell change">Change</div>
													<div class="watchlist-header-cell change-pct">Change %</div>
												</div>
												<div class="watchlist-table-body">
													{#each watchlistData as item, index}
														<div class="watchlist-row" class:watchlist-row-reveal={shouldAnimate} style="animation-delay: {index * 10}ms;">
															<div class="watchlist-cell ticker">
																{#if item.icon}
																	<img
																		src={item.icon}
																		alt={`${item.ticker} icon`}
																		class="watchlist-ticker-icon"
																	/>
																{:else if item.ticker}
																	<span class="watchlist-default-icon">
																		{item.ticker.charAt(0).toUpperCase()}
																	</span>
																{/if}
																<span class="watchlist-ticker-name">{item.ticker || '--'}</span>
															</div>
															<div class="watchlist-cell price">${formatPrice(item.price)}</div>
															<div class="watchlist-cell change {getChangeClass(item.change)}">
																{formatChange(item.change)}
															</div>
															<div class="watchlist-cell change-pct {getChangeClass(item.changePct)}">
																{formatChangePct(item.changePct)}
															</div>
														</div>
													{/each}
												</div>
											</div>
										</div>
									{/if}
								</div>
							{:else}
							 <span> {event.headline} </span>						
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
{/if}

<style>
	.thinking-trace {
		margin: 0.75rem 0 0 0;
		border: 1px solid rgba(255, 255, 255, 0.4);
		border-radius: 1rem;
		padding: 0.75rem;
	}

	.thinking-trace.no-timeline {
		border: 1px solid transparent;
	}

	.status-header {
		display: flex;
		align-items: center;
		margin-bottom: 0.5rem;
	}

	.current-status {
		color: transparent;
		font-size: 0.9rem;
		font-weight: 500;
		flex: 1;
		background: linear-gradient(
			90deg,
			var(--text-secondary, #aaa),
			rgba(255, 255, 255, 0.9),
			var(--text-secondary, #aaa)
		);
		background-size: 200% auto;
		background-clip: text;
		-webkit-background-clip: text;
		animation: loading-text-highlight 2.5s infinite linear;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	@keyframes loading-text-highlight {
		0% {
			background-position: 200% center;
		}
		100% {
			background-position: -200% center;
		}
	}

	.status-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		opacity: 0;
		width: 0;
		transition: opacity 0.3s ease, width 0.3s ease;
		overflow: hidden;
	}

	.status-icon.visible {
		opacity: 1;
		width: 24px;
	}

	.timeline-dropdown-toggle {
		background: none;
		border: none;
		padding: 0.25rem;
		cursor: pointer;
		color: #ffffff;
		display: flex;
		align-items: center;
		justify-content: center;
		opacity: 0.7;
		transition: opacity 0.2s ease;
		border-radius: 0.25rem;
	}

	.timeline-dropdown-toggle:hover {
		opacity: 1;
	}

	.chevron-icon {
		transition: transform 0.2s ease;
		transform: rotate(0deg); 
	}

	.chevron-icon.expanded {
		transform: rotate(90deg);
	}

	.timeline-items {
		margin-left: 0.5rem;
		margin-top: 0.5rem;
	}

	.timeline-item {
		position: relative;
		display: flex;
		align-items: flex-start;
		margin-bottom: 1rem;
		line-height: 1.4;
	}

	.timeline-item:last-child {
		margin-bottom: 0;
	}

	.timeline-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background-color: var(--text-secondary, #ccc);
		margin-right: 0.75rem;
		margin-top: 0.4rem;
		flex-shrink: 0;
		position: relative;
	}

	.timeline-dot::after {
		content: '';
		position: absolute;
		left: 50%;
		top: 100%;
		transform: translateX(-50%);
		width: 1px;
		height: 1.5rem;
		background-color: var(--text-secondary, #ccc);
		opacity: 0.3;
	}

	.timeline-item:last-child .timeline-dot::after {
		display: none;
	}

	.timeline-content {
		opacity: 0.8;
		flex: 1;
		font-size: 0.8rem;
		color: var(--text-secondary, #ccc);
		min-width: 0; /* Allows flex item to shrink */
		max-width: 100%;
	}

	/* Hide timeline dots and connecting lines when collapsed */
	.timeline-items.collapsed .timeline-dot {
		display: none;
	}

	.timeline-items.collapsed .timeline-dot::after {
		display: none;
	}

	.timeline-websearch {
		margin-top: 0.25rem;
		display: inline-block;
		width: fit-content;
	}

	.web-search-chip {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		padding: 0.25rem 0.5rem;
		background: transparent;
		border-radius: 1rem;
		font-size: 0.75rem;
		color: var(--text-primary);
		border: 1px solid rgba(255, 255, 255, 0.2);
		position: relative;
		overflow: hidden;
	}

	.web-search-chip.animate-fade-in {
		animation: fadeInUp 0.3s ease-out;
	}

	.web-search-chip::before {
		content: '';
		position: absolute;
		top: 0;
		left: 0;
		height: 100%;
		width: 0;
		background: #303030;
		border-radius: 1rem;
		animation: fillBackground 0.5s ease-out 0.1s forwards;
		z-index: -1;
	}

	@keyframes fillBackground {
		from {
			width: 0;
		}
		to {
			width: 100%;
		}
	}

	.web-search-chip .search-icon {
		flex-shrink: 0;
		opacity: 0.8;
	}

	.web-search-chip .search-query {
		font-weight: 300;
	}

	@keyframes fadeInUp {
		from {
			opacity: 0;
			transform: translateY(10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.timeline-citations {
		margin-top: 0.25rem;
		width: 100%;
		max-width: 100%;
		overflow: hidden;
	}

	.citations-header {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		margin-bottom: 0.5rem;
		font-size: 0.75rem;
		color: var(--text-secondary);
		opacity: 0.8;
	}

	.citations-container {
		max-height: 200px;
		overflow-y: auto;
		border: 1.5px solid #272929;
		border-radius: 0.5rem;
		background: #1f2121;
		width: 100%;
		max-width: 100%;
		box-sizing: border-box;
	}

	.citation-item {
		padding: 0.5rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		cursor: pointer;
		transition: background-color 0.2s ease;
		width: 100%;
		box-sizing: border-box;
		min-width: 0; /* Allows text to shrink */
		display: flex;
		align-items: center;
		gap: 0.25rem;
	}

	.citation-item.animate-fade-in {
		animation: fadeInUp 0.3s ease-out;
	}

	.citation-item:last-child {
		border-bottom: none;
	}

	.citation-item:hover {
		background-color: rgba(255, 255, 255, 0.05);
	}

	.citation-item:focus {
		outline: 1px solid var(--c-blue);
		outline-offset: -1px;
		background-color: rgba(255, 255, 255, 0.05);
	}

	.citation-title {
		font-size: 0.75rem;
		font-weight: 500;
		color: var(--text-primary);
		line-height: 1.3;
		white-space: nowrap;
		overflow: hidden;
		flex: 1;
		min-width: 0;
	}

	.citation-url {
		font-size: 0.7rem;
		color: var(--text-secondary);
		opacity: 0.7;
		font-family: monospace;
		flex-shrink: 0;
		white-space: nowrap;
	}

	.citation-favicon {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		flex-shrink: 0;
		margin-right: 0.5rem;
		object-fit: cover;
	}

	.citation-favicon-placeholder {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		flex-shrink: 0;
		margin-right: 0.5rem;
		background-color: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
	}

	/* Watchlist inline table styles */
	.timeline-watchlist {
		margin-top: 0.25rem;
		width: 100%;
		max-width: 100%;
		overflow: hidden;
	}

	.watchlist-header {
		display: flex;
		align-items: center;
		gap: 0.25rem;
		margin-bottom: 0.5rem;
		font-size: 0.75rem;
		color: var(--text-secondary);
		opacity: 0.8;
	}

	.watchlist-header .watchlist-icon {
		flex-shrink: 0;
		opacity: 0.8;
	}

	.watchlist-container {
		border: 1.5px solid #2e2e2e;
		border-radius: 0.5rem;
		background: #0f0f0f;
		width: 100%;
		max-width: 400px;
		margin: 0 auto;
		box-sizing: border-box;
		overflow: hidden;
	}

	.watchlist-container-animate {
		animation: fadeInUp 0.2s ease-out, expandContainer 0.25s ease-in;
	}

	.watchlist-table {
		width: 100%;
		display: flex;
		flex-direction: column;
	}

	.watchlist-table-header {
		display: flex;
		background: rgba(255, 255, 255, 0.02);
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		padding: 0.4rem;
	}

	.watchlist-header-cell {
		font-size: 0.8rem;
		font-weight: 500;
		color: #ffffff;
		opacity: 0.8;
		text-transform: none;
	}

	.watchlist-header-cell.ticker {
		flex: 1.2;
		min-width: 0;
		text-align: left;
	}

	.watchlist-header-cell.price,
	.watchlist-header-cell.change,
	.watchlist-header-cell.change-pct {
		flex: 0.8;
		min-width: 45px;
		text-align: right;
	}

	.watchlist-table-body {
		overflow-y: auto;
	}

	.watchlist-row {
		display: flex;
		padding: 0.4rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
		transition: background-color 0.2s ease;
	}

	.watchlist-row:last-child {
		border-bottom: none;
	}

	.watchlist-row:hover {
		background: rgba(255, 255, 255, 0.02);
	}

	.watchlist-row-reveal {
		opacity: 0;
		transform: translateY(-10px);
		animation: watchlistRowReveal 0.2s ease-in forwards;
	}

	@keyframes watchlistRowReveal {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	@keyframes expandContainer {
		from {
			max-height: 40px;
		}
		to {
			max-height: 400px;
		}
	}

	.watchlist-cell {
		font-size: 0.75rem;
		display: flex;
		align-items: center;
	}

	.watchlist-cell.ticker {
		flex: 1.2;
		min-width: 0;
		gap: 0.25rem;
		text-align: left;
	}

	.watchlist-cell.price,
	.watchlist-cell.change,
	.watchlist-cell.change-pct {
		flex: 0.8;
		min-width: 45px;
		justify-content: flex-end;
		text-align: right;
	}

	.watchlist-ticker-icon {
		width: 16px;
		height: 16px;
		border-radius: 50%;
		object-fit: cover;
		background-color: var(--ui-bg-element);
		flex-shrink: 0;
	}

	.watchlist-default-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background-color: var(--ui-border);
		color: var(--text-primary);
		font-size: 0.6rem;
		font-weight: 500;
		user-select: none;
		flex-shrink: 0;
	}

	.watchlist-ticker-name {
		font-weight: 500;
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* Change colors */
	.watchlist-cell.positive {
		color: var(--c-green, #4ade80);
	}

	.watchlist-cell.negative {
		color: var(--c-red, #f87171);
	}

	/* Custom scrollbar for watchlist table body */
	.watchlist-table-body::-webkit-scrollbar {
		width: 4px;
	}

	.watchlist-table-body::-webkit-scrollbar-track {
		background: transparent;
		border-radius: 2px;
	}

	.watchlist-table-body::-webkit-scrollbar-thumb {
		background-color: rgba(255, 255, 255, 0.1);
		border-radius: 2px;
	}

	.watchlist-table-body::-webkit-scrollbar-thumb:hover {
		background-color: rgba(255, 255, 255, 0.2);
	}

	/* Chart styles */
	.timeline-chart {
		width: 100%;
		height: 300px; /* Fixed height for the chart */
		background-color: #121212;
		border: 1.5px solid #2e2e2e;
		border-radius: 0.5rem;
		overflow: hidden;
	}

	.timeline-chart.animate-fade-in {
		animation: fadeInUp 0.3s ease-out;
	}


	.chart-container {
		width: 100%;
		height: 100%; /* Subtract header height */
	}
	.chart-legend {
		position: relative;
		background: none;
		overflow: hidden;
		color: hsl(0, 0%, 0%);
		padding: 4px 6px;
		border-radius: 4px;
		font-size: 0.4rem;
		display: flex;
		gap: 0.4rem;
		pointer-events: none;
	}
	.chart-legend .ticker {
		font-weight: 600;
	}
</style>
