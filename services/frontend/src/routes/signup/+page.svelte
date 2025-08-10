<script lang="ts">
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	// Auth now comes from server via $page.data in header
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	// Use server-provided auth state for header
	$: isAuthenticated = $page.data?.isAuthenticated ?? false;

	onMount(() => {
		if ($page.data?.isAuthenticated) {
			goto('/app');
		}
	});
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
