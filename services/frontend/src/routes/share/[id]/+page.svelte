<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';

	onMount(() => {
		if (browser) {
			// Get the conversation ID from the route parameter
			const conversationId = $page.params.id;
			
			if (conversationId) {
				// Redirect to the app page with the share parameter
				goto(`/app?share=${conversationId}`, { replaceState: true });
			} else {
				// If no ID is provided, redirect to the app page
				goto('/app', { replaceState: true });
			}
		}
	});
</script>

<!-- Hidden redirect page - no content shown during redirect -->

<style>
	:global(body) {
		background-color: var(--c1, #1a1a1a);
	}
</style> 