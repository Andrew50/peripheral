<script lang="ts">
	import { privateRequest } from '$lib/core/backend';
	import type { Instance } from '$lib/core/types';
	import { queryChart } from '$lib/features/chart/interface';
	import { onMount, onDestroy } from 'svelte';
	import '$lib/core/global.css';
	import { createEventDispatcher } from 'svelte';

	const dispatch = createEventDispatcher();

	const tfs = ['1w', '1d', '1h', '1'];
	let instances: Instance[] = [];

	let loopActive = false;
	let securityIndex = 0;
	let tfIndex = 0;
	let speed = 5; //seconds

	function loop() {
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
		dispatch('exit');
	}

	onMount(() => {
		privateRequest<Instance[]>('getScreensavers', {}).then((v: Instance[]) => {
			instances = v;
			loopActive = true;
			loop();
		});
	});

	onDestroy(() => {
		loopActive = false;
	});
</script>

<!-- Small screensaver indicator in the corner -->
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
				<span class="click-hint">(Click to exit)</span>
			</div>
			{#if instances.length > 0 && instances[securityIndex]}
				<div class="screensaver-info">
					<span class="ticker">{instances[securityIndex].symbol || 'Loading...'}</span>
					<span class="timeframe">{tfs[tfIndex]}</span>
				</div>
			{:else}
				<div class="screensaver-info">Loading charts...</div>
			{/if}
		</div>
	</div>
</div>

<style>
	.screensaver-container {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 100;
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
		margin-bottom: 4px;
		color: var(--f1);
		display: flex;
		align-items: center;
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
	}

	.click-hint {
		font-size: 0.7rem;
		font-weight: normal;
		color: var(--f2);
		margin-left: 8px;
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

	.screensaver-info {
		font-size: 0.8rem;
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.ticker {
		font-weight: bold;
		color: var(--f1);
	}

	.timeframe {
		color: var(--f2);
		background-color: var(--c3);
		padding: 2px 4px;
		border-radius: 3px;
		font-size: 0.7rem;
	}
</style>
