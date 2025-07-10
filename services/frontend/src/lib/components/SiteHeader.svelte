<script lang="ts">
	import { goto, preloadCode } from '$app/navigation';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';

	import { timelineProgress } from '$lib/landing/timeline';
	
	// Auth state prop
	export let isAuthenticated: boolean = false;
	// Visibility & transparency state
	let isHeaderVisible = true;
	let isHeaderTransparent = true;
	let prevScrollY = 0;
	import '$lib/styles/splash.css';
	function handleScroll() {
		const currentY = window.scrollY;
		const isOnSplashPage = window.location.pathname === '/';
		// Show header when at top, within 20px, or scrolling up
		if (currentY === 0 || currentY < 20 || currentY < prevScrollY) {
			isHeaderVisible = true;
		} else if (isOnSplashPage && $timelineProgress === 1) {
			isHeaderVisible = false;
		}
		// Make header transparent near the top of the page
		isHeaderTransparent =  currentY < 30 || (isOnSplashPage && $timelineProgress !== 1);
		prevScrollY = currentY;
	}

	// Navigation helpers
	function navigateTo(path: string) {
		if (!browser) return;
		if (window.location.pathname === path) {
			// Already on the desired route – just scroll to top for a snappy UX
			window.scrollTo({ top: 0, behavior: 'smooth' });
		} else {
			goto(path);
		}
	}

	onMount(() => {
		if (!browser) return;
		// Preload code for commonly visited routes to make subsequent navigations instantaneous
		const routes = ['/', '/pricing', '/login', '/signup'];
		routes.forEach((p) => preloadCode(p));

		handleScroll();
		window.addEventListener('scroll', handleScroll);
		return () => window.removeEventListener('scroll', handleScroll);
	});
	
</script>
<!-- Window scroll listener -->
<svelte:window on:scroll={handleScroll} />

<!-- Pill-style global site header reused across all pages -->
<header id="site-header" class:transparent={isHeaderTransparent} class:hidden-up={!isHeaderVisible}>
	<nav class="header-content">
		<div class="logo-section" on:click={() => navigateTo('/') } style="cursor: pointer;">
			<img src="/atlantis_logo_transparent.png" alt="Peripheral Logo" class="logo-image" />
			<p class="logo-text">Peripheral</p>
		</div>
		<div class="navigation">
			<button class="nav-button secondary" on:click={() => navigateTo('/pricing')}>Pricing</button>
			{#if isAuthenticated}
				<button class="nav-button primary" on:click={() => navigateTo('/app')}>Go to Terminal</button>
			{:else}
				<button class="nav-button secondary" on:click={() => navigateTo('/login')}>Login</button>
				<button class="nav-button primary" on:click={() => navigateTo('/signup')}>Sign up</button>
			{/if}
		</div>
	</nav>
</header>

<style>
	/* Expose required colour variables so header looks identical everywhere */
	:global(:root) {
		--color-dark: #0B2E33;
		--color-primary: #4F7C82;
		--header-h: 48px;
		--header-top: 16px;
	}

	.header-content {
		padding: 0 1rem;
		display: flex;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		height: 100%;
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		position: relative;
	}

	.logo-section {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.logo-image {
		height: 32px;
		width: auto;
		object-fit: contain;
		max-width: 140px;
	}

	.logo-text {
		color: var(--color-dark);
		font-size: 1.25rem;
		font-weight: 700;
		margin: 0;
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		letter-spacing: -0.02em;
	}

	.navigation {
		display: flex;
		gap: 0.75rem;
		align-items: center;
	}

	.nav-button {
		padding: 0.35rem 0.9rem;
		border: none;
		border-radius: 20px;
		font-size: 0.8rem;
		font-weight: 600;
		cursor: pointer;
		text-decoration: none;
		background: transparent;
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		white-space: nowrap;
		transition: all 0.15s ease;
	}

	.nav-button.secondary {
		background: transparent;
		color: #000000;
		border: 1px solid var(--color-primary);
	}

	.nav-button.primary {
		background: rgb(0, 0, 0);
		color: #f5f9ff;
	}

	.nav-button.primary:hover,
	.nav-button.secondary:hover {
		transform: translateY(-1px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
	}

	#site-header {
		position: fixed;
		top: var(--header-top);
		left: 50%;
		transform: translateX(-50%);
		width: 75vw;
		max-width: 1400px;
		height: var(--header-h);
		background: #f5f9ff;
		backdrop-filter: blur(16px);
		border: 1px solid rgba(255, 255, 255, 0.25);
		border-radius: 999px;
		transition: all 0.4s cubic-bezier(.4, 0, .2, 1);
		z-index: 1050;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		cursor: pointer;
	}

	/* Header behaviour modifiers */
	#site-header.transparent {
		background: none;
		backdrop-filter: none;
		box-shadow: none;
		border: none;
	}

	#site-header.hidden-up {
		transform: translateX(-50%) translateY(-120%);
		opacity: 0;
		pointer-events: none;
		transition: transform 0.4s cubic-bezier(.4, 0, .2, 1), opacity 0.4s cubic-bezier(.4, 0, .2, 1);
	}
	:global(*) {
			box-sizing: border-box;
	}
	/* Responsive tweaks */
	@media (max-width: 768px) {
		.header-content {
			padding: 0 1rem;
		}

		.navigation {
			gap: 0.5rem;
		}

		.nav-button {
			padding: 0.4rem 1rem;
			font-size: 0.8rem;
		}

		.logo-image {
			height: 28px;
		}

		.logo-text {
			font-size: 1.1rem;
		}
	}
</style> 