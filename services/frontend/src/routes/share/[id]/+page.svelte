<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	export let data: {
		conversationId: string;
		meta: {
			title: string;
			description: string;
			shareUrl: string;
			ogImageUrl: string;
		};
	};

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

<svelte:head>
	<!-- Basic meta tags -->
	<title>{data.meta.title} - Atlantis</title>
	<meta name="description" content={data.meta.description} />
	
	<!-- Open Graph meta tags for Facebook, LinkedIn, etc. -->
	<meta property="og:title" content={data.meta.title} />
	<meta property="og:description" content={data.meta.description} />
	<meta property="og:image" content={data.meta.ogImageUrl} />
	<meta property="og:url" content={data.meta.shareUrl} />
	<meta property="og:type" content="article" />
	<meta property="og:site_name" content="Atlantis" />
	
	<!-- Twitter Card meta tags -->
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content={data.meta.title} />
	<meta name="twitter:description" content={data.meta.description} />
	<meta name="twitter:image" content={data.meta.ogImageUrl} />
	
	<!-- Additional meta tags for better sharing -->
	<meta property="article:author" content="Atlantis" />
	<meta name="theme-color" content="#0a0a0a" />
	
	<!-- Canonical URL -->
	<link rel="canonical" href={data.meta.shareUrl} />
</svelte:head>

<!-- Hidden redirect page - no content shown during redirect -->

<style>
	:global(body) {
		background-color: var(--c1, #1a1a1a);
	}
</style> 