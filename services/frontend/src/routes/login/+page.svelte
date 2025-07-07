<script>
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import '$lib/styles/splash.css';
	import ChipSection from '$lib/landing/ChipSection.svelte';
</script>

<SiteHeader />

<Auth loginMenu={true} />

<ChipSection />

<SiteFooter />
