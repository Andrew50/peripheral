<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { isPublicViewing as isPublicViewingStore } from '$lib/utils/stores/stores';
	import type { AuthLayoutData } from '$lib/auth';
	import AlertPopup from '$lib/components/alertPopup.svelte';
	export let data: AuthLayoutData;

	// Set up client-side state based on server-provided data
	onMount(() => {
		if (!browser) return;

		// Set public viewing mode
		isPublicViewingStore.set(data.isPublicViewing);

		if (data.isAuthenticated && data.user && data.user.authToken) {
			// Set sessionStorage for client-side API calls (faster than cookies)
			// Only set if not already present to avoid overwriting newer tokens
			if (!sessionStorage.getItem('authToken')) {
				sessionStorage.setItem('authToken', data.user.authToken);
				sessionStorage.setItem('profilePic', data.user.profilePic || '');
			}
		} else if (data.isPublicViewing) {
			// Clear any existing auth data for public viewing
			sessionStorage.removeItem('authToken');
			sessionStorage.removeItem('profilePic');
		}
	});
</script>
<AlertPopup />

<slot />
