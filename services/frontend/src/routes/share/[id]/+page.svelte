<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	export let data: {
		conversationId: string;
		isBot: boolean;
		meta: {
			title: string;
			description: string;
			shareUrl: string;
			ogImageUrl: string;
		};
	};

	onMount(() => {
		// Only redirect if this is a real user (not a bot) and we're in the browser
		console.log(data.isBot)
		if (browser && !data.isBot) {
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
	<title>{data.meta.title}Atlantis</title>
	<meta name="description" content={data.meta.description} />
	
	<!-- Open Graph meta tags for Facebook, LinkedIn, etc. -->
	<meta property="og:title" content={data.meta.title} />
	<meta property="og:image" content={data.meta.ogImageUrl} />
	<meta property="og:description" content={data.meta.description} />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta property="og:image:type" content="image/png" />
	<meta property="og:image:alt" content={data.meta.title} />
	<meta property="og:url" content={data.meta.shareUrl} />
	<meta property="og:type" content="website" />
	<meta property="og:site_name" content="Atlantis" />
	
	<!-- Twitter Card meta tags -->
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:site" content="@atlantis" />
	<meta name="twitter:title" content={data.meta.title} />
	<meta name="twitter:description" content={data.meta.description} />
	<meta name="twitter:image" content={data.meta.ogImageUrl} />
	<meta name="twitter:image:alt" content={data.meta.title} />
	
	<!-- Additional meta tags for better sharing -->
	<meta property="article:author" content="Atlantis" />
	<meta name="theme-color" content="#0a0a0a" />
	
</svelte:head>


<style>
	:global(body) {
		background-color: var(--c1, #1a1a1a);
	}
</style> 