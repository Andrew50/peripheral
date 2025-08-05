<script lang="ts">
	import { alertPopup } from '$lib/utils/stores/stores';
	import { fade } from 'svelte/transition';
	import { queryChart } from '$lib/features/chart/interface';
	import { onMount, onDestroy } from 'svelte';

	let timeUpdateInterval: NodeJS.Timeout | null = null;
	let timeDelta = 0;

	function dismissAlert(alertId: number = 0) {
		alertPopup.set(null);
	}

	function handleAlertClick(event: MouseEvent) {
		// Don't navigate if clicking on X button
		if ((event.target as HTMLElement).closest('.close-button')) {
			return;
		}

		if ($alertPopup?.tickers && $alertPopup.tickers.length > 0) {
			const ticker = $alertPopup.tickers[0];
			if (ticker && $alertPopup.securityId) {
				queryChart({
					ticker: ticker,
					securityId: $alertPopup.securityId,
					timeframe: '1d',
					timestamp: 0,
					extendedHours: false
				});
			}
		}
		// Hide the alert popup after navigating or clicking anywhere
		dismissAlert($alertPopup.alertId);
	}

	function updateTimeDelta() {
		if ($alertPopup) {
			timeDelta = Math.round((Date.now() - $alertPopup.timestamp) / 1000);
		}
	}

	// Update time delta every second
	onMount(() => {
		if ($alertPopup) {
			updateTimeDelta();
			timeUpdateInterval = setInterval(updateTimeDelta, 1000);
		}
	});

	onDestroy(() => {
		if (timeUpdateInterval) {
			clearInterval(timeUpdateInterval);
		}
	});

	// Restart interval when alert changes
	$: if ($alertPopup) {
		if (timeUpdateInterval) {
			clearInterval(timeUpdateInterval);
		}
		updateTimeDelta();
		timeUpdateInterval = setInterval(updateTimeDelta, 1000);
	} else if (timeUpdateInterval) {
		clearInterval(timeUpdateInterval);
		timeUpdateInterval = null;
	}

	// Function to get the appropriate icon based on alert type
	function getAlertIcon(alertType: string = 'default'): string {
		return 'icon-movingcontext.svg';
	}

	// Function to get the appropriate trending icon based on alert type
	function getTrendingIcon(alertType: string = 'default'): string {
		return 'icon-movingup.svg';
	}

	// Function to get alert type from alert data
	function getAlertType(alert: any): string {
		return alert?.type || alert?.alertType || 'default';
	}
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="alert-container">
	{#if $alertPopup}
		<div class="alert-popup" transition:fade on:click={handleAlertClick}>
			<div class="alert-header">
				<div class="alert-title">
					<div class="maximize-icon">
						<img src={getAlertIcon(getAlertType($alertPopup))} alt="Alert Icon" />
					</div>
					<div class="alert-stack">
						<div class="alert-content">
							<p class="alert-message">{$alertPopup.message}</p>
							<p class="alert-metadata">
								{timeDelta}s • {new Date($alertPopup.timestamp).toLocaleTimeString()} • {$alertPopup.type}
							</p>
						</div>
						<div class="alert-buttons">
							{#if $alertPopup.tickers && Array.isArray($alertPopup.tickers)}
								{#each $alertPopup.tickers as ticker}
									<div class="alert-button">
										<div class="trending-icon">
											<img src={getTrendingIcon(getAlertType($alertPopup))} alt="Trending Icon" />
										</div>
										<p class="alert-ticker">{ticker}</p>
									</div>
								{/each}
							{/if}
						</div>
					</div>
				</div>
				<button class="close-button" on:click={() => dismissAlert($alertPopup.alertId)}>
					<svg width="12" height="12" viewBox="0 0 12 12" fill="none">
						<path
							d="M9 3L3 9M3 3L9 9"
							stroke="rgba(245, 245, 245, 0.8)"
							stroke-width="1.5"
							stroke-linecap="round"
							stroke-linejoin="round"
						/>
					</svg>
				</button>
			</div>
		</div>
	{/if}
</div>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@400;700&display=swap');

	.alert-container {
		position: fixed;
		top: clamp(10px, 2vh, 20px);
		right: clamp(10px, 2vw, 20px);
		z-index: 1000;
		display: flex;
		flex-direction: column;
		gap: clamp(5px, 1vh, 10px);
	}

	.alert-popup {
		width: clamp(260px, 30vw, 320px);
		backdrop-filter: blur(14.735342979431152px);
		background-color: rgb(214 214 214 / 30%);
		border-radius: 16px;
		padding: 16px;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	@media (width <= 768px) {
		.alert-popup {
			padding: 12px;
		}
	}

	.alert-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		width: 100%;
	}

	.alert-title {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		align-items: flex-start;
		gap: 8px;
		padding: 0;
		flex-grow: 1;
		flex-shrink: 1;
		width: 100%;
	}

	.maximize-icon {
		flex: 0 1 auto;
	}

	.maximize-icon img {
		width: 16px;
		height: 16px;
		border-radius: 3px;
		padding: 1px;
	}

	.alert-stack {
		display: flex;
		flex-direction: column;
		justify-content: flex-start;
		align-items: flex-start;
		gap: 12px;
		padding: 0;
		flex: 1 1 auto;
	}

	.alert-content {
		display: flex;
		flex-direction: column;
		justify-content: flex-start;
		align-items: flex-start;
		gap: 4px;
		padding: 0;
		flex: 0 1 auto;
	}

	.alert-message {
		flex: 0 1 auto;
		font-family: Inter, sans-serif;
		font-weight: 700;
		font-size: 14px;
		line-height: 139.9999976158142%;
		text-decoration: none;
		text-transform: none;
		color: rgb(245 245 245 / 100%);
		margin: 0;
	}

	.alert-metadata {
		flex: 0 1 auto;
		font-family: Inter, sans-serif;
		font-weight: normal;
		font-size: 12px;
		line-height: 139.9999976158142%;
		text-decoration: none;
		text-transform: none;
		color: rgb(245 245 245 / 100%);
		margin: 0;
		opacity: 0.8;
	}

	.alert-buttons {
		display: flex;
		flex-flow: row wrap;
		justify-content: flex-start;
		align-items: center;
		gap: 6px;
		padding: 0;
		flex: 0 1 auto;
	}

	.alert-button {
		display: flex;
		flex-direction: row;
		justify-content: center;
		align-items: center;
		gap: 3px;
		padding: 3px 6px;
		flex: 0 1 auto;
		background-color: rgb(0 153 81 / 100%);
		border: 1px solid rgb(20 174 92 / 100%);
		border-radius: 4px;
	}

	.trending-icon {
		flex: 0 1 auto;
	}

	.trending-icon img {
		width: 10px;
		height: 10px;
	}

	.alert-ticker {
		flex: 0 1 auto;
		font-family: Inter, sans-serif;
		font-weight: normal;
		font-size: 12px;
		line-height: 100%;
		text-decoration: none;
		text-transform: none;
		color: rgb(245 245 245 / 100%);
		margin: 0;
	}

	.close-button {
		background: none;
		border: none;
		cursor: pointer;
		padding: 2px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 2px;
		transition: background-color 0.2s ease;
		flex-shrink: 0;
	}

	.close-button:hover {
		background-color: rgb(255 255 255 / 10%);
	}

	.close-button:active {
		background-color: rgb(255 255 255 / 20%);
	}
</style>
