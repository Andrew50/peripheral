<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { startPricingPreload } from '$lib/utils/pricing-loader';

	if (browser) {
		document.title = 'Peripheral';
	}

	let isLoaded = false;
	let isHeaderVisible = true;
	let isHeaderTransparent = true;
	let prevScrollY = 0;

	function handleScroll() {
		const currentY = window.scrollY;
		// Header visibility: show if at top, within 20px, or scrolling up
		if (currentY === 0 || currentY < 20 || currentY < prevScrollY) {
			isHeaderVisible = true;
		} else {
			isHeaderVisible = false;
		}
		// Header transparency: transparent if < 30px from top
		isHeaderTransparent = currentY < 30;
		prevScrollY = currentY;
	}

	onMount(() => {
		if (browser) {
			// Start preloading pricing configuration early
			startPricingPreload();
			if (window.scrollY > 30) {
				isHeaderVisible = false;
				isHeaderTransparent = false;
			} 
			// Only set loaded state for animation
			isLoaded = true;
			document.body.classList.add('loaded');
		}
	});

	function navigateToLogin() {
		goto('/login');
	}

	function navigateToSignup() {
		goto('/signup');
	}

	function navigateToPricing() {
		goto('/pricing');
	}

	function navigateToApp() {
		goto('/app');
	}

	function handleNavClick(event: Event) {
		// Prevent pill click when clicking navigation buttons
		event.stopPropagation();
	}

	// Subsections data
	const subsections = [
		{
			title: 'Transform ideas into edge in minutes',
			description: 'From concept to execution, our platform turns your trading insights into profitable strategies faster than ever before.',
			content: 'Whether you have a hunch about market patterns or a complex algorithmic strategy, Peripheral provides the tools to test, refine, and deploy your ideas with unprecedented speed and precision.'
		},
		{
			title: 'Never miss a trade.',
			description: 'Stay ahead of the market with instant access to live data, news, and analytics across all major exchanges.',
			content: 'Our advanced data infrastructure delivers sub-minute precision for all US stocks and ETFs, combined with intelligent filtering and alerting systems that keep you informed of what matters most.'
		},
		{
			title: 'Built for serious traders',
			description: 'Professional-grade tools designed for both individual traders and institutional-level strategies.',
			content: 'From backtesting with historical data since 2008 to real-time screening and portfolio management, every feature is crafted to meet the demanding needs of serious market participants.'
		}
	];
</script>

<!-- Window scroll listener -->
<svelte:window on:scroll={handleScroll} />

<!-- Morphing pill header -->
<header id="site-header" class:transparent={isHeaderTransparent} class:hidden-up={!isHeaderVisible}>
	<nav class="header-content">
		<div class="logo-section">
			<img src="/atlantis_logo_transparent.png" alt="Peripheral Logo" class="logo-image" />
			<p class="logo-text">Peripheral</p>
		</div>
		<div class="navigation">
			<button class="nav-button secondary" on:click={(e) => { handleNavClick(e); navigateToPricing(); }}>Pricing</button>
			<button class="nav-button secondary" on:click={(e) => { handleNavClick(e); navigateToLogin(); }}>Login</button>
			<button class="nav-button primary" on:click={(e) => { handleNavClick(e); navigateToSignup(); }}>Sign up</button>
		</div>
	</nav>
</header>

<main class="landing-container">
	<!-- Hero Section -->
	<section class="hero-section" class:loaded={isLoaded}>
		<div class="hero-content">
			<h1 class="hero-title">
				<span class="gradient-text">Peripheral</span>
			</h1>
			<p class="hero-subtitle">
				Peripheral is the terminal to envision and execute your trading ideas.<br />
			</p>
			<div class="hero-actions">
				<button class="cta-button primary" on:click={navigateToSignup}>
					Start Trading
					<svg class="arrow-icon" viewBox="0 0 24 24" fill="none">
						<path
								d="M5 12H19M19 12L12 5M19 12L12 19"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
						/>
					</svg>
				</button>
				<button class="cta-button secondary" on:click={navigateToApp}> View Demo </button>
			</div>
		</div>
	</section>

	<!-- Subsections -->
	<section class="subsections-section">
		<div class="subsections-content">
			{#each subsections as subsection, index}
				<div class="subsection" class:reverse={index % 2 === 1}>
					<div class="subsection-text">
						<h2 class="subsection-title">{subsection.title}</h2>
						<p class="subsection-description">{subsection.description}</p>
						<p class="subsection-content">{subsection.content}</p>
					</div>
					<div class="subsection-visual">
						<div class="visual-placeholder">
							<div class="visual-icon">
								{#if index === 0}
									⚡
								{:else if index === 1}
									📊
								{:else}
									🎯
								{/if}
							</div>
						</div>
					</div>
				</div>
			{/each}
		</div>
	</section>


	<!-- Footer -->
	<footer class="landing-footer">
		<div class="footer-content">
			<div class="footer-section footer-left"> 
				<p class="footer-description">The final trading terminal.</p>
			</div>
			<div class="footer-sections-right">
				<div class="footer-section">
					<h4 class="footer-title">Platform</h4>
					<ul class="footer-links">
						<li><button on:click={navigateToPricing}>Pricing</button></li>
						<li><button on:click={navigateToApp}>Dashboard</button></li>
					</ul>
				</div>
				<div class="footer-section">
					<h4 class="footer-title">Account</h4>
					<ul class="footer-links">
						<li><button on:click={navigateToLogin}>Login</button></li>
						<li><button on:click={navigateToSignup}>Sign Up</button></li>
					</ul>
				</div>
				<div class="footer-section">
					<h4 class="footer-title">Connect</h4>
					<div class="footer-social-row">
						<a href="https://twitter.com/peripheralio" target="_blank" rel="noopener noreferrer" class="footer-social-icon" aria-label="X (Twitter)">
							<img src="/x-logo-white.png" alt="X (Twitter)" style="width: 18px; height: 18px; object-fit: contain; display: block;" />
						</a>
						<a href="https://discord.gg/peripheral" target="_blank" rel="noopener noreferrer" class="footer-social-icon" aria-label="Discord">
							<img src="/Discord-Symbol-White.png" alt="Discord" style="width: 18px; height: 18px; object-fit: contain; display: block;" />
						</a>
						<a href="https://www.linkedin.com/company/peripheralio" target="_blank" rel="noopener noreferrer" class="footer-social-icon" aria-label="LinkedIn">
							<img src="/InBug-White.png" alt="LinkedIn" style="width: 18px; height: 18px; object-fit: contain; display: block;" />
						</a>

					</div>
				</div>
			</div>
		</div>
		<div class="footer-bottom">
			<p>Atlantis Labs, Inc. 2025</p>
		</div>
		<div class="footer-brand">Peripheral</div>
	</footer>
</main>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700;800&display=swap');
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');
	
	/* Ensure fonts load properly */
	:global(html) {
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		background: linear-gradient(135deg, var(--color-light) 0%, var(--color-accent) 100%);
	}

	:global(body) {
		background: var(--color-light);
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
	}

	:root {
		--color-dark: #0B2E33;
		--color-primary: #4F7C82;
		--color-accent: #93B1B5;
		--color-light: #B8E3E9;
		--pill-size: 40px;
		--header-h: 48px;
		--header-top: 16px;
	}

	.landing-container {
		position: relative;
		width: 100%;
		background: linear-gradient(135deg, var(--color-light) 0%, var(--color-accent) 100%);
		color: var(--color-dark);
		font-family:
				'Geist',
				'Inter',
				-apple-system,
				BlinkMacSystemFont,
				'Segoe UI',
				Roboto,
				sans-serif;
		display: flex;
		flex-direction: column;
		padding-top: var(--header-h);
	}

	/* Background Effects */
	.background-animation {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		z-index: 0;
		pointer-events: none;
	}

	.gradient-orb {
		position: absolute;
		border-radius: 50%;
		filter: blur(100px);
		opacity: 0.3;
		animation: float 20s ease-in-out infinite;
	}


	.static-gradient {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: var(--color-light);
		z-index: -1;
	}

	@keyframes float {
			0%,
			100% {
					transform: translate(0, 0) scale(1);
			}
			25% {
					transform: translate(30px, -30px) scale(1.1);
			}
			50% {
					transform: translate(-20px, 20px) scale(0.9);
			}
			75% {
					transform: translate(20px, 10px) scale(1.05);
			}
	}

	/* Navigation Header */
	.landing-header {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		z-index: 1000;
		background: var(--color-dark);
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		transition: all 0.3s ease;
	}

	.header-content {
		padding: 0 2.5rem;
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
	}

	.nav-button.secondary {
		background: #00000000;
		color: #000000;
		border: 1px solid var(--color-primary);
	}

	.nav-button.secondary:hover {
		background: var(--color-primary);
		color: var(--color-light);
	}

	.nav-button.primary {
		background: rgb(0, 0, 0);
		color: #f5f9ff;
	}

	.nav-button.primary:hover {
		background: var(--color-accent);
		color: var(--color-dark);
		border: 1px solid var(--color-accent);
	}

	/* Pill icon (hidden by default) */
	.pill-icon {
		display: none;
		position: absolute;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		color: var(--color-dark);
		z-index: 10;
	}

	/* Pill state styles */
	#site-header {
		position: fixed;
		top: var(--header-top);
		left: 50%;
		transform: translateX(-50%);
		width: 90vw;
		max-width: 1400px;
		height: var(--header-h);
		background: #f5f9ff;
		backdrop-filter: blur(16px);
		border: 1px solid rgba(0,0,0,.08);
		border-radius: 999px;
		transition: all .4s cubic-bezier(.4,0,.2,1);
		z-index: 1050;
		box-shadow: 0 4px 20px rgba(0,0,0,.1);
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		cursor: pointer;
	}

	#site-header.pill {
		width: var(--pill-size);
		height: var(--pill-size);
		border-radius: 50%;
		top: var(--header-top);
		transform: translateX(-50%);
		cursor: pointer;
	}

	#site-header.pill .header-content {
		padding: 0;
		justify-content: center;
	}

	#site-header.pill .logo-section,
	#site-header.pill .navigation {
		display: none;
	}

	#site-header.pill .pill-icon {
		display: block;
	}

	#site-header.pill:hover {
		background: rgba(184, 227, 233, 0.95);
		transform: translateX(-50%) scale(1.05);
	}

	/* Hero Section */
	.hero-section {
		position: relative;
		z-index: 10;
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 8rem 2rem 4rem;
		text-align: center;
		width: 100%;
		flex-shrink: 0;
	}

	.hero-content {
		max-width: 800px;
		opacity: 0;
		transform: translateY(30px);
		transition: all 1s ease;
	}

	.hero-section.loaded .hero-content {
		opacity: 1;
		transform: translateY(0);
	}

	.hero-badge {
		display: inline-block;
		padding: 0.5rem 1.5rem;
		background: rgba(59, 130, 246, 0.1);
		border: 1px solid rgba(59, 130, 246, 0.3);
		border-radius: 100px;
		color: #60a5fa;
		font-size: 0.8rem;
		font-weight: 600;
		letter-spacing: 1px;
		text-transform: uppercase;
		margin-bottom: 2rem;
	}

	.hero-title {
		font-size: clamp(3.5rem, 8vw, 6rem);
		font-weight: 800;
		margin: 0 0 1.5rem 0;
		letter-spacing: -0.02em;
		line-height: 1.1;
	}

	.gradient-text {
		background: linear-gradient(
				135deg,
				#3b82f6 0%,
				#6366f1 25%,
				#8b5cf6 50%,
				#ec4899 75%,
				#f59e0b 100%
		);
		background-size: 200% 200%;
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
		animation: gradient-shift 8s ease infinite;
	}

	@keyframes gradient-shift {
			0%,
			100% {
					background-position: 0% 50%;
			}
			25% {
					background-position: 100% 50%;
			}
			50% {
					background-position: 100% 100%;
			}
			75% {
					background-position: 0% 100%;
			}
	}

	.hero-subtitle {
		font-size: clamp(1.1rem, 3vw, 1.5rem);
		color: #f5f9ff;
		margin-bottom: 3rem;
		line-height: 1.6;
		font-weight: 400;
	}

	.hero-actions {
		display: flex;
		gap: 1rem;
		justify-content: center;
		flex-wrap: wrap;
	}

	.cta-button {
		padding: 1rem 2rem;
		border: none;
		border-radius: 12px;
		font-size: 1rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.3s ease;
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		text-decoration: none;
		background: transparent;
		white-space: nowrap;
	}

	.cta-button.primary {
		background: linear-gradient(135deg, #3b82f6 0%, #6366f1 100%);
		color: #f5f9ff;
		border: 1px solid transparent;
	}

	.cta-button.primary:hover {
		background: linear-gradient(135deg, #2563eb 0%, #5b21b6 100%);
		transform: translateY(-2px);
		box-shadow: 0 8px 25px rgba(59, 130, 246, 0.4);
	}

	.cta-button.secondary {
		color: #f5f9ff;
		border: 1px solid rgba(255, 255, 255, 0.3);
		background: rgba(255, 255, 255, 0.05);
	}

	.cta-button.secondary:hover {
		background: rgba(255, 255, 255, 0.1);
		border-color: rgba(255, 255, 255, 0.5);
	}

	.cta-button.outline {
		color: #3b82f6;
		border: 2px solid #3b82f6;
		background: transparent;
	}

	.cta-button.outline:hover {
		background: rgba(59, 130, 246, 0.1);
		transform: translateY(-1px);
	}

	.cta-button.large {
		padding: 1.25rem 2.5rem;
		font-size: 1.1rem;
	}

	.arrow-icon {
		width: 20px;
		height: 20px;
	}

	/* Subsections Section */
	.subsections-section {
		position: relative;
		z-index: 10;
		padding: 6rem 2rem;
		background: rgba(255, 255, 255, 0.02);
		width: 100%;
		flex-shrink: 0;
	}

	.subsections-content {
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 2rem;
	}

	.subsection {
		display: flex;
		align-items: center;
		gap: 4rem;
		margin-bottom: 6rem;
		padding: 3rem 0;
	}

	.subsection:last-child {
		margin-bottom: 0;
	}

	.subsection.reverse {
		flex-direction: row-reverse;
	}

	.subsection-text {
		flex: 1;
		max-width: 500px;
	}

	.subsection-title {
		font-size: clamp(2rem, 5vw, 2.5rem);
		font-weight: 700;
		margin: 0 0 1.5rem 0;
		color: var(--color-dark);
		line-height: 1.2;
	}

	.subsection-description {
		font-size: 1.2rem;
		color: var(--color-primary);
		font-weight: 500;
		margin-bottom: 1.5rem;
		line-height: 1.5;
	}

	.subsection-content {
		font-size: 1rem;
		color: var(--color-dark);
		line-height: 1.7;
		opacity: 0.8;
	}

	.subsection-visual {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 300px;
	}

	.visual-placeholder {
		width: 200px;
		height: 200px;
		background: var(--color-accent);
		border: 2px solid var(--color-primary);
		border-radius: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all 0.3s ease;
		backdrop-filter: blur(10px);
	}

	.visual-placeholder:hover {
		transform: translateY(-5px);
		box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1);
	}

	.visual-icon {
		font-size: 4rem;
		opacity: 0.8;
	}

	/* CTA Section */
	.cta-section {
		position: relative;
		z-index: 10;
		padding: 6rem 2rem;
		background: linear-gradient(135deg, rgba(59, 130, 246, 0.1) 0%, rgba(139, 92, 246, 0.1) 100%);
		text-align: center;
		width: 100%;
		flex-shrink: 0;
	}

	.cta-content {
		max-width: 600px;
		margin: 0 auto;
	}

	.cta-title {
		font-size: clamp(2rem, 5vw, 3rem);
		font-weight: 700;
		margin: 0 0 1rem 0;
		color: var(--color-dark);
	}

	.cta-subtitle {
		font-size: 1.2rem;
		color: var(--color-primary);
		margin-bottom: 2.5rem;
	}

	.cta-actions {
		display: flex;
		gap: 1rem;
		justify-content: center;
		flex-wrap: wrap;
	}

	/* Footer */
	.landing-footer {
		position: relative;
		z-index: 10;
		background: var(--color-dark);
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		width: 100vw;
		left: 50%;
		transform: translateX(-50%);
		padding: 3rem 0 1rem 0;
		flex-shrink: 0;
		margin-top: auto;
	}

	.footer-content, .footer-bottom, .footer-brand {

		margin: 0 auto;
		padding: 0 2rem;
	}

	.footer-content {
		display: flex;
		flex-direction: row;
		justify-content: space-between;
		align-items: flex-start;
		gap: 0;
	}

	.footer-left {
		flex: 1 1 0%;
		min-width: 180px;
	}

	.footer-sections-right {
		display: flex;
		flex-direction: row;
		gap: 2rem;
		justify-content: flex-end;
		align-items: flex-start;
	}

	.footer-logo {
		height: 40px;
		width: auto;
		object-fit: contain;
		max-width: 160px;
		margin-bottom: 1rem;
	}

	.footer-description {
		color: #f5f9ff;
		font-size: 0.9rem;
		line-height: 1.5;
	}

	.footer-title {
		color: var(--color-light);
		font-size: 1.1rem;
		font-weight: 600;
		margin: 0 0 1rem 0;
	}

	.footer-links {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.footer-links li {
		margin-bottom: 0.5rem;
	}

	.footer-links button {
		background: none;
		border: none;
		color: #9ca3af;
		cursor: pointer;
		font-size: 0.9rem;
		padding: 0;
		text-align: left;
		transition: color 0.3s ease;
	}

	.footer-links button:hover {
		color: #f5f9ff;
	}

	.social-link {
		color: #9ca3af;
		text-decoration: none;
		font-size: 0.9rem;
		transition: color 0.3s ease;
	}

	.social-link:hover {
		color: #ffffff;
	}

	.footer-bottom {
		text-align: left;
		color: var(--color-primary);
		font-size: 0.9rem;
		display: flex;
		align-items: flex-end;
	}

	/* Responsive Design */
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

		.hero-section {
			padding: 6rem 1rem 3rem;
		}

		.hero-actions {
			flex-direction: column;
			align-items: center;
		}

		.cta-button {
			width: 100%;
			max-width: 300px;
			justify-content: center;
		}

		.subsection {
			flex-direction: column;
			gap: 2rem;
			margin-bottom: 4rem;
			padding: 2rem 0;
		}

		.subsection.reverse {
			flex-direction: column;
		}

		.subsection-text {
			max-width: 100%;
		}

		.subsection-visual {
			min-height: 200px;
		}

		.visual-placeholder {
			width: 150px;
			height: 150px;
		}

		.visual-icon {
			font-size: 3rem;
		}

		.cta-actions {
			flex-direction: column;
			align-items: center;
		}

		.footer-content {
			flex-direction: column;
			align-items: stretch;
		}
		.footer-sections-right {
			flex-direction: column;
			gap: 1.2rem;
			align-items: stretch;
		}
	}

	@media (max-width: 480px) {
		.subsection {
			gap: 1.5rem;
			margin-bottom: 3rem;
			padding: 1.5rem 0;
		}

		.visual-placeholder {
			width: 120px;
			height: 120px;
		}

		.visual-icon {
			font-size: 2.5rem;
		}

		.footer-content {
			flex-direction: column;
		}
		.footer-sections-right {
			flex-direction: column;
			gap: 1rem;
		}
	}

	/* Global styles for proper layout */
	:global(*) {
		box-sizing: border-box;
	}

	#site-header.transparent {
		background: transparent;
		box-shadow: none;
		border: none;
	}

	#site-header.hidden-up {
		transform: translateX(-50%) translateY(-120%);
		opacity: 0;
		pointer-events: none;
		transition: transform 0.4s cubic-bezier(.4,0,.2,1), opacity 0.4s cubic-bezier(.4,0,.2,1);
	}

	.footer-brand {
		width: 100vw;
		position: relative;
		left: 50%;
		transform: translateX(-50%);
		text-align: center;
		font-size: clamp(4rem, 14vw, 12rem);
		font-weight: 900;
		color: #f5f9ff;
		padding: 0.5rem 0 0.5rem 0;
		line-height: 1;
		letter-spacing: -0.06em;
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		z-index: 1;
		pointer-events: none;
		user-select: none;
	}

	.footer-social-row {
		display: flex;
		gap: 0.3rem;
		align-items: center;
		justify-content: center;
		margin-top: 0.5rem;
	}

	.footer-social-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: none;
		border-radius: 50%;
		transition: none;
		font-size: 2rem;
	}

	.footer-social-icon:hover {
		color: inherit;
		background: none;
	}

	.footer-social-icon img {
		filter: grayscale(1) brightness(0.8);
		opacity: 0.7;
		transition: filter 0.2s, opacity 0.2s;
	}

	.footer-social-icon:hover img {
		filter: none;
		opacity: 1;
	}

</style>
