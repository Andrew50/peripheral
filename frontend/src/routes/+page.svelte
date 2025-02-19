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

	let showHero = false;

	onMount(() => {
		setTimeout(() => {
			showHero = true;
		}, 1000);
	});
</script>

<main class="main-container">
	<div class="chart-container">
		<MarketChart />
	</div>

	{#if showHero}
		<div class="hero-container" transition:fade>
			<Header />
			<div class="hero-content">
				<h1>ATLANTIS</h1>
				<p class="tagline">Comprehensive Market Analysis Suite</p>
				<div class="cta-buttons">
					<a href="/signup" class="cta-button primary">Get Started</a>
					<a href="/login" class="cta-button secondary">Sign In</a>
				</div>
				<div class="features-grid">
					<div class="feature-card">
						<div class="feature-header">
							<span class="feature-icon">📊</span>
							<h3>Advanced Charting</h3>
						</div>
						<p>Real-time data & historical replay with custom indicators</p>
					</div>
					<div class="feature-card">
						<div class="feature-header">
							<span class="feature-icon">🎯</span>
							<h3>Pattern Recognition</h3>
						</div>
						<p>Custom screening patterns with real-time market alerts</p>
					</div>
					<div class="feature-card">
						<div class="feature-header">
							<span class="feature-icon">📈</span>
							<h3>Trade Analysis</h3>
						</div>
						<p>Comprehensive trade journaling and performance metrics</p>
					</div>
					<div class="feature-card">
						<div class="feature-header">
							<span class="feature-icon">⚡</span>
							<h3>Market Data</h3>
						</div>
						<p>Time & sales, level 2, and full US market coverage</p>
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
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 1;
		opacity: 0.15;
		pointer-events: none;
	}

	.hero-container {
		position: relative;
		z-index: 2;
		background: linear-gradient(to bottom, rgba(0, 0, 0, 0.85), rgba(26, 26, 46, 0.9));
		min-height: 100vh;
	}

	.hero-content {
		padding-top: 12vh;
		text-align: center;
		height: calc(100vh - 80px);
		overflow-y: auto;
		padding-bottom: 2rem;
	}

	h1 {
		color: #3b82f6;
		font-family: 'Space Mono', monospace;
		font-size: 4rem;
		letter-spacing: 0.5rem;
		margin-bottom: 0.5rem;
		text-transform: uppercase;
	}

	.tagline {
		color: #94a3b8;
		font-size: 1.25rem;
		margin-bottom: 2rem;
		font-weight: 300;
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
		grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
		gap: 1.5rem;
		padding: 0 2rem;
		max-width: 1200px;
		margin: 0 auto;
	}

	.feature-card {
		background: rgba(30, 41, 59, 0.5);
		padding: 1.5rem;
		border-radius: 12px;
		text-align: left;
		transition: transform 0.3s ease;
		border: 1px solid rgba(59, 130, 246, 0.1);
	}

	.feature-header {
		display: flex;
		align-items: center;
		margin-bottom: 0.75rem;
		gap: 0.75rem;
	}

	.feature-icon {
		font-size: 1.5rem;
	}

	.feature-card h3 {
		color: #e2e8f0;
		margin: 0;
		font-size: 1.1rem;
		font-weight: 500;
	}

	.feature-card p {
		color: #94a3b8;
		font-size: 0.9rem;
		line-height: 1.4;
		margin: 0;
	}

	.feature-card:hover {
		transform: translateY(-2px);
		background: rgba(30, 41, 59, 0.7);
		border-color: rgba(59, 130, 246, 0.2);
	}
</style>
