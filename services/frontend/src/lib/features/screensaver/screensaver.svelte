<!-- 
DEPRECATED: This screensaver component has been deprecated and is no longer in use.
All screensaver functionality has been commented out in the frontend.
-->

<!--
<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import type { Instance } from '$lib/utils/types/types';
	import { queryChart } from '$lib/features/chart/interface';
	import { onMount, onDestroy } from 'svelte';
	import '$lib/styles/global.css';
	import { createEventDispatcher } from 'svelte';
	import { settings } from '$lib/utils/stores/stores';
	import { get } from 'svelte/store';

	const dispatch = createEventDispatcher();

	export let active = false;

	let instances: Instance[] = [];
	let currentSettings = get(settings);

	let loopActive = false;
	let securityIndex = 0;
	let tfIndex = 0;

	// Inactivity timer
	let inactivityTimer: ReturnType<typeof setTimeout> | null = null;

	function startInactivityTimer() {
		// Only start timer if screensaver is enabled in settings
		if (!get(settings).enableScreensaver) return;

		// Clear any existing timer
		if (inactivityTimer) {
			clearTimeout(inactivityTimer);
		}

		// Set new timer using the configured timeout value (convert seconds to milliseconds)
		inactivityTimer = setTimeout(
			() => {
				startScreensaver();
			},
			get(settings).screensaverTimeout * 1000
		);
	}

	function resetInactivityTimer() {
		startInactivityTimer();
	}

	function startScreensaver() {
		// Only start if screensaver is enabled in settings
		if (!get(settings).enableScreensaver) return;

		if (!active) {
			active = true;
			// Update current settings
			currentSettings = get(settings);

			if (instances.length > 0) {
				loopActive = true;
				loop();
			} else {
				// Load instances based on data source
				loadInstances();
			}
		}
	}

	function loadInstances() {
		const dataSource = currentSettings.screensaverDataSource;

		if (dataSource === 'gainers-losers') {
			// Use the existing getScreensavers endpoint
			privateRequest<Instance[]>('getScreensavers', {}).then((v: Instance[]) => {
				instances = v;
				loopActive = true;
				loop();
			});
		} else if (dataSource === 'watchlist' && currentSettings.screensaverWatchlistId) {
			// Load from specified watchlist
			privateRequest<Instance[]>('getWatchlistItems', {
				watchlistId: currentSettings.screensaverWatchlistId
			}).then((v: Instance[]) => {
				instances = v;
				loopActive = true;
				loop();
			});
		} else if (
			dataSource === 'user-defined' &&
			currentSettings.screensaverTickers &&
			currentSettings.screensaverTickers.length > 0
		) {
			// Load from user-defined tickers
			privateRequest<Instance[]>('getInstancesByTickers', {
				tickers: currentSettings.screensaverTickers
			}).then((v: Instance[]) => {
				instances = v;
				loopActive = true;
				loop();
			});
		} else {
			// Fallback to gainers-losers if configuration is invalid
			privateRequest<Instance[]>('getScreensavers', {}).then((v: Instance[]) => {
				instances = v;
				loopActive = true;
				loop();
			});
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
		const timeframes = currentSettings.screensaverTimeframes;

		// Skip if we don't have valid timeframes
		if (!timeframes || timeframes.length === 0) {
			securityIndex = (securityIndex + 1) % instances.length;
			setTimeout(() => {
				loop();
			}, currentSettings.screensaverSpeed * 1000);
			return;
		}

		instance.timeframe = timeframes[tfIndex];
		queryChart(instance);

		tfIndex++;
		if (tfIndex >= timeframes.length) {
			tfIndex = 0;
			securityIndex++;
			if (securityIndex >= instances.length) {
				securityIndex = 0;
			}
		}

		if (loopActive) {
			setTimeout(() => {
				loop();
			}, currentSettings.screensaverSpeed * 1000);
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
		// Subscribe to settings changes
		unsubscribe = settings.subscribe((newSettings) => {
			// Update our local copy of the settings
			currentSettings = newSettings;

			// If screensaver setting is disabled and screensaver is active, stop it
			if (!newSettings.enableScreensaver && active) {
				stopScreensaver();
			}

			// If screensaver setting is enabled and timer isn't running, start it
			if (newSettings.enableScreensaver && !inactivityTimer) {
				startInactivityTimer();
			}
		});

		// Load instances on mount if screensaver active
		if (active) {
			loadInstances();
		}

		// Set up activity listeners
		window.addEventListener('mousemove', handleUserActivity);
		window.addEventListener('mousedown', handleUserActivity);
		window.addEventListener('keypress', handleUserActivity);
		window.addEventListener('touchstart', handleUserActivity);
		window.addEventListener('scroll', handleUserActivity);

		// Start the inactivity timer if screensaver is enabled
		if (currentSettings.enableScreensaver) {
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
-->
