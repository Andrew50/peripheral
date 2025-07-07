<script>
	import Auth from '$lib/components/auth.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
</script>

<SiteHeader />

<Auth loginMenu={false} />

<SiteFooter />
