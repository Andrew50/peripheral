<script lang="ts">
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import '$lib/styles/splash.css';
	import ChipSection from '$lib/landing/ChipSection.svelte';
	import { getAuthState } from '$lib/auth';

	// Auth state - check immediately to prevent flash
	let isAuthenticated = getAuthState();
</script>

<SiteHeader {isAuthenticated} />

<Auth mode="login" />

<ChipSection />

<SiteFooter />
