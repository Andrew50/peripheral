<script lang="ts">
	import { onMount } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { privateRequest } from '$lib/utils/helpers/backend';

	// Props
	export let ticker: string = '';
	export let trigger: number = 0; // A monotonically increasing value from parent when a new ticker is loaded
	export let maxAgeMs = 24 * 60 * 60 * 1000; // 24 h by default
	export let position: 'top' | 'bottom' = 'top';

	let visible = false;
	let content = '';
	let timestamp = 0;
	let timeoutId: NodeJS.Timeout | null = null;
	let progressInterval: NodeJS.Timeout | null = null;
	let progress = 100; // Progress bar starts at 100% and goes to 0%
	const dismissTimeMs = 15000; // 15 seconds

	// Helper – storage key
	function makeKey(tkr: string, ts: number) {
		return `whyMoving_seen_${tkr}_${ts}`;
	}

	function hasSeen(tkr: string, ts: number) {
		if (typeof window === 'undefined') return true;
		return localStorage.getItem(makeKey(tkr, ts)) === 'true';
	}

	function markSeen(tkr: string, ts: number) {
		if (typeof window === 'undefined') return;
		localStorage.setItem(makeKey(tkr, ts), 'true');
	}

	function clearTimers() {
		if (timeoutId) {
			clearTimeout(timeoutId);
			timeoutId = null;
		}
		if (progressInterval) {
			clearInterval(progressInterval);
			progressInterval = null;
		}
	}

	function startDismissTimer() {
		clearTimers();
		progress = 100;
		
		// Update progress every 100ms for smooth animation
		const updateInterval = 100;
		const progressDecrement = (100 * updateInterval) / dismissTimeMs;
		
		progressInterval = setInterval(() => {
			progress = Math.max(0, progress - progressDecrement);
			if (progress <= 0) {
				clearTimers();
				close();
			}
		}, updateInterval);

		// Backup timeout in case interval fails
		timeoutId = setTimeout(() => {
			clearTimers();
			close();
		}, dismissTimeMs);
	}

	async function fetchWhyMoving(tkr: string) {
		if (!tkr) return;
        console.log('fetchWhyMoving', tkr);
		try {
			const res = await privateRequest<any[]>('getWhyMoving', { tickers: [tkr] });
            console.log('whyMoving', res);
			if (!Array.isArray(res) || res.length === 0) return;
			const item = res[0];
			if (!item?.content) return;

			timestamp = new Date(item.created_at).getTime();
			// Only consider recent messages
			if (Date.now() - timestamp > maxAgeMs) return;

			if (!hasSeen(tkr, timestamp)) {
				content = item.content;
				visible = true;
				markSeen(tkr, timestamp);
				startDismissTimer();
			}
		} catch (e) {
			console.error('WhyMoving fetch error:', e);
		}
	}

	// Reactive: whenever ticker or trigger changes, fetch
	$: if (trigger && ticker) {
		visible = false; // Reset first to allow animation replay
		content = '';
		clearTimers(); // Clear any existing timers
		fetchWhyMoving(ticker);
	}

	function close() {
		clearTimers();
		visible = false;
	}

	// Cleanup on component destroy
	import { onDestroy } from 'svelte';
	onDestroy(() => {
		clearTimers();
	});
</script>

{#if visible}
	<div class="wm-overlay" transition:fade={{duration:150}}>
		<div class="wm-box" transition:fly={{ y: position==='top'? -15 : 15, duration:200 }}>
			<div class="wm-content">
				{content}
			</div>
			<div class="wm-progress-container">
				<div 
					class="wm-progress-bar" 
					style="width: {progress}%"
				></div>
			</div>
		</div>
	</div>
{/if}

<style>
	.wm-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		pointer-events: none;
		display: flex;
		justify-content: center;
		padding-top: 20px; /* Positioned much higher */
		z-index: 1002;
	}
	.wm-box {
		pointer-events: auto;
		max-width: min(85vw, 550px);
		background: rgba(0, 0, 0, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 1rem;
		backdrop-filter: var(--backdrop-blur);
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
		color: #fff;
		font-size: 0.85rem;
		overflow: hidden;
	}
	.wm-content {
		padding: 0.75rem;
		line-height: 1.4;
		white-space: pre-wrap;
		color: rgba(255, 255, 255, 0.95);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
		font-weight: 400;
		font-size: 0.8rem;
	}
	
	.wm-progress-container {
		height: 3px;
		background: rgba(255, 255, 255, 0.1);
		overflow: hidden;
		position: relative;
	}
	
	.wm-progress-bar {
		height: 100%;
		background: linear-gradient(90deg, 
			var(--accent-color, #3a8bf7) 0%, 
			rgba(58, 139, 247, 0.8) 50%, 
			var(--accent-color, #3a8bf7) 100%
		);
		transition: width 0.1s linear;
		border-radius: 0 2px 2px 0;
		box-shadow: 0 0 6px rgba(58, 139, 247, 0.3);
	}
	
	/* Responsive design */
	@media (max-width: 768px) {
		.wm-overlay {
			padding-top: 10px;
		}
		.wm-box {
			margin: 0 15px;
			border-radius: 0.75rem;
			max-width: min(90vw, 450px);
		}
		.wm-content {
			padding: 0.625rem;
			font-size: 0.75rem;
		}
	}
</style>

