<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { startPricingPreload } from '$lib/utils/pricing-loader';

	onMount(() => {
		if (!browser) return;
		startPricingPreload();
	});
</script>

<slot />
