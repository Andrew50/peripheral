<script lang="ts">
	import Header from '$lib/components/header.svelte';
	import '$lib/styles/global.css';
	import '$lib/styles/landing.css';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { startPricingPreload } from '$lib/utils/pricing-loader';

	if (browser) {
		document.title = 'Peripheral - Professional Trading Platform';
	}

	let isLoaded = false;

	onMount(() => {
		if (browser) {
			// Start preloading pricing configuration early
			startPricingPreload(); //this checks if its already been loaded so wont double load

			// Smooth scroll effect and animations
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

	// Feature data
	const features = [
		{
			icon: '📊',
			title: 'Real-time Charting',
			description:
				'Sub-minute charting for all US stocks and ETFs with professional-grade analysis tools.',
			highlights: ['Real-time data', 'Sub-minute precision', 'All US markets', 'Professional tools']
		},
		{
			icon: '👁️',
			title: 'Smart Watchlists',
			description:
				'Create, manage, and monitor multiple watchlists with advanced screening and alert capabilities.',
			highlights: ['Unlimited watchlists', 'Advanced screening', 'Bulk operations', 'Custom alerts']
		},
		{
			icon: '🔔',
			title: 'Intelligent Alerts',
			description:
				'Price, news, and strategy alerts with importance thresholds across all market instruments.',
			highlights: ['Price alerts', 'News monitoring', 'Strategy triggers', 'Market-wide coverage']
		},
		{
			icon: '📈',
			title: 'Strategy Backtesting',
			description:
				'Test any strategy on any set of stocks with historical data going back to 2008.',
			highlights: [
				'Historical data since 2008',
				'1-minute+ timeframes',
				'Custom strategies',
				'Performance metrics'
			]
		},
		{
			icon: '🔍',
			title: 'Market Screening',
			description:
				'Screen all stocks in real-time using advanced filters and custom strategy conditions.',
			highlights: ['Real-time screening', 'Custom filters', 'Strategy-based', 'Sortable results']
		},
		{
			icon: '📰',
			title: 'News Intelligence',
			description:
				'Access real-time and historical news with advanced filtering for stocks, ETFs, and markets.',
			highlights: ['Real-time news', 'Historical archive', 'Smart filtering', 'Market context']
		}
	];
</script>

<main class="landing-container">
	<!-- Background Effects -->
	<div class="background-animation">
		<div class="gradient-orb orb-1"></div>
		<div class="gradient-orb orb-2"></div>
		<div class="gradient-orb orb-3"></div>
		<div class="static-gradient"></div>
	</div>

	<!-- Navigation Header -->
	<header class="landing-header">
		<div class="header-content">
			<div class="logo-section">
				<img src="/atlantis_logo_transparent.png" alt="Peripheral Logo" class="logo-image" />
			</div>
			<nav class="navigation">
				<button class="nav-button secondary" on:click={navigateToLogin}>Login</button>
				<button class="nav-button secondary" on:click={navigateToPricing}>Pricing</button>
				<button class="nav-button primary" on:click={navigateToSignup}>Get Started</button>
			</nav>
		</div>
	</header>

	<!-- Hero Section -->
	<section class="hero-section" class:loaded={isLoaded}>
		<div class="hero-content">
			<div class="hero-badge">PROFESSIONAL TRADING PLATFORM</div>
			<h1 class="hero-title">
				<span class="gradient-text">Peripheral</span>
			</h1>
			<p class="hero-subtitle">
				The complete trading platform for serious traders.<br />
				Real-time data, advanced analytics, and intelligent automation.
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

	<!-- Features Section -->
	<section class="features-section">
		<div class="features-content">
			<div class="section-header">
				<h2 class="section-title">Powerful Trading Features</h2>
				<p class="section-subtitle">Everything you need to trade with confidence and precision</p>
			</div>

			<div class="features-grid">
				{#each features as feature}
					<div class="feature-card">
						<div class="feature-icon">{feature.icon}</div>
						<h3 class="feature-title">{feature.title}</h3>
						<p class="feature-description">{feature.description}</p>
						<ul class="feature-highlights">
							{#each feature.highlights as highlight}
								<li>{highlight}</li>
							{/each}
						</ul>
					</div>
				{/each}
			</div>
		</div>
	</section>

	<!-- Call-to-Action Section -->
	<section class="cta-section">
		<div class="cta-content">
			<h2 class="cta-title">Ready to elevate your trading?</h2>
			<p class="cta-subtitle">Join the next generation of professional traders</p>
			<div class="cta-actions">
				<button class="cta-button primary large" on:click={navigateToSignup}>
					Get Started Free
				</button>
				<button class="cta-button outline large" on:click={navigateToPricing}>
					View Pricing
				</button>
			</div>
		</div>
	</section>

	<!-- Footer -->
	<footer class="landing-footer">
		<div class="footer-content">
			<div class="footer-section">
				<img src="/atlantis_logo_transparent.png" alt="Peripheral" class="footer-logo" />
				<p class="footer-description">Professional trading platform for serious traders</p>
			</div>
			<div class="footer-section">
				<h4 class="footer-title">Platform</h4>
				<ul class="footer-links">
					<li><button on:click={navigateToApp}>Trading App</button></li>
					<li><button on:click={navigateToPricing}>Pricing</button></li>
				</ul>
			</div>
			<div class="footer-section">
				<h4 class="footer-title">Connect</h4>
				<ul class="footer-links">
					<li>
						<a
							href="https://twitter.com/peripheralio"
							target="_blank"
							rel="noopener noreferrer"
							class="social-link">Twitter</a
						>
					</li>
					<li>
						<a
							href="https://discord.gg/peripheral"
							target="_blank"
							rel="noopener noreferrer"
							class="social-link">Discord</a
						>
					</li>
					<li>
						<a href="mailto:info@peripheral.io" class="social-link">Email</a>
					</li>
				</ul>
			</div>
			<div class="footer-section">
				<h4 class="footer-title">Account</h4>
				<ul class="footer-links">
					<li><button on:click={navigateToLogin}>Login</button></li>
					<li><button on:click={navigateToSignup}>Sign Up</button></li>
				</ul>
			</div>
		</div>
		<div class="footer-bottom">
			<p>2025 Atlantis Labs, Inc</p>
		</div>
	</footer>
</main>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');

	.landing-container {
		position: relative;
		width: 100%;
		min-height: 100vh;
		background: #0a0b0d;
		color: #f9fafb;
		font-family:
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		overflow-x: hidden;
		display: flex;
		flex-direction: column;
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

	.orb-1 {
		width: 400px;
		height: 400px;
		background: radial-gradient(circle, rgba(59, 130, 246, 0.4) 0%, transparent 70%);
		top: -200px;
		left: -200px;
		animation-delay: 0s;
	}

	.orb-2 {
		width: 600px;
		height: 600px;
		background: radial-gradient(circle, rgba(139, 92, 246, 0.3) 0%, transparent 70%);
		top: 40%;
		right: -300px;
		animation-delay: -10s;
	}

	.orb-3 {
		width: 500px;
		height: 500px;
		background: radial-gradient(circle, rgba(16, 185, 129, 0.2) 0%, transparent 70%);
		bottom: -250px;
		left: 20%;
		animation-delay: -5s;
	}

	.static-gradient {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: linear-gradient(135deg, #0a0b0d 0%, #111827 50%, #0a0b0d 100%);
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
		background: rgba(10, 11, 13, 0.8);
		backdrop-filter: blur(20px);
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		transition: all 0.3s ease;
	}

	.header-content {
		max-width: 1200px;
		margin: 0 auto;
		padding: 1rem 2rem;
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.logo-section {
		display: flex;
		align-items: center;
	}

	.logo-image {
		height: 50px;
		width: auto;
		object-fit: contain;
		max-width: 200px;
	}

	.navigation {
		display: flex;
		gap: 1rem;
		align-items: center;
	}

	.nav-button {
		padding: 0.75rem 1.5rem;
		border: none;
		border-radius: 8px;
		font-size: 0.9rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.3s ease;
		text-decoration: none;
		background: transparent;
	}

	.nav-button.secondary {
		color: #9ca3af;
		border: 1px solid transparent;
	}

	.nav-button.secondary:hover {
		color: #ffffff;
		background: rgba(255, 255, 255, 0.1);
	}

	.nav-button.primary {
		background: #3b82f6;
		color: #ffffff;
		border: 1px solid #3b82f6;
	}

	.nav-button.primary:hover {
		background: #2563eb;
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
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
		color: #9ca3af;
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
		color: #ffffff;
		border: 1px solid transparent;
	}

	.cta-button.primary:hover {
		background: linear-gradient(135deg, #2563eb 0%, #5b21b6 100%);
		transform: translateY(-2px);
		box-shadow: 0 8px 25px rgba(59, 130, 246, 0.4);
	}

	.cta-button.secondary {
		color: #ffffff;
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

	/* Features Section */
	.features-section {
		position: relative;
		z-index: 10;
		padding: 6rem 2rem;
		background: rgba(255, 255, 255, 0.02);
		width: 100%;
		flex-shrink: 0;
	}

	.features-content {
		max-width: 1200px;
		margin: 0 auto;
	}

	.section-header {
		text-align: center;
		margin-bottom: 4rem;
	}

	.section-title {
		font-size: clamp(2.5rem, 6vw, 3.5rem);
		font-weight: 700;
		margin: 0 0 1rem 0;
		color: #ffffff;
	}

	.section-subtitle {
		font-size: 1.2rem;
		color: #9ca3af;
		font-weight: 400;
	}

	.features-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
		gap: 2rem;
		margin-top: 3rem;
		width: 100%;
		max-width: 100%;
	}

	.feature-card {
		background: rgba(255, 255, 255, 0.03);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 16px;
		padding: 2rem;
		transition: all 0.3s ease;
		backdrop-filter: blur(10px);
	}

	.feature-card:hover {
		transform: translateY(-5px);
		border-color: rgba(59, 130, 246, 0.3);
		box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
	}

	.feature-icon {
		font-size: 3rem;
		margin-bottom: 1.5rem;
		display: block;
	}

	.feature-title {
		font-size: 1.5rem;
		font-weight: 600;
		margin: 0 0 1rem 0;
		color: #ffffff;
	}

	.feature-description {
		color: #9ca3af;
		line-height: 1.6;
		margin-bottom: 1.5rem;
	}

	.feature-highlights {
		list-style: none;
		padding: 0;
		margin: 0;
	}

	.feature-highlights li {
		color: #60a5fa;
		margin-bottom: 0.5rem;
		position: relative;
		padding-left: 1.5rem;
		font-size: 0.9rem;
	}

	.feature-highlights li::before {
		content: '✓';
		position: absolute;
		left: 0;
		color: #22c55e;
		font-weight: bold;
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
		color: #ffffff;
	}

	.cta-subtitle {
		font-size: 1.2rem;
		color: #9ca3af;
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
		background: rgba(0, 0, 0, 0.5);
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		padding: 3rem 2rem 1rem;
		width: 100%;
		flex-shrink: 0;
		margin-top: auto;
	}

	.footer-content {
		max-width: 1200px;
		margin: 0 auto;
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 2rem;
	}

	.footer-section {
		display: flex;
		flex-direction: column;
	}

	.footer-logo {
		height: 40px;
		width: auto;
		object-fit: contain;
		max-width: 160px;
		margin-bottom: 1rem;
	}

	.footer-description {
		color: #9ca3af;
		font-size: 0.9rem;
		line-height: 1.5;
	}

	.footer-title {
		color: #ffffff;
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
		color: #ffffff;
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
		max-width: 1200px;
		margin: 2rem auto 0;
		padding-top: 2rem;
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		text-align: center;
		color: #6b7280;
		font-size: 0.9rem;
	}

	/* Responsive Design */
	@media (max-width: 768px) {
		.header-content {
			padding: 1rem;
		}

		.navigation {
			gap: 0.5rem;
		}

		.nav-button {
			padding: 0.5rem 1rem;
			font-size: 0.8rem;
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

		.features-grid {
			grid-template-columns: 1fr;
			gap: 1.5rem;
		}

		.feature-card {
			padding: 1.5rem;
		}

		.cta-actions {
			flex-direction: column;
			align-items: center;
		}

		.footer-content {
			grid-template-columns: repeat(2, 1fr);
			text-align: center;
		}
	}

	@media (max-width: 480px) {
		.features-grid {
			grid-template-columns: 1fr;
		}

		.feature-card {
			margin: 0 auto;
			max-width: 100%;
		}

		.footer-content {
			grid-template-columns: 1fr;
		}
	}

	/* Global styles for proper layout */
	:global(*) {
		box-sizing: border-box;
	}

	:global(body) {
		margin: 0;
		padding: 0;
		overflow-x: hidden;
	}

	:global(html) {
		overflow-x: hidden;
	}
</style>
