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


	// Subsections data
	const subsections = [
		{
			title: 'Transform ideas into edge in minutes',
			description:
				'Backtest trading strategies, analyze event or macro trading opportunities, or research investment portfolios in minutes, not days.',
			content: '',
			image: '/query_glass.png'
		},
		{
			title: 'Never miss a trade.',
			description:
				'Deploy strategies to receive alerts when they trigger in realtime. Our infrastructure delivers alerts down to minute resolution within five seconds of the event triggering.',
			content: '',
			image: '/alert_glass.png'
		},
		{
			title: 'Analysis at the speed of now',
			description:
				'In dynamic, fast-moving markets, every second counts. Peripheral provides quality analysis of news, fundamentals, and data XX% faster than ChatGPT, Perplexity, and comparable services with accurate data.',
			content: '',
			image: '/splash-speed-color.png'
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
			<h1 class="hero-title">
				The <span class="gradient-text">best</span> way to trade.
			</h1>
			<p class="hero-subtitle">
				Peripheral enables you to envision and execute your trading ideas.<br />
			</p>
			<a href="/signup" class="hero-cta-button">
				Get Started for Free
			</a>
		</div>
	</section>

	<main class="landing-container">
	<!-- Subsections moved to be directly below title -->
	<section class="subsections-section">
		<div class="subsections-content">
			{#each subsections as subsection, index}
				<div class="subsection" class:reverse={index % 2 === 0} class:frictionless={index === 3} class:speed-analysis={index === 2} class:never-miss={index === 1}>
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
	<ChipSection />
	<!-- Footer -->
	<SiteFooter />
</main>
</div>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700;800&display=swap');
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');


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
		background: linear-gradient(
			180deg,
			rgba(3, 1, 85, 0.75) 0%,
			rgba(2, 1, 50, 0.8) 30%,
			#010022 60%,
			#000000 100%
		);
	}

	.landing-container {
		position: relative;
		width: 100%;
		background: transparent;
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
		min-height: 100vh;
	}

	.hero-title-section {
		position: relative;
		z-index: 20;
		padding: 10rem 0.5rem 17rem 0.5rem;
		background: transparent;
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 60vh;
	}

	.hero-title-container {
		text-align: center;
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 1rem;
	}

	.hero-title {
		font-size: clamp(4.05rem, 6vw, 7.5rem);
		font-weight: 800;
		margin: 0 0 1.5rem 0;
		letter-spacing: -0.02em;
		line-height: 1.1;
		color: #f5f9ff;
		text-shadow:
			0 2px 12px rgba(0, 0, 0, 0.2),
			0 1px 0 rgba(255, 255, 255, 0.01);
	}

	.hero-subtitle {
		font-size: clamp(1.1rem, 3vw, 1.5rem);
		color: rgba(245, 249, 255, 0.85);
		margin-bottom: 1.5rem;
		line-height: 1.6;
		margin-top: 0;
		font-weight: 400;
		font-family: 'Geist', 'Inter', sans-serif;
	}

	.hero-cta-button {
		display: inline-block;
		background: white;
		color: black;
		text-decoration: none;
		padding: 1rem 2rem;
		border-radius: 2rem;
		font-size: 1.1rem;
		font-weight: 600;
		font-family: 'Geist', 'Inter', sans-serif;
		transition: all 0.3s ease;
		box-shadow: 
			0 4px 14px 0 rgba(255, 255, 255, 0.1),
			0 2px 4px 0 rgba(0, 0, 0, 0.1);
		margin-top: 1rem;
	}

	.hero-cta-button:hover {
		transform: translateY(-2px);
		box-shadow: 
			0 8px 25px 0 rgba(255, 255, 255, 0.2),
			0 4px 8px 0 rgba(0, 0, 0, 0.15);
		background: rgba(255, 255, 255, 0.95);
	}

	.hero-cta-button:active {
		transform: translateY(0);
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
		font-size: 1em;
		font-family: 'Geist', 'Inter', sans-serif;
		font-weight: 800;
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

	/* Subsections Section */
	.subsections-section {
		position: relative;
		z-index: 10;
		padding: 6rem 2rem;
		width: 100%;
		flex-shrink: 0;
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
		font-size: clamp(2rem, 5vw, 2.5rem);
		font-weight: 700;
		margin: 0 0 1.5rem 0;
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
		color: var(--color-primary);
		font-weight: 500;
		margin-bottom: 1.5rem;
		line-height: 1.5;
	}

	/* Specific styling for Frictionless Trading section */
	.subsection.frictionless .subsection-title {
		color: white;
	}

	.subsection.frictionless .subsection-description {
		color: white;
	}

	/* Specific styling for Analysis at the speed of now section */
	.subsection.speed-analysis .subsection-description {
		color: white;
	}

	/* Specific styling for Never miss a trade section */
	.subsection.never-miss .subsection-description {
		color: white;
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
		max-width: 1000px;
	}
	
	.subsection:first-child .subsection-image img {
		max-width: 1000px;
	}

	/* Make the study.png image white (first subsection) */
	.subsection:first-child .subsection-image img {
		filter: brightness(0) invert(1);
	}

	/* Responsive Design */
	@media (max-width: 768px) {
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

	@media (max-width: 480px) {
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
