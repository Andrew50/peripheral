<script lang="ts">
	import { privateRequest } from '$lib/core/backend';
	import type { Instance } from '$lib/core/types';
	import { queryChart } from '$lib/features/chart/interface';
	import { onMount, onDestroy } from 'svelte';
	import '$lib/core/global.css';

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

<!-- Centered popup container -->
<div class="screensaver-popup">
	<!-- Content of the screensaver -->
	<div>Screensaver Active</div>
</div>

<style>
	.screensaver-popup {
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		background-color: var(--c1);
		padding: 20px;
		border-radius: 8px;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
		z-index: 1000;
	}
</style>
