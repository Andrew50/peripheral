<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	// Pricing preload removed - now handled directly in pricing page
	import ChipSection from '$lib/landing/ChipSection.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import '$lib/styles/splash.css';
	import { getAuthState, getCookie } from '$lib/auth';

	if (browser) {
		document.title = 'Peripheral';
	}

	// Auth state - check immediately to prevent flash
	let isAuthenticated = getAuthState();

	// Subsections data for the landing page
	const subsections = [
		{
			title: 'Backtest trading ideas',
			description:
				'Backtest trading strategies, analyze event or macro trading opportunities, or research investment portfolios in minutes, not days.',
			content: '',
			image: '/query_glass.png'
		},
		{
			title: 'Analysis now, not after the trade',
			description:
				'In dynamic, fast-moving markets, every second counts. Our agent analyzes headlines, fundamental events, and data 100% faster than ChatGPT and Perplexity.',
			content: '',
			image: '/splash-speed-color.png'
		},
		{
			title: 'Never miss a trade.',
			description:
				'Deploy strategies to receive alerts when they trigger in realtime. Our infrastructure delivers alerts down to minute resolution within five seconds of the event triggering.',
			content: '',
			image: '/alert_glass.png'
		},
		{
			title: 'Frictionless trading.',
			description:
				'All the insights you need, before you ask. Our context aware terminal automatically surfaces the most relevant news, data, and insights about symbols you care about. Stay in the flow.',
			content: '',
			image: '/Group 10.png'
		}
	];
</script>

<SiteHeader {isAuthenticated} />

<!-- Wrapper with unified gradient -->
<div class="page-wrapper">
	<!-- Title Section - Extracted from HeroAnimation -->
	<section class="hero-title-section">
		<div class="hero-title-container">
			<h1 class="hero-title">The Intelligent Trading Terminal</h1>
			<p class="hero-subtitle">
				Backtest trading ideas, analyze breaking news and event driven strategies, and deploy agents
				in realtime.<br />
			</p>
			<a href="/signup" class="hero-cta-button">
				<div class="button-border-layer-1">
					<div class="button-border-layer-2">
						<div class="button-border-layer-3">
							<div class="button-content">Supercharge your trading →</div>
						</div>
					</div>
				</div>
			</a>
		</div>
	</section>
	<ChipSection />
	<main class="landing-container">
		<!-- Subsections moved to be directly below title -->
		<section class="subsections-section">
			<h2 class="features-title">Features</h2>

			<div class="subsections-content">
				{#each subsections as subsection, index}
					<div
						class="subsection"
						class:reverse={index % 2 === 0}
						class:frictionless={index === 3}
						class:speed-analysis={index === 2}
						class:never-miss={index === 1}
					>
						<div class="subsection-text">
							<h2 class="subsection-title">{subsection.title}</h2>
							<p class="subsection-description">{subsection.description}</p>
							<p class="subsection-content">{subsection.content}</p>
						</div>
						<div class="subsection-image">
							<img src={subsection.image} alt={subsection.title} />
						</div>
					</div>
				{/each}
			</div>
		</section>

		<!-- HeroAnimation moved below subsections 
	<HeroAnimation {defaultKey} {chartsByKey} />
-->

		<!-- Ideas Chips Section -->
		<!-- Footer -->
		<SiteFooter />
	</main>
</div>
<svelte:head>
	<!-- Basic meta tags -->
	<title>Peripheral</title>
	<meta name="description" content="The intelligent trading terminal." />

	<!-- Open Graph meta tags for Facebook, LinkedIn, etc. -->
	<meta property="og:title" content="Peripheral.io" />
	<meta property="og:image" content="/og-homepage.png" />
	<meta property="og:description" content="The intelligent trading terminal." />
	<meta property="og:image:width" content="1200" />
	<meta property="og:image:height" content="630" />
	<meta property="og:image:type" content="image/png" />
	<meta property="og:image:alt" content="Peripheral | The Intelligent Trading Terminal" />
	<meta property="og:url" content="https://peripheral.io" />
	<meta property="og:type" content="website" />
	<meta property="og:site_name" content="Peripheral" />

	<!-- Twitter Card meta tags -->
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:site" content="@peripheralio" />
	<meta name="twitter:title" content="Peripheral.io" />
	<meta name="twitter:description" content="The intelligent trading terminal." />
	<meta name="twitter:image" content="/og-homepage.png" />
	<meta name="twitter:image:alt" content="Peripheral | The Intelligent Trading Terminal" />

	<!-- Additional meta tags for better sharing -->
	<meta property="article:author" content="Peripheral" />
	<meta name="theme-color" content="#0a0a0a" />
</svelte:head>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700;800&display=swap');
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');
	@import url('https://fonts.googleapis.com/css2?family=Instrument+Sans:wght@400;500;600;700;800&display=swap');

	/* Critical global styles - applied immediately to prevent layout shift */
	:global(*) {
		box-sizing: border-box;
	}

	:global(html) {
		-ms-overflow-style: none; /* IE and Edge */
	}

	:global(body) {
		-ms-overflow-style: none; /* IE and Edge */
	}

	/* Override width restrictions from global landing styles - moved to top for immediate application */
	:global(.landing-container) {
		max-width: none !important;
		width: 100% !important;
		margin: 0 !important;
		padding: 0 !important; /* remove side gutters */
	}

	.page-wrapper {
		width: 100%;
		min-height: 100vh;
		background: linear-gradient(180deg, #010022 0%, #02175f 100%);
	}

	.landing-container {
		position: relative;
		width: 100%;
		background: transparent;
		color: var(--color-dark);
		font-family:
			'Instrument Sans',
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		display: flex;
		flex-direction: column;
		min-height: 100vh;
	}

	.hero-title-section {
		position: relative;
		z-index: 20;
		padding: 14rem 0.5rem 12rem;
		background: transparent;
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 60vh;
	}

	.hero-title-container {
		text-align: center;
		max-width: 1800px;
		margin: 0 auto;
		padding: 0 1rem;
	}

	.hero-title {
		font-size: clamp(4.05rem, 6vw, 7.5rem);
		font-weight: 400;
		margin: 0 0 1rem;
		letter-spacing: -0.02em;
		line-height: 1.1;
		color: #f5f9ff;
		text-shadow:
			0 2px 12px rgb(0 0 0 / 20%),
			0 1px 0 rgb(255 255 255 / 1%);
	}

	.hero-subtitle {
		font-size: clamp(1.1rem, 3vw, 1.5rem);
		color: rgb(245 249 255 / 85%);
		margin-bottom: 1.5rem;
		line-height: 1.6;
		margin-top: 0;
		font-weight: 400;
		font-family: 'Instrument Sans', sans-serif;
	}

	.hero-cta-button {
		display: inline-block;
		text-decoration: none;
		margin-top: 1rem;
		padding: 4px;
		border-radius: 58px;
		border: 1px solid rgb(255 255 255 / 4%);
		transition: all 0.3s ease;
		background: transparent;
	}

	.button-border-layer-1 {
		border-radius: 50px;
		border: 1px solid rgb(255 255 255 / 8%);
		padding: 4px;
		background: transparent;
	}

	.button-border-layer-2 {
		border-radius: 42px;
		border: 1px solid rgb(255 255 255 / 20%);
		padding: 4px;
		background: transparent;
	}

	.button-border-layer-3 {
		border-radius: 40px;
		background: linear-gradient(135deg, rgb(255 255 255 / 95%) 0%, rgb(240 240 240 / 98%) 100%);
		padding: 0;
		overflow: hidden;
	}

	.button-content {
		padding: 1rem 2rem;
		font-size: 1.1rem;
		font-weight: 400;
		font-family: 'Instrument Sans', sans-serif;
		color: #1a1a1a;
		text-align: center;
		background: transparent;
		border-radius: 40px;
		transition: transform 0.3s ease;
	}

	.hero-cta-button:hover .button-content {
		transform: scale(1.05);
	}

	.hero-cta-button:active {
		transform: translateY(0);
	}

	/* Subsections Section */
	.subsections-section {
		position: relative;
		z-index: 10;
		padding: 6rem 2rem;
		width: 100%;
		flex-shrink: 0;
	}

	.features-title {
		font-size: clamp(3rem, 7vw, 4rem);
		font-weight: 400;
		margin: 0 0 4rem;
		color: #f5f9ff;
		line-height: 1.2;
		text-align: center;
		letter-spacing: -0.02em;
	}

	.subsections-content {
		width: 80vw;
		max-width: 1400px;
		margin: 0 auto;
		padding: 0 2rem;
	}

	.subsection {
		display: flex;
		align-items: center;
		gap: 20rem;
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
		font-size: clamp(3rem, 7vw, 3.5rem);
		font-weight: 400;
		margin: 0 0 1.5rem;
		color: #f5f9ff;
		line-height: 1.2;
	}

	/* Specific styling for Transform ideas section (first subsection) */
	.subsection:first-child .subsection-title {
		color: white;
	}

	.subsection:first-child .subsection-description {
		color: white;
	}

	.subsection-description {
		font-size: 1.2rem;
		color: #f5f9ff;
		font-weight: 400;
		margin-bottom: 1.5rem;
		line-height: 1.5;
	}

	.subsection-content {
		font-size: 1rem;
		color: #f5f9ff;
		line-height: 1.7;
		opacity: 0.8;
	}

	.subsection-image {
		flex: 1;
		max-width: 700px;
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.subsection-image img {
		width: 100%;
		max-width: 400px;
		height: auto;
		image-rendering: -webkit-optimize-contrast;
		-ms-interpolation-mode: bicubic;
	}

	/* Make query_glass.png bigger */
	.subsection:first-child .subsection-image {
		max-width: 1200px;
	}

	.subsection:first-child .subsection-image img {
		max-width: 1200px;
	}

	/* Responsive Design */
	@media (width <= 768px) {
		.hero-title-section {
			padding: 2rem 1rem;
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
		}

		.subsection {
			flex-direction: column;
			gap: 3rem;
			margin-bottom: 4rem;
			padding: 2rem 0;
		}

		.subsection.reverse {
			flex-direction: column;
		}

		.subsection-text {
			max-width: 100%;
		}

		.subsection-image {
			max-width: 100%;
			order: 2;
		}

		.subsection-image img {
			max-width: 350px;
		}
	}

	@media (width <= 480px) {
		.hero-title-section {
			padding: 1.5rem 1rem;
			min-height: 100vh;
			display: flex;
			align-items: center;
			justify-content: center;
		}

		.subsection {
			gap: 1.5rem;
			margin-bottom: 3rem;
			padding: 1.5rem 0;
		}
	}
</style>
