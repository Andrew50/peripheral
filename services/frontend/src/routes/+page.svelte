<script>
	import Header from '$lib/components/header.svelte';
	import '$lib/styles/global.css';
	import { browser } from '$app/environment';

	if (browser) {
		document.title = 'Atlantis';
	}
</script>

<main class="main-container">
	<!-- Remove #if showHero block -->
	<!-- {#if showHero} -->
		<div class="hero-wrapper">
			<Header />
			<section class="hero-section">
				<div class="hero-content content-padding">
					<h1>Atlantis</h1>
					<p class="tagline fluid-text">Comprehensive Market Analysis Suite</p>
					<div class="cta-buttons responsive-flex">
						<a href="/signup" class="cta-button primary responsive-shadow">Get Started</a>
						<a href="/login" class="cta-button secondary responsive-border">Sign In</a>
					</div>
					<!-- Features grid will be moved to its own section -->
				</div>
			</section>

			<section class="features-section">
					<div class="features-grid responsive-grid">
						<div class="feature-card responsive-shadow responsive-border">
							<div class="feature-header">
								<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="feature-icon">
									<path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
								</svg>
								<h3>Advanced Charting</h3>
							</div>
							<p class="fluid-text">Real-time data & historical replay with custom indicators</p>
						</div>
						<div class="feature-card responsive-shadow responsive-border">
							<div class="feature-header">
								<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="feature-icon">
									<path stroke-linecap="round" stroke-linejoin="round" d="M15.042 21.672 13.684 16.6m0 0-2.51 2.225.569-9.47 5.227 7.917-3.286-.672Zm-7.518-.267A8.25 8.25 0 1 1 20.25 10.5M8.288 14.212A5.25 5.25 0 1 1 17.25 10.5" />
								</svg>
								<h3>Pattern Recognition</h3>
							</div>
							<p class="fluid-text">Custom screening patterns with real-time market alerts</p>
						</div>
						<div class="feature-card responsive-shadow responsive-border">
							<div class="feature-header">
								<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="feature-icon">
									<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.125 1.125 0 0 1 0 2.25H5.625a1.125 1.125 0 0 1 0-2.25Z" />
								</svg>
								<h3>Trade Analysis</h3>
							</div>
							<p class="fluid-text">Comprehensive trade journaling and performance metrics</p>
						</div>
						<div class="feature-card responsive-shadow responsive-border">
							<div class="feature-header">
								<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="feature-icon">
									<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75Z" />
								</svg>
								<h3>Market Data</h3>
							</div>
							<p class="fluid-text">Time & sales, level 2, and full US market coverage</p>
						</div>
					</div>
			</section>
		</div>
	<!-- {/if} -->
</main>

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		overflow-x: hidden; /* Prevent horizontal scroll */
		/* Use background from theme variables */
		background: var(--c2, #1a1c21); /* Fallback color */
		/* Use primary font from theme */
		font-family: var(--font-primary, 'Fira Sans', sans-serif); /* Fallback font */
		min-height: 100vh;
		width: 100vw;
		color: var(--f1, #f9fafb); /* Default text color */
	}

	.main-container {
		position: relative;
		width: 100vw;
		min-height: 100vh;
		margin: 0;
		padding: 0;
		overflow-y: auto; /* Allow vertical scroll */
		overflow-x: hidden;
		display: flex;
		flex-direction: column;
	}

	.hero-wrapper {
		display: flex;
		flex-direction: column;
		flex-grow: 1; /* Allow wrapper to take available space */
	}

	.hero-section {
		display: flex;
		flex-direction: column;
		justify-content: center; /* Center content vertically */
		align-items: center; /* Center content horizontally */
		min-height: 80vh; /* Give hero ample space */
		width: 100%;
		padding: var(--space-xl, 4rem) 0; /* Use spacing variables */
	}

	/* Ensure header is positioned correctly */
	:global(app-header) { /* Assuming Header component uses <app-header> tag or similar */
		position: sticky;
		top: 0;
		z-index: 10;
		background: var(--c2, #1a1c21); /* Give header a background */
	}

	.hero-content {
		text-align: center;
		max-width: 800px; /* Constrain width for readability */
		padding-left: var(--space-md, 1.5rem); /* Use spacing variables */
		padding-right: var(--space-md, 1.5rem);
	}

	h1 {
		/* Use accent color from theme */
		color: var(--c3, #3b82f6);
		font-family: var(--font-primary, 'Fira Sans', sans-serif);
		margin-bottom: var(--space-xs, 0.5rem);
		text-transform: uppercase;
		font-size: clamp(2.8rem, 6vw, 4.5rem);
		font-weight: 600; /* Slightly bolder */
	}

	.tagline {
		/* Use primary font color */
		color: var(--f1, #f9fafb);
		margin-bottom: var(--space-lg, 2.5rem);
		font-weight: 300;
		font-size: clamp(1.1rem, 2.5vw, 1.4rem);
		max-width: 600px; /* Limit width for better readability */
		margin-left: auto;
		margin-right: auto;
	}

	.cta-buttons {
		display: flex;
		gap: var(--space-md, 1.5rem);
		justify-content: center;
		margin-bottom: var(--space-xl, 4rem); /* More space below buttons */
	}

	@media (max-width: 480px) {
		.cta-buttons {
			flex-direction: column;
			align-items: center;
			gap: var(--space-sm, 1rem);
		}
	}

	.cta-button {
		font-weight: 500; /* Align with button.css */
		text-decoration: none;
		transition: all 0.2s ease; /* Align with button.css */
		min-width: 160px; /* Fixed min-width */
		padding: var(--space-sm, 0.75rem) var(--space-lg, 1.5rem);
		border-radius: var(--radius-md, 6px); /* Use radius variables */
		font-size: clamp(0.95rem, 2vw, 1.1rem);
		border: 1px solid transparent; /* Base border */
	}

	.primary {
		/* Use theme accent color */
		background: var(--c3, #3b82f6);
		color: white; /* Ensure contrast */
		border-color: var(--c3, #3b82f6);
		/* box-shadow: 0 4px 14px rgba(59, 130, 246, 0.2); Removed for cleaner look */
	}

	.primary:hover {
		/* Use theme hover color */
		background: var(--c3-hover, #2563eb);
		border-color: var(--c3-hover, #2563eb);
		transform: translateY(-2px);
		/* box-shadow: 0 6px 18px rgba(59, 130, 246, 0.25); */
	}

	.secondary {
		background: transparent;
		/* Use theme accent color for border/text */
		color: var(--c3, #3b82f6);
		border: 1px solid var(--c3, #3b82f6); /* Use 1px border */
	}

	.secondary:hover {
		background: rgba(var(--c3-rgb, 59, 130, 246), 0.1); /* Use RGB version for opacity */
		transform: translateY(-2px);
		color: var(--c3-hover, #2563eb); /* Darken text slightly */
		border-color: var(--c3-hover, #2563eb); /* Darken border slightly */
	}

	.features-section {
		padding: var(--space-xl, 4rem) var(--space-lg, 2.5rem); /* Generous padding */
		background: var(--c1, #2d3139); /* Slightly different background for contrast */
		width: 100%;
	}

	.features-grid {
		/* responsive-grid handles columns */
		max-width: 1200px; /* Standard max width */
		margin: 0 auto;
		gap: var(--space-lg, 2.5rem); /* Larger gap */
	}

	.feature-card {
		/* Use theme background/border */
		background: var(--ui-bg-element, rgba(255, 255, 255, 0.03));
		padding: var(--space-lg, 2.5rem);
		border-radius: var(--radius-lg, 12px); /* Larger radius */
		text-align: left;
		transition: transform 0.3s ease, background-color 0.3s ease, border-color 0.3s ease;
		border: 1px solid var(--ui-border, rgba(255, 255, 255, 0.1));
		height: 100%;
		/* box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1); */ /* Removed shadow */
	}

	.feature-header {
		display: flex;
		align-items: center;
		margin-bottom: var(--space-md, 1.5rem);
		gap: var(--space-md, 1.5rem);
	}

	.feature-icon {
		font-size: clamp(1.8rem, 3.5vw, 2.2rem);
		/* Color the icon using accent */
		color: var(--c3, #3b82f6);
		/* Add width and height for SVG */
		width: clamp(32px, 5vw, 44px);
		height: clamp(32px, 5vw, 44px);
	}

	.feature-card h3 {
		/* Use primary text color */
		color: var(--f1, #f9fafb);
		margin: 0;
		font-weight: 500;
		font-size: clamp(1.1rem, 2.5vw, 1.3rem);
	}

	.feature-card p {
		/* Use secondary text color */
		color: var(--f2, #9ca3af);
		line-height: 1.6; /* Slightly more line spacing */
		margin: 0;
		font-size: clamp(0.9rem, 2vw, 1rem);
	}

	.feature-card:hover {
		transform: translateY(-4px);
		background: var(--ui-bg-hover, rgba(255, 255, 255, 0.05));
		border-color: var(--c3, #3b82f6); /* Highlight border on hover */
	}
</style>
