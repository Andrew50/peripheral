<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount, onDestroy } from 'svelte';
	import { startPricingPreload } from '$lib/utils/pricing-loader';
	import { createChart } from 'lightweight-charts';
	import { fade } from 'svelte/transition';
	import type { TimelineEvent } from '$lib/landing/timeline';
	import { createTimelineEvents, sampleQuery, totalScroll } from '$lib/landing/timeline';
	import { timelineProgress } from '$lib/landing/timeline';
	import { get } from 'svelte/store';
	import PlotChunk from '$lib/features/chat/components/PlotChunk.svelte';
	import { isPlotData, getPlotData, generatePlotKey } from '$lib/features/chat/plotUtils';

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
	// Server-injected data (bars for SPY) forwarded from route
	export let defaultChartData: any = undefined;

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

	let chatMessages: ChatMessage[] = [];

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

	function addAssistantMessage(text: string) {
		const msg: ChatMessage = {
			message_id: 'hero_' + Date.now() + '_' + Math.random().toString(36).slice(2, 7),
			sender: 'assistant',
			text,
			contentChunks: [{ type: 'text', content: text }]
		};
		chatMessages = [...chatMessages, msg];
	}

	function removeLastMessage() {
		chatMessages = chatMessages.slice(0, -1);
	}

	const timelineEvents: TimelineEvent[] = createTimelineEvents({
		addUserMessage: addMessage,
		addAssistantMessage,
		removeLastMessage
	});


	let heroWrapper: HTMLElement;

	function updateHeroProgress() {
		if (!heroWrapper) return;
		const rect = heroWrapper.getBoundingClientRect();
		const travelled = Math.min(Math.max(-rect.top, 0), totalScroll);
		timelineProgress.set(travelled / totalScroll);
		evaluateTimeline();
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
            animationPhase='complete';
        }, 600);
		setTimeout(() => {
			animationPhase = 'complete';
			showHeroContent = true;
		}, 800);

	}
    function setData(data: any) {
        if (data.chartData?.bars?.length) {
            const serverBars = data.chartData.bars.map((bar: any) => ({
                time: bar.time as any,
                open: bar.open,
                high: bar.high,
                low: bar.low,
                close: bar.close,
                volume: bar.volume ?? bar.v ?? bar.vol ?? 0
            }));
        }
    }
	// ------- Hero chart legend state ---------
	const currentTicker = 'SPY';
	interface LegendData {
		open: number;
		high: number;
		low: number;
		close: number;
		volume: number;
	}
	let legendData: LegendData = { open: 0, high: 0, low: 0, close: 0, volume: 0 };

	// Chart & scroll setup
	onMount(() => {
		if (!browser) return;
        document.documentElement.style.setProperty('--hero-total-scroll', `${totalScroll}px`);
		updateHeroProgress();
		window.addEventListener('scroll', updateHeroProgress, { passive: true });
		startPricingPreload();

		// Set loaded flag for fade-in
		isLoaded = true;
		setTimeout(() => startTypewriterEffect(), 0);
		evaluateTimeline();

		if (!chartContainerRef) return;

		/* ----------------------------
		   Initialise lightweight-chart
		   ---------------------------- */
		const chart = createChart(chartContainerRef, {
			width: chartContainerRef.clientWidth,
			height: chartContainerRef.clientHeight,
			layout: { background: { color: 'transparent' }, textColor: '#0B2E33', attributionLogo: false },
			grid: {
				vertLines: { visible: true, color: 'rgba(11, 46, 51, 0.15)', style: 1 },
				horzLines: { visible: true, color: 'rgba(11, 46, 51, 0.15)', style: 1 }
			},
			timeScale: { visible: true },
			handleScroll: { mouseWheel: false, pressedMouseMove: true, horzTouchDrag: true, vertTouchDrag: true },
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
		volumeSeries.priceScale().applyOptions({ scaleMargins: { top: 0.9, bottom: 0 }, visible: false });

		/* ----- Generate placeholder sine-wave data ----- */
		const now = Math.floor(Date.now() / 1000);
		const candleData = Array.from({ length: 200 }, (_, i) => {
			const base = 100 + Math.sin(i / 4) * 3;
			const open = base + (Math.random() - 0.5) * 2;
			const close = open + (Math.random() - 0.5) * 4;
			const high = Math.max(open, close) + Math.random() * 2;
			const low = Math.min(open, close) - Math.random() * 2;
			const volume = Math.floor(1000 + Math.random() * 9000);

			return { time: (now - (200 - i) * 60) as any, open, high, low, close, volume };
		});
		candleSeries.setData(candleData);
		volumeSeries.setData(
			candleData.map((bar: any) => ({
				time: bar.time,
				value: bar.volume,
				color: bar.close >= bar.open ? '#26a69a' : '#ef5350'
			}))
		);

		// Inject real SPY data if provided by server
		if (defaultChartData?.chartData?.bars?.length) {
			const serverBars = defaultChartData.chartData.bars.map((bar: any) => ({
				time: bar.time as any,
				open: bar.open,
				high: bar.high,
				low: bar.low,
				close: bar.close,
				volume: bar.volume ?? bar.v ?? bar.vol ?? 0
			}));
			candleSeries.setData(serverBars);
			volumeSeries.setData(
				serverBars.map((bar: any) => ({
					time: bar.time,
					value: bar.volume,
					color: bar.close >= bar.open ? '#26a69a' : '#ef5350'
				}))
			);
			if (serverBars.length) {
				const lastBar = serverBars[serverBars.length - 1];
				legendData = { open: lastBar.open, high: lastBar.high, low: lastBar.low, close: lastBar.close, volume: lastBar.volume } as any;
			}
		}

		// Subscribe to crosshair for reactive legend updates
		chart.subscribeCrosshairMove((param) => {
			if (!param || !param.seriesData) return;
			const bar = param.seriesData.get(candleSeries);
			const volBar = param.seriesData.get(volumeSeries);
			if (bar && typeof bar === 'object' && 'open' in bar) {
				const volumeVal = volBar && typeof volBar === 'object' && 'value' in volBar ? (volBar as any).value : legendData.volume;
				legendData = { open: (bar as any).open, high: (bar as any).high, low: (bar as any).low, close: (bar as any).close, volume: volumeVal } as any;
			}
		});

		new ResizeObserver(() => {
			chart.applyOptions({
				width: chartContainerRef!.clientWidth,
				height: chartContainerRef!.clientHeight
			});
		}).observe(chartContainerRef);
	});

	onDestroy(() => {
		if (!browser) return;
		window.removeEventListener('scroll', updateHeroProgress);
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
            <div class="animation-input-container" class:typing={animationPhase === 'typing'} class:submitted={animationPhase === 'submitted'} class:complete={animationPhase === 'complete'}>
                <div class="animation-input-wrapper">
                    <textarea
                        class="animation-input"
                        bind:value={animationInput}
                        bind:this={animationInputRef}
                        readonly
                        rows="1"
                        class:typing-cursor={animationPhase === 'typing'}
                    ></textarea>
                    <button
                        class="animation-send"
                        class:pulse={animationPhase === 'submitted'}
                    >
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
                    <div class="hero-chat-messages" class:has-messages={chatMessages.length > 0}>
                        {#if chatMessages.length !== 0}
                            {#each chatMessages as msg (msg.message_id)}
                                <div in:fade={{ duration: 200 }} out:fade={{ duration: 200 }} class="message-wrapper {msg.sender}">
                                    {#if msg.sender === 'user'}
                                        <div class="message user">
                                            <div class="message-content">
                                                <p>{msg.text}</p>
                                            </div>
                                        </div>
                                    {:else}
                                        {#if msg.contentChunks && msg.contentChunks.length > 0}
                                            <div class="assistant-message">
                                                {#each msg.contentChunks as chunk, idx}
                                                    {#if chunk.type === 'text'}
                                                        <p>{@html typeof chunk.content === 'string' ? chunk.content : String(chunk.content)}</p>
                                                    {:else if chunk.type === 'table'}
                                                        {#if isTableData(chunk.content)}
                                                            {@const tableData = chunk.content}
                                                            <div class="assistant-table">
                                                                <table>
                                                                    {#if tableData.caption}
                                                                        <caption>{@html tableData.caption}</caption>
                                                                    {/if}
                                                                    <thead>
                                                                        <tr>
                                                                            {#each tableData.headers as header}
                                                                                <th>{@html header}</th>
                                                                            {/each}
                                                                        </tr>
                                                                    </thead>
                                                                    <tbody>
                                                                        {#each tableData.rows as row}
                                                                            <tr>
                                                                                {#each row as cell}
                                                                                    <td>{@html typeof cell === 'string' ? cell : String(cell)}</td>
                                                                                {/each}
                                                                            </tr>
                                                                        {/each}
                                                                    </tbody>
                                                                </table>
                                                            </div>
                                                        {:else}
                                                            <p>Invalid table data</p>
                                                        {/if}
                                                    {:else if chunk.type === 'plot'}
                                                        {#if isPlotData(chunk.content)}
                                                            {@const plotData = getPlotData(chunk.content)}
                                                            {#if plotData}
                                                                <PlotChunk {plotData} plotKey={generatePlotKey(msg.message_id, idx)} />
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
                                            <p class="assistant-message">{msg.text}</p>
                                        {/if}
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
        content: "";
        position: absolute;
        inset: 0;
        border-radius: 28px; /* match input wrapper radius */
        pointer-events: none;
        border: 2px solid var(--color-primary);
        box-shadow: 0 0 0 0 rgba(79,124,130,0.6);
        animation: ringPulse 1s ease-out forwards;
    }
    @keyframes ringPulse {
        /* Faster expansion (first 0.2s), longer hold (0.5s), then fade */
        0%   { box-shadow: 0 0 0 0 rgba(79,124,130,0.6);   opacity: 1; }
        20%  { box-shadow: 0 0 0 8px rgba(79,124,130,0.6); opacity: 1; }
        70%  { box-shadow: 0 0 0 8px rgba(79,124,130,0.6); opacity: 1; }
        100% { box-shadow: 0 0 0 8px rgba(79,124,130,0);  opacity: 0; }
    }

    /* Fade the entire container out once the ring pulse finishes */
    @keyframes containerFadeOut {
        0%   { opacity: 1; transform: scale(1); }
        100% { opacity: 0; transform: scale(0.95); }
    }

    /* hero stuff */
    .hero-title {
			font-size: clamp(2.7rem, 4vw, 5rem);
			font-weight: 800;
			margin: 0 0 1.5rem 0;
			letter-spacing: -0.02em;
			line-height: 1.1;
			color: var(--color-dark);
			text-shadow: 0 2px 12px rgba(0,0,0,0.2), 0 1px 0 rgba(255,255,255,0.01);
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
        :root { --hero-widget-h: 220px; }
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
        content: "";
        position: absolute;
        inset: 0;
        pointer-events: none;
        z-index: -1;
        /* Brighter hue – using primary brand colour */
        --halo-rgb: 79,124,130;
        /* Inner colour wash */
        background: radial-gradient(ellipse at 50% 50%,
            rgba(var(--halo-rgb),0.55) 0%,
            rgba(var(--halo-rgb),0.25) 45%,
            rgba(var(--halo-rgb),0.00) 70%);
        /* Concentric steps */
        box-shadow:
            0 0 0 48px rgba(var(--halo-rgb),0.15),
            0 0 0 96px rgba(var(--halo-rgb),0.10),
            0 0 0 144px rgba(var(--halo-rgb),0.07),
            0 0 0 192px rgba(var(--halo-rgb),0.04),
            0 0 0 240px rgba(var(--halo-rgb),0.02);
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
        0%, 50% { opacity: 1; }
        51%, 100% { opacity: 0; }
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
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 1rem;
        min-height: 120px;
        flex: 1;
        justify-content: center;
        align-items: center;
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
        :root { --hero-widget-h: 220px; }
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
        max-width: 80%;
        text-align: left;
    }

    /* Force Inter font inside chat */
    .hero-chat-container, .hero-chat-container * {
        font-family: 'Geist';
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
        height: calc(100vh + var(--hero-total-scroll));   /* 100 vh +  totalScroll (1500 px) */
        position: relative;
    }
    .hero-pin{
        position: sticky;   /*  or 'fixed: top:0; left:0; width:100%' */
        top: 0;
        height: 100vh;      /*  fills the viewport while pinned  */
    }
</style>