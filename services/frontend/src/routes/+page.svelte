	<script lang="ts">
		import { browser } from '$app/environment';
		import { onMount } from 'svelte';
		import { goto } from '$app/navigation';
		import { startPricingPreload } from '$lib/utils/pricing-loader';
		import ChipSection from '$lib/landing/ChipSection.svelte';
		import SiteHeader from '$lib/components/SiteHeader.svelte';
		import SiteFooter from '$lib/components/SiteFooter.svelte';
		import HeroAnimation from '$lib/landing/HeroAnimation.svelte';
		import '$lib/styles/splash.css';

		if (browser) {
			document.title = 'Peripheral';
		}

		onMount(() => {

			if (browser) {
				// Start preloading pricing configuration early
				startPricingPreload();
			}
		});
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


	<SiteHeader/>

	<main class="landing-container">
		<HeroAnimation />
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
		<!-- Ideas Chips Section -->
		<ChipSection />
		<!-- Big Centered Tagline Section -->
		<section class="tagline-section">
			<div class="tagline-inner">
				<p class="tagline-pretext">JUMP INTO</p>
				<h2 class="tagline-text">The Final Trading Terminal.</h2>
				<button class="tagline" on:click={() => goto('/signup')}>Get Started</button>
			</div>
		</section>

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

		:global(html), :global(body) {
			margin: 0;
			font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			background: linear-gradient(135deg, var(--color-light) 0%, var(--color-accent) 100%);
			/* Prevent rubber-band / pull-to-refresh scrolling that lets the page scroll above the top */
			overscroll-behavior-y: none;
			overscroll-behavior-x: contain;
		}

		:global(body) {
			background: linear-gradient(135deg, var(--color-light) 0%, var(--color-accent) 100%);
			font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
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
		/* Responsive Design */
		@media (max-width: 768px) {
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
		}

		/* Global styles for proper layout */
		:global(*) {
			box-sizing: border-box;
		}


		.tagline-section {
			width: 100vw;
			padding: 4rem 0 8rem 0;
			display: flex;
			justify-content: center;
			align-items: center;
			background: none;
		}
		.tagline-inner {
			display: flex;
			flex-direction: column;
			align-items: center;
		}
		.tagline-text {
			font-size: clamp(2.5rem, 7vw, 5rem);
			font-weight: 900;
			color: var(--color-dark);
			text-align: center;
			margin: 0;
			letter-spacing: -0.04em;
			line-height: 1.1;
			font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		}
		.tagline {
			margin-top: 3rem;
			font-size: 1.2rem;
			padding: 1.1rem 2.5rem;
			background: rgb(0, 0, 0);
			color: #f5f9ff;
			border: 1px solid transparent;
			border-radius: 999px;
			font-weight: 600;
			cursor: pointer;
			transition: all 0.1s ease;
			box-shadow: none;
			display: inline-flex;
			align-items: center;
			justify-content: center;
			text-decoration: none;
			white-space: nowrap;
		}
		.tagline:hover {
			transform: translateY(-3px);
			box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
		}

		.tagline-pretext {
			font-size: 1.2rem;
			font-weight: 500;
			color: var(--color-primary);
			margin: 0 0 0.5rem 0;
			text-align: center;
		}

		/* Corner glow blooms */
		:global(body)::before {
			content: "";
			position: fixed;
			inset: 0;
			pointer-events: none;
			z-index: -1;
			background:
				radial-gradient(80rem 80rem at 0% 0%,
					rgba(var(--color-accent-rgb,147,177,181),0.55) 0%,
					rgba(var(--color-accent-rgb,147,177,181),0.35) 35%,
					rgba(var(--color-accent-rgb,147,177,181),0.0) 70%),
				radial-gradient(80rem 80rem at 100% 100%,
					rgba(var(--color-dark-rgb,11,46,51),0.55) 0%,
					rgba(var(--color-dark-rgb,11,46,51),0.35) 35%,
					rgba(var(--color-dark-rgb,11,46,51),0.0) 70%);
			filter: blur(120px);
		}


	</style>