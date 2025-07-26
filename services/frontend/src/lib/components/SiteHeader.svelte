<script lang="ts">
	import { goto, preloadCode } from '$app/navigation';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	import { page } from '$app/stores';


	// Auth state prop
	export let isAuthenticated: boolean = false;
	// Visibility & transparency state
	let isHeaderVisible = true;
	let isHeaderTransparent = true;
	let prevScrollY = 0;
	// Mobile sidebar state
	let isSidebarOpen = false;
	let isMobile = false;

	import '$lib/styles/splash.css';

	function handleScroll() {
		if (!browser) return;
		const currentY = window.scrollY;
		
		// Show header when at top, within 20px, or scrolling up
		if (currentY === 0 || currentY < 20 || currentY < prevScrollY) {
			isHeaderVisible = true;
		} else {
			isHeaderVisible = false;
		}
		isHeaderTransparent = currentY < 30;


		prevScrollY = currentY;
	}

	// Navigation helpers
	function navigateTo(path: string) {
		if (!browser) return;
		// Close sidebar on navigation
		isSidebarOpen = false;
		if (window.location.pathname === path) {
			// Already on the desired route – just scroll to top for a snappy UX
			window.scrollTo({ top: 0, behavior: 'smooth' });
		} else {
			goto(path);
		}
	}

	function toggleSidebar() {
		isSidebarOpen = !isSidebarOpen;
	}

	function closeSidebar() {
		isSidebarOpen = false;
	}

	function handleResize() {
		if (!browser) return;
		const wasMobile = isMobile;
		isMobile = window.innerWidth < 768;
		
		// Close sidebar when transitioning from mobile to desktop
		if (wasMobile && !isMobile && isSidebarOpen) {
			isSidebarOpen = false;
		}
	}

	onMount(() => {
		if (!browser) return;
		// Preload code for commonly visited routes to make subsequent navigations instantaneous
		const routes = ['/', '/pricing', '/login', '/signup'];
		routes.forEach((p) => preloadCode(p));

		// Initial scroll state setup
		handleScroll();
		// Initial mobile state setup
		isMobile = window.innerWidth < 768;
	});
</script>

<!-- Window scroll and resize listeners -->
<svelte:window on:scroll={handleScroll} on:resize={handleResize} />

<!-- Mobile sidebar overlay -->
{#if isSidebarOpen && isMobile}
	<div
		class="sidebar-overlay"
		on:click={closeSidebar}
		on:keydown={(e) => e.key === 'Escape' && closeSidebar()}
		role="button"
		tabindex="0"
		aria-label="Close sidebar"
	></div>
{/if}

<!-- Mobile sidebar -->
<div class="sidebar" class:open={isSidebarOpen && isMobile}>
	<div class="sidebar-header">
		<div class="logo-section">
			<img src={isHeaderTransparent ? "/favicon.png" : "/favicon-black.png"} alt="Peripheral Logo" class="logo-image" />
			<p class="logo-text">Peripheral</p>
		</div>
		<button class="close-button" on:click={closeSidebar}>
			<svg width="24" height="24" viewBox="0 0 24 24" fill="none">
				<path
					d="M18 6L6 18M6 6L18 18"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
				/>
			</svg>
		</button>
	</div>
	<nav class="sidebar-nav">
		<button class="sidebar-nav-button secondary" on:click={() => navigateTo('/pricing')}>Pricing</button>
		{#if isAuthenticated}
			<button class="sidebar-nav-button primary" on:click={() => navigateTo('/app')}
				>Go to Terminal</button
			>
		{:else}
			<button class="sidebar-nav-button primary" on:click={() => navigateTo('/login')}>Login</button>
			<button class="sidebar-nav-button primary" on:click={() => navigateTo('/signup')}
				>Sign up</button
			>
		{/if}
	</nav>
</div>

<!-- Pill-style global site header reused across all pages -->
<header id="site-header" class:transparent={isHeaderTransparent} class:hidden-up={!isHeaderVisible}>
	<nav class="header-content">
		<button
			class="logo-section"
			on:click={() => navigateTo('/')}
			on:keydown={(e) => e.key === 'Enter' && navigateTo('/')}
			aria-label="Go to home page"
			style="cursor: pointer; background: none; border: none; padding: 0;"
		>
			<img src={isHeaderTransparent ? "/favicon.png" : "/favicon-black.png"} alt="Peripheral Logo" class="logo-image" />
			<p class="logo-text" class:transparent={isHeaderTransparent}>Peripheral</p>
		</button>

		<!-- Desktop navigation -->
		<div class="navigation desktop-nav">
			<button class="nav-button secondary"  class:transparent={isHeaderTransparent} on:click={() => navigateTo('/pricing')}>Pricing</button>
			{#if isAuthenticated}
				<button class="nav-button primary" class:transparent={isHeaderTransparent} on:click={() => navigateTo('/app')}
					>Go to Terminal</button
				>
			{:else}
				<button class="nav-button secondary" class:transparent={isHeaderTransparent} on:click={() => navigateTo('/login')}>Login</button>
				<button class="nav-button primary" class:transparent={isHeaderTransparent} on:click={() => navigateTo('/signup')}>Sign up</button>
			{/if}
		</div>

		<!-- Mobile hamburger menu -->
		<button class="hamburger-menu mobile-only" class:transparent={isHeaderTransparent} on:click={toggleSidebar}>
			<div class="hamburger-line"></div>
			<div class="hamburger-line"></div>
			<div class="hamburger-line"></div>
		</button>
	</nav>
</header>

<style>
	/* Expose required colour variables so header looks identical everywhere */
	:global(:root) {
		--color-dark: #0b2e33;
		--color-primary: #4f7c82;
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
		font-family:
			'Geist',
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		position: relative;
	}

	.logo-section {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.logo-image {
		height: 28px;
		width: auto;
		object-fit: contain;
		max-width: 140px;
	}

	.logo-text {
		color: #000000;
		font-size: 1.25rem;
		font-weight: 700;
		margin: 0;
		font-family:
			'Geist',
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		letter-spacing: -0.02em;
		transition: color 0.4s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.logo-text.transparent {
		color: #ffffff;
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
		font-family:
			'Geist',
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		white-space: nowrap;
		transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.nav-button.secondary {
		color: #000000;
		border: 1px solid #000000;
	}

	.nav-button.primary {
		background: rgb(0, 0, 0);
		color: #ffffff;
	}

	.nav-button.primary:hover,
	.nav-button.secondary:hover {
		transform: translateY(-1px);
		transition: none;
	}

	/* Transparent header styles */
	.nav-button.secondary.transparent {
		color: #ffffff;
		border: 1px solid #ffffff;
	}

	.nav-button.primary.transparent {
		background: #ffffff;
		color: #000000;
	}

	.nav-button.primary.transparent:hover,
	.nav-button.secondary.transparent:hover {
		transform: translateY(-1px);
		transition: none;
	}


	/* Hamburger Menu */
	.hamburger-menu {
		display: none;
		flex-direction: column;
		justify-content: space-between;
		width: 18px;
		height: 14px;
		background: none;
		border: none;
		cursor: pointer;
		padding: 0;
	}

	.hamburger-line {
		width: 100%;
		height: 1.5px;
		background-color: #000000;
		border-radius: 1px;
		transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.hamburger-menu.transparent .hamburger-line {
		background-color: #ffffff;
	}

	/* Sidebar */
	.sidebar-overlay {
		position: fixed;
		top: 0;
		left: 0;
		width: 100vw;
		height: 100vh;
		background: rgba(0, 0, 0, 0.5);
		z-index: 1999;
	}

	.sidebar {
		position: fixed;
		top: 0;
		right: -100vw;
		width: 100vw;
		height: 100vh;
		background: #ffffff;
		z-index: 2000;
		transition: right 0.4s cubic-bezier(0.4, 0, 0.2, 1);
		box-shadow: -4px 0 20px rgba(0, 0, 0, 0.1);
	}

	/* Hide sidebar completely on desktop to prevent any flashing */
	@media (min-width: 769px) {
		.sidebar {
			display: none !important;
		}
		.sidebar-overlay {
			display: none !important;
		}
	}

	.sidebar.open {
		right: 0;
	}

	.sidebar-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1.5rem 1.5rem 1rem 1.5rem;
		border-bottom: none;
	}

	.close-button {
		background: none;
		border: none;
		cursor: pointer;
		padding: 0.5rem;
		color: #000;
		width: 40px;
		height: 40px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.sidebar-nav {
		display: flex;
		flex-direction: column;
		padding: 2rem 1.5rem;
		gap: 2rem;
	}

	.sidebar-nav-button {
		padding: 0;
		border: none;
		border-radius: 0;
		font-size: 1.25rem;
		font-weight: 400;
		cursor: pointer;
		text-decoration: none;
		background: transparent;
		font-family:
			'Geist',
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		text-align: left;
		transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
		color: #000;
		line-height: 1.4;
	}

	.sidebar-nav-button.primary {
		background: transparent;
		color: #000;
		border: none;
		font-weight: 400;
	}

	.sidebar-nav-button:hover {
		color: #666;
		background: none;
	}

	.sidebar-nav-button.primary:hover {
		color: #666;
		background: none;
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
		transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
		z-index: 1050;
		box-shadow: 0 4px 20px rgba(0, 0, 0, 0.1);
		font-family:
			'Geist',
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
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
		transition:
			transform 0.4s cubic-bezier(0.4, 0, 0.2, 1),
			opacity 0.4s cubic-bezier(0.4, 0, 0.2, 1);
	}

	:global(*) {
		box-sizing: border-box;
	}

	/* Mobile-specific styles */
	.mobile-only {
		display: none;
	}

	.desktop-nav {
		display: flex;
	}

	/* Responsive tweaks */
	@media (max-width: 768px) {
		/* Mobile header becomes rectangle */
		#site-header {
			top: 0;
			left: 0;
			transform: none;
			width: 100vw;
			max-width: none;
			height: 60px;
			border-radius: 0;
			background: #f5f9ff;
			backdrop-filter: blur(16px);
			border: none;
			border-bottom: 1px solid rgba(0, 0, 0, 0.1);
			box-shadow: 0 2px 10px rgba(0, 0, 0, 0.05);
		}

		#site-header.transparent {
			background: transparent;
			backdrop-filter: none;
			border-bottom: none;
			box-shadow: none;
		}

		#site-header.hidden-up {
			transform: translateY(-100%);
		}

		.header-content {
			padding: 0 1rem;
		}

		/* Hide desktop navigation, show mobile menu */
		.desktop-nav {
			display: none;
		}

		.mobile-only {
			display: flex;
		}

		.logo-image {
			height: 28px;
		}

		.logo-text {
			font-size: 1.1rem;
		}

		/* Adjust root variables for mobile */
		:global(:root) {
			--header-h: 60px;
			--header-top: 0;
		}
	}
</style>
