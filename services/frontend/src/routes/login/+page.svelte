<script lang="ts">
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import '$lib/styles/splash.css';
	import { getAuthState } from '$lib/auth';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';

	// Auth state - check immediately to prevent flash
	let isAuthenticated = getAuthState();

	// Invite code from URL query parameter
	let inviteCode = '';

	onMount(() => {
		// Extract invite code from URL query parameter
		inviteCode = $page.url.searchParams.get('invite') || '';
	});
</script>

<SiteHeader {isAuthenticated} />

<Auth mode="login" {inviteCode} />
