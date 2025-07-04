<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { redirectToCheckout, redirectToCustomerPortal } from '$lib/utils/helpers/stripe';
	import { subscriptionStatus, fetchSubscriptionStatus } from '$lib/utils/stores/stores';
	import { PRICING_CONFIG, getStripePrice, formatPrice } from '$lib/config/pricing';

	// Initialize component
	async function initializeComponent() {
		await fetchSubscriptionStatus();
	}

	// Handle subscription upgrade
	async function handleUpgrade(priceId: string) {
		subscriptionStatus.update((s) => ({ ...s, loading: true, error: '' }));

		try {
			const response = await privateRequest<{ sessionId: string; url: string }>(
				'createCheckoutSession',
				{ priceId: getStripePrice(priceId as 'plus' | 'pro') }
			);
			await redirectToCheckout(response.sessionId);
		} catch (error) {
			console.error('Error creating checkout session:', error);
			subscriptionStatus.update((s) => ({
				...s,
				loading: false,
				error: 'Failed to start checkout process. Please try again.'
			}));
		}
	}

	// Handle manage subscription
	async function handleManageSubscription() {
		subscriptionStatus.update((s) => ({ ...s, loading: true, error: '' }));

		try {
			const response = await privateRequest<{ url: string }>('createCustomerPortal', {});
			redirectToCustomerPortal(response.url);
		} catch (error) {
			console.error('Error opening customer portal:', error);
			subscriptionStatus.update((s) => ({
				...s,
				loading: false,
				error: 'Failed to open subscription management. Please try again.'
			}));
		}
	}

	// Run initialization on mount
	onMount(() => {
		initializeComponent();
	});
</script>

<svelte:head>
	<title>Billing & Subscription - Atlantis Trading</title>
</svelte:head>

<div class="billing-container">
	<div class="billing-header">
		<h1>Billing & Subscription</h1>
		<p>Manage your subscription and billing information</p>
	</div>

	{#if $subscriptionStatus.loading}
		<div class="loading-message">Loading subscription information...</div>
	{:else if $subscriptionStatus.error}
		<div class="error-message">{$subscriptionStatus.error}</div>
	{:else}
		<!-- Available Plans -->
		<div class="plans-section">
			<h2>Choose Your Plan</h2>
			<div class="plans-grid">
				<!-- Free Plan -->
				<div class="plan-card {!$subscriptionStatus.isActive ? 'current-plan' : ''}">
					<div class="plan-header">
						{#if !$subscriptionStatus.isActive}
							<div class="current-badge">Current Plan</div>
						{/if}
						<h3>Free</h3>
						<div class="plan-price">
							<span class="price">$0</span>
							<span class="period">/month</span>
						</div>
					</div>
					<ul class="plan-features">
						<li>Delayed charting</li>
						<li>5 queries</li>
						<li>Watchlists</li>
					</ul>
					{#if !$subscriptionStatus.isActive}
						<button class="btn btn-current" disabled> Current Plan </button>
					{:else}
						<button class="btn btn-secondary" disabled> Downgrade not available </button>
					{/if}
				</div>

				<!-- Plus Plan -->
				<div class="plan-card {$subscriptionStatus.currentPlan === 'Plus' ? 'current-plan' : ''}">
					<div class="plan-header">
						{#if $subscriptionStatus.currentPlan === 'Plus'}
							<div class="current-badge">Current Plan</div>
						{/if}
						<h3>Plus</h3>
						<div class="plan-price">
							<span class="price">$99</span>
							<span class="period">/month</span>
						</div>
					</div>
					<ul class="plan-features">
						<li>Realtime charting</li>
						<li>250 queries</li>
						<li>5 strategy alerts</li>
						<li>Single strategy screening</li>
						<li>100 news or price alerts</li>
					</ul>
					{#if $subscriptionStatus.currentPlan === 'Plus'}
						<button
							class="btn btn-secondary"
							on:click={handleManageSubscription}
							disabled={$subscriptionStatus.loading}
						>
							{$subscriptionStatus.loading ? 'Loading...' : 'Manage Subscription'}
						</button>
					{:else}
						<button
							class="btn btn-primary"
							on:click={() => handleUpgrade('plus')}
							disabled={$subscriptionStatus.loading}
						>
							{$subscriptionStatus.loading ? 'Loading...' : 'Choose Plus'}
						</button>
					{/if}
				</div>

				<!-- Pro Plan -->
				<div
					class="plan-card featured {$subscriptionStatus.currentPlan === 'Pro'
						? 'current-plan'
						: ''}"
				>
					<div class="plan-header">
						{#if $subscriptionStatus.currentPlan !== 'Pro'}
							<div class="popular-badge">Most Popular</div>
						{/if}
						{#if $subscriptionStatus.currentPlan === 'Pro'}
							<div class="current-badge">Current Plan</div>
						{/if}
						<h3>Pro</h3>
						<div class="plan-price">
							<span class="price">$199</span>
							<span class="period">/month</span>
						</div>
					</div>
					<ul class="plan-features">
						<li>Sub 1 minute charting</li>
						<li>Multi chart</li>
						<li>1000 queries</li>
						<li>20 strategy alerts</li>
						<li>Multi strategy screening</li>
						<li>400 alerts</li>
						<li>Watchlist alerts</li>
					</ul>
					{#if $subscriptionStatus.currentPlan === 'Pro'}
						<button
							class="btn btn-secondary"
							on:click={handleManageSubscription}
							disabled={$subscriptionStatus.loading}
						>
							{$subscriptionStatus.loading ? 'Loading...' : 'Manage Subscription'}
						</button>
					{:else}
						<button
							class="btn btn-primary"
							on:click={() => handleUpgrade('pro')}
							disabled={$subscriptionStatus.loading}
						>
							{$subscriptionStatus.loading ? 'Loading...' : 'Choose Pro'}
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.billing-container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
		font-family: 'Inter', sans-serif;
		min-height: 100vh;
		background-color: var(--c1);
		color: var(--f1);
	}

	.billing-header {
		text-align: center;
		margin-bottom: 3rem;
	}

	.billing-header h1 {
		font-size: 2.5rem;
		font-weight: 700;
		color: var(--f1);
		margin-bottom: 0.5rem;
	}

	.billing-header p {
		font-size: 1.1rem;
		color: var(--f2);
	}

	.loading-message {
		text-align: center;
		color: var(--f2);
		padding: 2rem;
		font-size: 1.1rem;
	}

	.error-message {
		margin: 1.25rem 0;
		padding: 1rem 1.25rem;
		background-color: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border-radius: 6px;
		font-size: 0.9375rem;
		text-align: center;
		border: 1px solid rgba(239, 68, 68, 0.2);
	}

	.plans-section {
		margin-bottom: 3rem;
	}

	.plans-section h2 {
		font-size: 2rem;
		font-weight: 600;
		color: var(--f1);
		text-align: center;
		margin-bottom: 2rem;
	}

	.plans-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 1.5rem;
		margin-top: 1rem;
	}

	@media (max-width: 1024px) {
		.plans-grid {
			grid-template-columns: 1fr;
			max-width: 400px;
			margin: 1rem auto 0;
		}
	}

	.plan-card {
		background: rgba(255, 255, 255, 0.03);
		border: 1px solid rgba(255, 255, 255, 0.08);
		border-radius: 8px;
		padding: 1.5rem;
		position: relative;
		transition: all 0.2s ease;
	}

	.plan-card:hover {
		border-color: rgba(255, 255, 255, 0.15);
	}

	.plan-card.featured {
		border-color: var(--c3);
	}

	.plan-card.current-plan {
		border-color: #22c55e;
		background: rgba(34, 197, 94, 0.05);
	}

	.popular-badge {
		position: absolute;
		top: -8px;
		left: 50%;
		transform: translateX(-50%);
		background: var(--c3);
		color: white;
		font-size: 0.75rem;
		font-weight: 600;
		padding: 0.25rem 0.75rem;
		border-radius: 12px;
	}

	.current-badge {
		position: absolute;
		top: -8px;
		left: 50%;
		transform: translateX(-50%);
		background: #22c55e;
		color: white;
		font-size: 0.75rem;
		font-weight: 600;
		padding: 0.25rem 0.75rem;
		border-radius: 12px;
	}

	.plan-header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.plan-header h3 {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--f1);
		margin-bottom: 1rem;
	}

	.plan-price {
		display: flex;
		align-items: baseline;
		justify-content: center;
		gap: 0.25rem;
	}

	.price {
		font-size: 3rem;
		font-weight: 700;
		color: var(--f1);
	}

	.period {
		font-size: 1rem;
		color: var(--f2);
	}

	.plan-features {
		list-style: none;
		padding: 0;
		margin-bottom: 2rem;
	}

	.plan-features li {
		padding: 0.5rem 0;
		color: var(--f2);
		position: relative;
		padding-left: 1.5rem;
	}

	.plan-features li::before {
		content: '✓';
		position: absolute;
		left: 0;
		color: #22c55e;
		font-weight: 600;
	}

	.btn {
		width: 100%;
		padding: 0.75rem 1.5rem;
		border: none;
		border-radius: 8px;
		font-size: 1rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.2s ease;
		font-family: 'Inter', sans-serif;
	}

	.btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--c3);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--c3-hover);
		transform: translateY(-1px);
	}

	.btn-secondary {
		background: transparent;
		color: var(--f1);
		border: 1px solid rgba(255, 255, 255, 0.2);
	}

	.btn-secondary:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.2);
	}

	.btn-current {
		background: rgba(34, 197, 94, 0.2);
		color: #22c55e;
		border: 1px solid rgba(34, 197, 94, 0.3);
	}
</style>
