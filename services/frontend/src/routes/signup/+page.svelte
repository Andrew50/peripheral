<script lang="ts">
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import { getAuthState } from '$lib/auth';

	// Auth state - check immediately to prevent flash
	let isAuthenticated = getAuthState();
</script>

<SiteHeader {isAuthenticated} />

<Auth mode="signup" />

<style>
	/* Hide vertical scrollbar while preserving scroll */
	:global(html, body) {
		scrollbar-width: none;
	}

	:global(html::-webkit-scrollbar),
	:global(body::-webkit-scrollbar) {
		width: 0;
		height: 0;
	}
</style>
