<script lang="ts">
	import { alertPopup } from '$lib/utils/stores/stores';
	import { fade } from 'svelte/transition';

	function dismissAlert(alertId: number = 0) {
		alertPopup.set(null);
	}

	// Function to get the appropriate icon based on alert type
	function getAlertIcon(alertType: string = 'default'): string {
		const iconMap: Record<string, string> = {
			price: 'icon-price-alert.svg',
			strategy: 'icon-strategy-alert.svg',
			triggered: 'icon-triggered-alert.svg',
			movingContext: 'icon-movingcontext.svg',
			default: 'icon-movingcontext.svg'
		};
		return iconMap[alertType] || iconMap['default'];
	}
 
	// Function to get the appropriate trending icon based on alert type
	function getTrendingIcon(alertType: string = 'default'): string {
		const iconMap: Record<string, string> = {
			price: 'icon-price-trend.svg',
			strategy: 'icon-strategy-trend.svg',
			triggered: 'icon-movingup.svg',
			movingContext: 'icon-movingup.svg',
			default: 'icon-movingup.svg'
		};
		return iconMap[alertType] || iconMap['default'];
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
		<div class="alert-popup" transition:fade on:click={() => dismissAlert($alertPopup.alertId)}>
			<div class="alert-title">
				<div class="maximize-icon">
					<img src={getAlertIcon(getAlertType($alertPopup))} alt="Alert Icon" />
				</div>
				<div class="alert-stack">
					<div class="alert-content">
						<p class="alert-message">{$alertPopup.message}</p>
						<p class="alert-metadata">
							{Math.round((Date.now() - $alertPopup.timestamp) / 1000)}s • {new Date(
								$alertPopup.timestamp
							).toLocaleTimeString()} • {$alertPopup.type}
						</p>
					</div>
					<div class="alert-buttons">
						{#each $alertPopup.tickers as ticker}
							<div class="alert-button">
								<div class="trending-icon">
									<img src={getTrendingIcon(getAlertType($alertPopup))} alt="Trending Icon" />
								</div>
								<p class="alert-ticker">{ticker}</p>
							</div>
						{/each}
					</div>
				</div>
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
		background-color: rgba(214, 214, 214, 0.3);
		border-radius: 16px;
		padding: 16px;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	@media (max-width: 768px) {
		.alert-popup {
			padding: 12px;
		}
	}

	.alert-title {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		align-items: flex-start;
		gap: 8px;
		padding: 0px;
		flex-grow: 1;
		flex-shrink: 1;
		width: 100%;
	}

	.maximize-icon {
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
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
		padding: 0px;
		flex-grow: 1;
		flex-shrink: 1;
		flex-basis: auto;
	}

	.alert-content {
		display: flex;
		flex-direction: column;
		justify-content: flex-start;
		align-items: flex-start;
		gap: 4px;
		padding: 0px;
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
	}

	.alert-message {
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
		font-family: 'Inter', sans-serif;
		font-weight: 700;
		font-size: 14px;
		line-height: 139.9999976158142%;
		text-decoration: none;
		text-transform: none;
		color: rgba(245, 245, 245, 1);
		margin: 0;
	}

	.alert-metadata {
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
		font-family: 'Inter', sans-serif;
		font-weight: normal;
		font-size: 12px;
		line-height: 139.9999976158142%;
		text-decoration: none;
		text-transform: none;
		color: rgba(245, 245, 245, 1);
		margin: 0;
		opacity: 0.8;
	}

	.alert-buttons {
		display: flex;
		flex-direction: row;
		justify-content: flex-start;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
		padding: 0px;
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
	}

	.alert-button {
		display: flex;
		flex-direction: row;
		justify-content: center;
		align-items: center;
		gap: 3px;
		padding: 3px 6px;
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
		background-color: rgba(0, 153, 81, 1);
		border: 1px solid rgba(20, 174, 92, 1);
		border-radius: 4px;
	}

	.trending-icon {
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
	}

	.trending-icon img {
		width: 10px;
		height: 10px;
	}

	.alert-ticker {
		flex-grow: 0;
		flex-shrink: 1;
		flex-basis: auto;
		font-family: 'Inter', sans-serif;
		font-weight: normal;
		font-size: 12px;
		line-height: 100%;
		text-decoration: none;
		text-transform: none;
		color: rgba(245, 245, 245, 1);
		margin: 0;
	}
</style>
