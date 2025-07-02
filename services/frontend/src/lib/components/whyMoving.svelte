<script lang="ts">
	import { onMount } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { isPublicViewing } from '$lib/utils/stores/stores';

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
		progress = 0;

		// Update progress every 100ms for smooth animation
		const updateInterval = 100;
		const progressIncrement = (100 * updateInterval) / dismissTimeMs;

		progressInterval = setInterval(() => {
			progress = Math.min(100, progress + progressIncrement);
			if (progress >= 100) {
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
		if (!tkr || $isPublicViewing) return;
		try {
			const res = await privateRequest<any[]>('getWhyMoving', { tickers: [tkr] });
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
	<div class="wm-overlay" transition:fade={{ duration: 150 }}>
		<div
			class="wm-box glass glass--pill glass--responsive"
			transition:fly={{ y: position === 'top' ? -15 : 15, duration: 200 }}
		>
			<div class="wm-content">
				{content}
			</div>
			<div class="wm-progress-container">
				<div class="wm-progress-bar" style="width: {progress}%"></div>
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
		padding-top: 100px; /* Moved down from 20px */
		z-index: 1002;
	}

	.wm-box {
		background: rgba(255, 255, 255, 0.05) !important;
		backdrop-filter: blur(8px);
		border: 1px solid rgba(255, 255, 255, 0.1);
	}
	.wm-content {
		padding: 0.5rem 0.75rem;
		line-height: 1.3;
		white-space: pre-wrap;
		color: rgba(255, 255, 255, 0.98);
		text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
		font-weight: 500;
		font-size: 0.75rem;
		position: relative;
		z-index: 1;
	}

	.wm-progress-container {
		height: 3px;
		background: rgba(255, 255, 255, 0.03);
		overflow: hidden;
		position: relative;
		z-index: 1;
	}

	.wm-progress-bar {
		height: 100%;
		border-radius: var(--glass-radius); /* same corners as parent */
		/* subtle inner bevel so it looks like liquid inside glass */
		box-shadow:
			inset 0 0 8px rgba(255, 255, 255, 0.4),
			inset 0 1px 0 rgba(255, 255, 255, 0.6);

		/* shimmer animation */
		background-size: 200% 100%;
		animation: shimmer 2.4s linear infinite;
		transition: width 0.12s linear; /* keeps your existing width anim */
	}

	/* Responsive design */
	@media (max-width: 768px) {
		.wm-overlay {
			padding-top: 60px;
		}
		.wm-box {
			margin: 0 15px;
			border-radius: 0.6rem;
			max-width: min(90vw, 480px);
		}
		.wm-content {
			padding: 0.4rem 0.6rem;
			font-size: 0.7rem;
			line-height: 1.25;
		}
	}
</style>
