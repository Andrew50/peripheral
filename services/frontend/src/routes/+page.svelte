<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	// Pricing preload removed - now handled directly in pricing page
	import ChipSection from '$lib/landing/ChipSection.svelte';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import HeroAnimation from '$lib/landing/HeroAnimation.svelte';
	import '$lib/styles/splash.css';
	import { getAuthState, getCookie } from '$lib/auth';

	if (browser) {
		document.title = 'Peripheral';
	}

	// Auth state - check immediately to prevent flash
	let isAuthenticated = getAuthState();

	onMount(() => {
		if (browser) {
			// Start preloading pricing configuration early
			// Pricing preload removed - now handled directly in pricing page
		}
	});

	// Subsections data
	const subsections = [
		{
			title: 'Transform ideas into edge in minutes',
			description:
				'Backtest trading strategies, analyze event or macro trading opportunities, or research investment portfolios in minutes, not days.',
			content: '',
			image: '/study.png'
		},
		{
			title: 'Never miss a trade.',
			description:
				'Deploy strategies to receive alerts when they trigger in realtime. Our infrastructure delivers alerts down to minute resolution within five seconds of the event triggering.',
			content: '',
			image: '/splash-deploy.png'
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

	// Data returned from the server-side `load` function
	export let data: {
		defaultKey: string;
		chartsByKey: Record<string, any>;
		defaultChartData: any;
	};

	const { defaultKey, chartsByKey, defaultChartData } = data;
</script> 

<SiteHeader {isAuthenticated} />

<main class="landing-container">
	<HeroAnimation {defaultKey} {chartsByKey} />
	<!-- Subsections -->
	<section class="subsections-section">
		<div class="subsections-content">
			{#each subsections as subsection, index}
				<div class="subsection" class:reverse={index % 2 === 0}>
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
	<!-- Ideas Chips Section -->
	<ChipSection />
	<!-- Footer -->
	<SiteFooter />
</main>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700;800&display=swap');
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');

	/* Global styles */
	:global(*) {
		box-sizing: border-box;
	}

	.landing-container {
		position: relative;
		width: 100%;
		background: linear-gradient(
			180deg,
			rgba(5, 1, 136, 0) 0%,
			rgba(3, 1, 85, 0.75) 50%,
			#010022 100%
		);
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

	.subsection-description {
		font-size: 1.2rem;
		color: var(--color-primary);
		font-weight: 500;
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
		max-width: 600px;
		height: auto;
		image-rendering: -webkit-optimize-contrast;
		-ms-interpolation-mode: bicubic;
	}

	/* Responsive Design */
	@media (max-width: 768px) {
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
		.subsection {
			gap: 1.5rem;
			margin-bottom: 3rem;
			padding: 1.5rem 0;
		}
	}

	/* Global styles for proper layout */
	:global(*) {
		box-sizing: border-box;
	}

	/* Background applied directly to landing container above */
</style>
