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
				description: 'Backtest trading strategies, analyze event or macro trading opportunities, or research investment portfolios in minutes, not days.',
			},
			{
				title: 'Never miss a trade.',
				description: 'Deploy strategies to receive alerts when they trigger in realtime. Our infrastructure delivers alerts down to minute resolution within five seconds of the event triggering.',
				content: ''
			},
			{
				title: 'Analysis at the speed of now',
				description: 'In dynamic, fast-moving markets, every second counts. Peripheral provides quality analysis of news, fundamentals, and data XX% faster than ChatGPT, Perplexity, and comparable services with accurate data.',
				content: ''
			}, 
			{
				title: 'Frictionless trading.',
				description: 'All the insights you need, before you ask. Our context aware terminal automatically surfaces the most relevant news, data, and insights about symbols you care about. Stay in the flow.',
				content: ''
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


	<SiteHeader/>

	<main class="landing-container">
		<HeroAnimation {defaultKey} {chartsByKey} {defaultChartData}/>
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