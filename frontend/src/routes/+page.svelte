<script>
	import Header from '$lib/utils/modules/header.svelte';
	import '$lib/core/global.css';
	import { browser } from '$app/environment';
	import MarketChart from './MarketChart.svelte';
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';

	if (browser) {
		document.title = 'Atlantis';
	}

	let showChart = true;
	let showHero = false;

	onMount(() => {
		setTimeout(() => {
			showChart = false;
			showHero = true;
		}, 3000);
	});
</script>

<main class="main-container">
	{#if showChart}
		<div class="chart-container" transition:fade>
			<MarketChart />
		</div>
	{/if}

	{#if showHero}
		<div class="hero-container" transition:fade>
			<Header />
			<div class="hero-content">
				<h1>ATLANTIS</h1>
				<p class="tagline">Your Gateway to Intelligent Trading</p>
				<div class="cta-buttons">
					<a href="/signup" class="cta-button primary">Get Started Free</a>
					<a href="/login" class="cta-button secondary">Sign In</a>
				</div>
				<div class="features-grid">
					<div class="feature-card">
						<span class="feature-icon">📈</span>
						<h3>Real-time Analytics</h3>
						<p>Advanced market insights at your fingertips</p>
					</div>
					<div class="feature-card">
						<span class="feature-icon">🤖</span>
						<h3>AI-Powered</h3>
						<p>Smart predictions and trading signals</p>
					</div>
					<div class="feature-card">
						<span class="feature-icon">🔒</span>
						<h3>Secure Platform</h3>
						<p>Enterprise-grade security for your assets</p>
					</div>
				</div>
			</div>
		</div>
	{/if}
</main>

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		overflow: hidden;
		background: black;
		font-family: 'Inter', sans-serif;
	}

	.main-container {
		position: relative;
		width: 100vw;
		height: 100vh;
		margin: 0;
		padding: 0;
		overflow: hidden;
	}

	.chart-container {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
	}

	.hero-container {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: linear-gradient(to bottom, #000000, #1a1a2e);
	}

	.hero-content {
		padding-top: 15vh;
		text-align: center;
		height: calc(100vh - 80px);
		overflow-y: auto;
		padding-bottom: 2rem;
	}

	h1 {
		color: #3b82f6;
		font-family: 'Space Mono', monospace;
		font-size: 5rem;
		letter-spacing: 0.8rem;
		margin-bottom: 1rem;
		text-transform: uppercase;
	}

	.tagline {
		color: #94a3b8;
		font-size: 1.5rem;
		margin-bottom: 2rem;
	}

	.cta-buttons {
		display: flex;
		gap: 1rem;
		justify-content: center;
		margin-bottom: 4rem;
	}

	.cta-button {
		padding: 1rem 2rem;
		border-radius: 8px;
		font-weight: 600;
		text-decoration: none;
		transition: all 0.3s ease;
	}

	.primary {
		background: #3b82f6;
		color: white;
	}

	.primary:hover {
		background: #2563eb;
		transform: translateY(-2px);
	}

	.secondary {
		background: transparent;
		color: #3b82f6;
		border: 2px solid #3b82f6;
	}

	.secondary:hover {
		background: rgba(59, 130, 246, 0.1);
		transform: translateY(-2px);
	}

	.features-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
		gap: 2rem;
		padding: 0 2rem;
		max-width: 1200px;
		margin: 0 auto;
		max-height: calc(100vh - 80px - 15vh - 15rem);
	}

	.feature-card {
		background: rgba(255, 255, 255, 0.05);
		padding: 2rem;
		border-radius: 12px;
		text-align: center;
		transition: transform 0.3s ease;
	}

	.feature-card:hover {
		transform: translateY(-5px);
	}

	.feature-icon {
		font-size: 2.5rem;
		margin-bottom: 1rem;
	}

	.feature-card h3 {
		color: #e2e8f0;
		margin-bottom: 0.5rem;
	}

	.feature-card p {
		color: #94a3b8;
		font-size: 0.9rem;
	}
</style>
