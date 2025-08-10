<script lang="ts">
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import '$lib/styles/splash.css';
	import { goto } from '$app/navigation';
	// Auth now comes from server via $page.data in header
	import { page } from '$app/stores';
	import { onMount } from 'svelte';

	// Use server-provided auth state for header
	$: isAuthenticated = $page.data?.isAuthenticated ?? false;

	// Invite code from URL query parameter
	let inviteCode = '';

	onMount(() => {
		// If already authenticated (server-validated), redirect immediately
		if ($page.data?.isAuthenticated) {
			goto('/app');
			return;
		}
		// Extract invite code from URL query parameter
		inviteCode = $page.url.searchParams.get('invite') || '';
	});
</script>

<SiteHeader {isAuthenticated} />

<Auth mode="login" {inviteCode} />

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
