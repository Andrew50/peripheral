<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { redirectToCheckout, redirectToCustomerPortal } from '$lib/utils/helpers/stripe';
	import { writable } from 'svelte/store';
	import { onMount } from 'svelte';
	import { getStripePrice } from '$lib/config/pricing';

	let subscriptionStatus = writable('inactive');
	let currentPlan = writable('');
	let loading = writable(false);
	let error = writable('');

	// Mock user subscription data - in real app this would come from backend
	let userSubscription = {
		status: 'inactive',
		plan: '',
		currentPeriodEnd: null
	};

	onMount(() => {
		// In a real implementation, you'd fetch user subscription status from backend
		// For now, we'll use mock data
		subscriptionStatus.set(userSubscription.status);
		currentPlan.set(userSubscription.plan);
	});

	async function handleUpgrade(priceId: string) {
		loading.set(true);
		error.set('');

		try {
			const response = await privateRequest<{ sessionId: string; url: string }>(
				'createCheckoutSession',
				{ priceId: getStripePrice(priceId as 'starter' | 'plus' | 'pro') }
			);

			await redirectToCheckout(response.sessionId);
		} catch (err) {
			console.error('Error creating checkout session:', err);
			error.set('Failed to start checkout process. Please try again.');
		} finally {
			loading.set(false);
		}
	}

	async function handleManageSubscription() {
		loading.set(true);
		error.set('');

		try {
			const response = await privateRequest<{ url: string }>('createCustomerPortal', {});
			redirectToCustomerPortal(response.url);
		} catch (err) {
			console.error('Error opening customer portal:', err);
			error.set('Failed to open subscription management. Please try again.');
		} finally {
			loading.set(false);
		}
	}

	$: isActive = $subscriptionStatus === 'active';
	$: isLoading = $loading;
	$: errorMessage = $error;
</script>

<svelte:head>
	<title>Pricing - Atlantis Trading</title>
</svelte:head>

<div class="pricing-container">
	<div class="pricing-header">
		<h1>Pricing & Plans</h1>
		<p>Manage your subscription and billing information</p>
	</div>

	{#if errorMessage}
		<div class="error-banner">
			<p>{errorMessage}</p>
		</div>
	{/if}

	<div class="pricing-content">
		<!-- Current Subscription Status -->
		<div class="current-plan-card">
			<h2>Current Plan</h2>
			{#if isActive}
				<div class="plan-active">
					<div class="plan-badge active">Active</div>
					<h3>{$currentPlan} Plan</h3>
					{#if userSubscription.currentPeriodEnd}
						<p class="renewal-date">
							Renews on {new Date(userSubscription.currentPeriodEnd).toLocaleDateString()}
						</p>
					{/if}
				</div>
				<button class="btn btn-secondary" on:click={handleManageSubscription} disabled={isLoading}>
					{isLoading ? 'Loading...' : 'Manage Subscription'}
				</button>
			{:else}
				<div class="plan-inactive">
					<div class="plan-badge inactive">No Active Subscription</div>
					<p>Choose a plan to unlock premium features</p>
				</div>
			{/if}
		</div>

		<!-- Available Plans -->
		{#if !isActive}
			<div class="plans-section">
				<h2>Choose Your Plan</h2>
				<div class="plans-grid">
					<!-- Starter Plan -->
					<div class="plan-card">
						<div class="plan-header">
							<h3>Starter</h3>
							<div class="plan-price">
								<span class="price">$19</span>
								<span class="period">/month</span>
							</div>
						</div>
						<ul class="plan-features">
							<li>Real-time market data</li>
							<li>Basic charts and analytics</li>
							<li>5 watchlists</li>
							<li>Email support</li>
						</ul>
						<button
							class="btn btn-primary"
							on:click={() => handleUpgrade('starter')}
							disabled={isLoading}
						>
							{isLoading ? 'Loading...' : 'Choose Starter'}
						</button>
					</div>

					<!-- Pro Plan -->
					<div class="plan-card featured">
						<div class="plan-header">
							<div class="popular-badge">Most Popular</div>
							<h3>Pro</h3>
							<div class="plan-price">
								<span class="price">$49</span>
								<span class="period">/month</span>
							</div>
						</div>
						<ul class="plan-features">
							<li>Everything in Starter</li>
							<li>Advanced analytics</li>
							<li>Unlimited watchlists</li>
							<li>AI-powered insights</li>
							<li>Priority support</li>
							<li>Strategy backtesting</li>
						</ul>
						<button
							class="btn btn-primary"
							on:click={() => handleUpgrade('pro')}
							disabled={isLoading}
						>
							{isLoading ? 'Loading...' : 'Choose Pro'}
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<style>
	.pricing-container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
		font-family: 'Inter', sans-serif;
	}

	.pricing-header {
		text-align: center;
		margin-bottom: 3rem;
	}

	.pricing-header h1 {
		font-size: 2.5rem;
		font-weight: 700;
		color: var(--text-primary);
		margin-bottom: 0.5rem;
	}

	.pricing-header p {
		font-size: 1.1rem;
		color: var(--text-secondary);
	}

	.error-banner {
		background: rgba(239, 68, 68, 0.1);
		border: 1px solid rgba(239, 68, 68, 0.3);
		border-radius: 8px;
		padding: 1rem;
		margin-bottom: 2rem;
		color: #ef4444;
		text-align: center;
	}

	.current-plan-card {
		background: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		border-radius: 12px;
		padding: 2rem;
		margin-bottom: 3rem;
	}

	.current-plan-card h2 {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 1.5rem;
	}

	.plan-badge {
		display: inline-block;
		padding: 0.25rem 0.75rem;
		border-radius: 20px;
		font-size: 0.875rem;
		font-weight: 500;
		margin-bottom: 1rem;
	}

	.plan-badge.active {
		background: rgba(34, 197, 94, 0.2);
		color: #22c55e;
	}

	.plan-badge.inactive {
		background: rgba(156, 163, 175, 0.2);
		color: #9ca3af;
	}

	.plan-active h3 {
		font-size: 1.25rem;
		font-weight: 600;
		color: var(--text-primary);
		margin-bottom: 0.5rem;
	}

	.renewal-date {
		color: var(--text-secondary);
		margin-bottom: 1.5rem;
	}

	.plans-section h2 {
		font-size: 2rem;
		font-weight: 600;
		color: var(--text-primary);
		text-align: center;
		margin-bottom: 2rem;
	}

	.plans-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
		gap: 2rem;
		max-width: 800px;
		margin: 0 auto;
	}

	.plan-card {
		background: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		border-radius: 12px;
		padding: 2rem;
		position: relative;
		transition:
			transform 0.2s ease,
			border-color 0.2s ease;
	}

	.plan-card:hover {
		transform: translateY(-4px);
		border-color: var(--accent-primary);
	}

	.plan-card.featured {
		border-color: var(--accent-primary);
		background: linear-gradient(135deg, var(--ui-bg-element) 0%, rgba(59, 130, 246, 0.05) 100%);
	}

	.popular-badge {
		position: absolute;
		top: -0.5rem;
		left: 50%;
		transform: translateX(-50%);
		background: var(--accent-primary);
		color: white;
		padding: 0.25rem 1rem;
		border-radius: 20px;
		font-size: 0.75rem;
		font-weight: 600;
	}

	.plan-header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.plan-header h3 {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--text-primary);
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
		color: var(--text-primary);
	}

	.period {
		font-size: 1rem;
		color: var(--text-secondary);
	}

	.plan-features {
		list-style: none;
		padding: 0;
		margin-bottom: 2rem;
	}

	.plan-features li {
		padding: 0.5rem 0;
		color: var(--text-secondary);
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
		background: var(--accent-primary);
		color: white;
	}

	.btn-primary:hover:not(:disabled) {
		background: #2563eb;
		transform: translateY(-1px);
	}

	.btn-secondary {
		background: transparent;
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
	}

	.btn-secondary:hover:not(:disabled) {
		background: var(--ui-bg-element-darker);
		border-color: var(--accent-primary);
	}

	@media (max-width: 768px) {
		.billing-container {
			padding: 1rem;
		}

		.billing-header h1 {
			font-size: 2rem;
		}

		.plans-grid {
			grid-template-columns: 1fr;
			gap: 1.5rem;
		}

		.plan-card {
			padding: 1.5rem;
		}

		.price {
			font-size: 2.5rem;
		}
	}
</style>
