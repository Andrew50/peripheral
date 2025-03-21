<script lang="ts">
	import { privateRequest } from '$lib/core/backend';
	import type { Instance } from '$lib/core/types';
	import { queryChart } from '$lib/features/chart/interface';
	import { onMount, onDestroy } from 'svelte';
	import '$lib/core/global.css';
	import { createEventDispatcher } from 'svelte';
	import { settings } from '$lib/core/stores';
	import { get } from 'svelte/store';

	const dispatch = createEventDispatcher();

	export let active = false;

	const tfs = ['1w', '1d', '1h', '1'];
	let instances: Instance[] = [];

	let loopActive = false;
	let securityIndex = 0;
	let tfIndex = 0;
	let speed = 5; //seconds

	// Inactivity timer settings
	const INACTIVITY_TIMEOUT = 5 * 60 * 1000; // 5 minutes in milliseconds
	let inactivityTimer: ReturnType<typeof setTimeout> | null = null;

	function startInactivityTimer() {
		// Only start timer if screensaver is enabled in settings
		if (!get(settings).enableScreensaver) return;

		// Clear any existing timer
		if (inactivityTimer) {
			clearTimeout(inactivityTimer);
		}

		// Set new timer
		inactivityTimer = setTimeout(() => {
			startScreensaver();
		}, INACTIVITY_TIMEOUT);
	}

	function resetInactivityTimer() {
		startInactivityTimer();
	}

	function startScreensaver() {
		// Only start if screensaver is enabled in settings
		if (!get(settings).enableScreensaver) return;

		if (!active) {
			active = true;
			if (instances.length > 0) {
				loopActive = true;
				loop();
			} else {
				// Load instances if not already loaded
				privateRequest<Instance[]>('getScreensavers', {}).then((v: Instance[]) => {
					instances = v;
					loopActive = true;
					loop();
				});
			}
		}
	}

	function stopScreensaver() {
		active = false;
		loopActive = false;
		resetInactivityTimer();
	}

	function loop() {
		if (!active || instances.length === 0) return;

		const instance = instances[securityIndex];
		instance.timeframe = tfs[tfIndex];
		queryChart(instance);
		tfIndex++;
		if (tfIndex >= tfs.length) {
			tfIndex = 0;
			securityIndex++;
			if (securityIndex >= instances.length) {
				securityIndex = 0;
			}
		}
		if (loopActive) {
			setTimeout(() => {
				loop();
			}, speed * 1000);
		}
	}

	function handleClick() {
		stopScreensaver();
		dispatch('exit');
	}

	function handleUserActivity() {
		resetInactivityTimer();
	}

	// Subscribe to settings changes
	let unsubscribe: () => void;

	onMount(() => {
		// Load instances on mount
		privateRequest<Instance[]>('getScreensavers', {}).then((v: Instance[]) => {
			instances = v;
			if (active) {
				loopActive = true;
				loop();
			}
		});

		// Set up activity listeners
		window.addEventListener('mousemove', handleUserActivity);
		window.addEventListener('mousedown', handleUserActivity);
		window.addEventListener('keypress', handleUserActivity);
		window.addEventListener('touchstart', handleUserActivity);
		window.addEventListener('scroll', handleUserActivity);

		// Subscribe to settings changes
		unsubscribe = settings.subscribe((newSettings) => {
			// If screensaver setting is disabled and screensaver is active, stop it
			if (!newSettings.enableScreensaver && active) {
				stopScreensaver();
			}

			// If screensaver setting is enabled and timer isn't running, start it
			if (newSettings.enableScreensaver && !inactivityTimer) {
				startInactivityTimer();
			}
		});

		// Start the inactivity timer if screensaver is enabled
		if (get(settings).enableScreensaver) {
			startInactivityTimer();
		}
	});

	onDestroy(() => {
		loopActive = false;

		// Clean up activity listeners
		window.removeEventListener('mousemove', handleUserActivity);
		window.removeEventListener('mousedown', handleUserActivity);
		window.removeEventListener('keypress', handleUserActivity);
		window.removeEventListener('touchstart', handleUserActivity);
		window.removeEventListener('scroll', handleUserActivity);

		// Clear the inactivity timer
		if (inactivityTimer) {
			clearTimeout(inactivityTimer);
		}

		// Unsubscribe from settings
		if (unsubscribe) {
			unsubscribe();
		}
	});
</script>

{#if active}
	<!-- Fullscreen overlay to capture clicks anywhere -->
	<div
		class="screensaver-container"
		on:click={handleClick}
		role="button"
		tabindex="0"
		on:keydown={(e) => e.key === 'Escape' && handleClick()}
	>
		<div class="screensaver-badge">
			<div class="screensaver-content">
				<div class="screensaver-title">
					Screensaver Active
					<span class="click-hint">(Click anywhere to exit)</span>
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.screensaver-container {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 1000;
		pointer-events: auto;
		cursor: pointer;
		/* Make the container transparent so it doesn't block the chart */
		background-color: transparent;
	}

	.screensaver-badge {
		position: absolute;
		top: 15px;
		right: 15px;
		background-color: var(--c2);
		padding: 8px 12px;
		border-radius: 6px;
		box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
		opacity: 0.8;
		transition:
			opacity 0.3s,
			transform 0.2s;
		max-width: 200px;
		border: 1px solid var(--c3);
		animation: pulse 2s infinite alternate;
		pointer-events: none;
	}

	@keyframes pulse {
		0% {
			box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
		}
		100% {
			box-shadow: 0 2px 15px rgba(100, 150, 255, 0.5);
		}
	}

	.screensaver-badge:hover {
		opacity: 1;
		transform: scale(1.05);
		animation: none;
	}

	.screensaver-title {
		font-size: 0.9rem;
		font-weight: bold;
		color: var(--f1);
		display: flex;
		align-items: center;
		flex-wrap: nowrap;
	}

	.screensaver-title::before {
		content: '';
		display: inline-block;
		width: 8px;
		height: 8px;
		background-color: #4caf50;
		border-radius: 50%;
		margin-right: 6px;
		animation: blink 1.5s infinite;
		flex-shrink: 0;
	}

	.click-hint {
		font-size: 0.65rem;
		font-weight: normal;
		color: var(--f2);
		margin-left: 6px;
		white-space: nowrap;
	}

	@keyframes blink {
		0%,
		100% {
			opacity: 1;
		}
		50% {
			opacity: 0.3;
		}
	}
</style>
