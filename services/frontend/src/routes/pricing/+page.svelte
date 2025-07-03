<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { redirectToCheckout, redirectToCustomerPortal } from '$lib/utils/helpers/stripe';
	import { subscriptionStatus, fetchSubscriptionStatus } from '$lib/utils/stores/stores';
	import { PRICING_CONFIG, getStripePrice, getPlan, formatPrice } from '$lib/config/pricing';
	import '$lib/styles/global.css';
	import '$lib/styles/landing.css';

	// Individual loading states for better UX
	let loadingStates = {
		plus: false,
		pro: false,
		manage: false
	};

	// Success/error feedback
	let feedbackMessage = '';
	let feedbackType: 'success' | 'error' | '' = '';

	// Show confirmation modal for upgrades
	let showConfirmation = false;
	let pendingUpgrade: { plan: string; price: number; planName: string } | null = null;

	// Component loading state
	let isLoaded = false;

	// Function to determine if the current user is a guest
	const isGuestAccount = (): boolean => {
		if (!browser) return true;
		const username = sessionStorage.getItem('username');
		return username === 'Guest' || !username;
	};

	// Initialize component
	async function initializeComponent() {
		if (!isGuestAccount()) {
			await fetchSubscriptionStatus();
		}
	}

	// Enhanced upgrade handler with individual loading states
	async function handleUpgrade(planKey: 'plus' | 'pro') {
		// Check if user is authenticated before allowing upgrade
		if (isGuestAccount()) {
			goto('/login');
			return;
		}

		const plan = getPlan(planKey);

		// Show confirmation for expensive plans
		if (plan.price >= 100) {
			pendingUpgrade = {
				plan: planKey,
				price: plan.price,
				planName: plan.name
			};
			showConfirmation = true;
			return;
		}

		await processUpgrade(planKey);
	}

	// Process the actual upgrade
	async function processUpgrade(planKey: 'plus' | 'pro') {
		// Double-check authentication before processing payment
		if (isGuestAccount()) {
			goto('/login');
			return;
		}

		loadingStates[planKey] = true;
		feedbackMessage = '';
		feedbackType = '';

		try {
			const priceId = getStripePrice(planKey);
			const response = await privateRequest<{ sessionId: string; url: string }>(
				'createCheckoutSession',
				{ priceId }
			);

			// Show success message briefly before redirect
			feedbackMessage = 'Redirecting to secure checkout...';
			feedbackType = 'success';

			// Small delay for user feedback
			setTimeout(async () => {
				await redirectToCheckout(response.sessionId);
			}, 1000);
		} catch (error) {
			console.error('Error creating checkout session:', error);
			feedbackMessage = 'Failed to start checkout. Please try again.';
			feedbackType = 'error';
			loadingStates[planKey] = false;
		}
	}

	// Enhanced manage subscription with individual loading
	async function handleManageSubscription() {
		// Check if user is authenticated before allowing management
		if (isGuestAccount()) {
			goto('/login');
			return;
		}

		loadingStates.manage = true;
		feedbackMessage = '';
		feedbackType = '';

		try {
			const response = await privateRequest<{ url: string }>('createCustomerPortal', {});
			feedbackMessage = 'Redirecting to customer portal...';
			feedbackType = 'success';

			setTimeout(() => {
				redirectToCustomerPortal(response.url);
			}, 500);
		} catch (error) {
			console.error('Error opening customer portal:', error);
			feedbackMessage = 'Failed to open customer portal. Please try again.';
			feedbackType = 'error';
			loadingStates.manage = false;
		}
	}

	// Confirm upgrade
	function confirmUpgrade() {
		if (pendingUpgrade) {
			processUpgrade(pendingUpgrade.plan as 'plus' | 'pro');
			showConfirmation = false;
			pendingUpgrade = null;
		}
	}

	// Cancel upgrade
	function cancelUpgrade() {
		showConfirmation = false;
		pendingUpgrade = null;
	}

	// Clear feedback message after timeout
	function clearFeedback() {
		setTimeout(() => {
			if (feedbackType === 'error') {
				feedbackMessage = '';
				feedbackType = '';
			}
		}, 5000);
	}

	// Watch for feedback changes to auto-clear errors
	$: if (feedbackMessage && feedbackType === 'error') {
		clearFeedback();
	}

	function navigateToHome() {
		goto('/');
	}

	// Run initialization on mount
	onMount(() => {
		if (browser) {
			document.title = 'Pricing & Plans - Peripheral';
			isLoaded = true;
		}
		// Initialize component for both authenticated and guest users
		// Guest users can view pricing, but will be redirected to login when attempting to upgrade
		initializeComponent();
	});
</script>

<svelte:head>
	<title>Pricing & Plans - Peripheral</title>
</svelte:head>

<!-- Use landing page design system -->
<div class="landing-background landing-reset">
	<!-- Background Effects -->
	<div class="landing-background-animation">
		<div class="landing-gradient-orb landing-orb-1"></div>
		<div class="landing-gradient-orb landing-orb-2"></div>
		<div class="landing-gradient-orb landing-orb-3"></div>
		<div class="landing-static-gradient"></div>
	</div>

	<!-- Header -->
	<header class="landing-header">
		<div class="landing-header-content">
			<div class="logo-section">
				<img
					src="/atlantis_logo_transparent.png"
					alt="Peripheral Logo"
					class="landing-logo"
					on:click={navigateToHome}
					style="cursor: pointer;"
				/>
			</div>
			<nav class="landing-nav">
				<button class="landing-button secondary" on:click={navigateToHome}>← Back to Home</button>
			</nav>
		</div>
	</header>

	<!-- Main Pricing Content -->
	<div class="landing-container" style="padding-top: 120px;">
		<div class="pricing-content landing-fade-in" class:loaded={isLoaded}>
			<!-- Hero Section -->
			<div class="pricing-hero">
				<h1 class="landing-title">Pricing & Plans</h1>
				<p class="landing-subtitle">Choose the perfect plan for your trading needs</p>
			</div>

			<!-- Feedback Messages -->
			{#if feedbackMessage}
				<div class="feedback-message {feedbackType}">
					{feedbackMessage}
				</div>
			{/if}

			{#if $subscriptionStatus.loading}
				<div class="loading-message">
					<div class="landing-loader"></div>
					<span>Loading subscription information...</span>
				</div>
			{:else if $subscriptionStatus.error && !isGuestAccount()}
				<div class="error-message">{$subscriptionStatus.error}</div>
			{:else}
				<!-- Available Plans -->
				<div class="plans-section">
					<div class="plans-grid">
						<!-- Free Plan -->
						<div
							class="plan-card landing-glass-card {!$subscriptionStatus.isActive &&
							!isGuestAccount()
								? 'current-plan'
								: ''}"
						>
							<div class="plan-header">
								{#if !$subscriptionStatus.isActive && !isGuestAccount()}
									<div class="current-badge">Current Plan</div>
								{/if}
								<h3>{getPlan('free').name}</h3>
								<div class="plan-price">
									<span class="price">{formatPrice(getPlan('free').price)}</span>
									<span class="period">{getPlan('free').period}</span>
								</div>
							</div>
							<ul class="plan-features">
								{#each getPlan('free').features as feature}
									<li>{feature}</li>
								{/each}
							</ul>
							{#if !$subscriptionStatus.isActive && !isGuestAccount()}
								<button class="landing-button primary full-width current" disabled>
									Current Plan
								</button>
							{:else if $subscriptionStatus.isActive}
								<button class="landing-button secondary full-width" disabled>
									Downgrade not available
								</button>
							{:else}
								<button class="landing-button secondary full-width" disabled> Free Plan </button>
							{/if}
						</div>

						<!-- Plus Plan -->
						<div
							class="plan-card landing-glass-card {$subscriptionStatus.currentPlan === 'Plus'
								? 'current-plan'
								: ''}"
						>
							<div class="plan-header">
								{#if $subscriptionStatus.currentPlan === 'Plus'}
									<div class="current-badge">Current Plan</div>
								{/if}
								<h3>{getPlan('plus').name}</h3>
								<div class="plan-price">
									<span class="price">{formatPrice(getPlan('plus').price)}</span>
									<span class="period">{getPlan('plus').period}</span>
								</div>
							</div>
							<ul class="plan-features">
								{#each getPlan('plus').features as feature}
									<li>{feature}</li>
								{/each}
							</ul>
							{#if $subscriptionStatus.currentPlan === 'Plus'}
								<button
									class="landing-button secondary full-width"
									on:click={handleManageSubscription}
									disabled={loadingStates.manage}
								>
									{#if loadingStates.manage}
										<div class="landing-loader"></div>
									{:else}
										Manage Subscription
									{/if}
								</button>
							{:else}
								<button
									class="landing-button primary full-width"
									on:click={() => handleUpgrade('plus')}
									disabled={loadingStates.plus}
								>
									{#if loadingStates.plus}
										<div class="landing-loader"></div>
									{:else}
										{isGuestAccount() ? 'Sign Up to Upgrade' : getPlan('plus').cta}
									{/if}
								</button>
							{/if}
						</div>

						<!-- Pro Plan -->
						<div
							class="plan-card landing-glass-card featured {$subscriptionStatus.currentPlan ===
							'Pro'
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
								<h3>{getPlan('pro').name}</h3>
								<div class="plan-price">
									<span class="price">{formatPrice(getPlan('pro').price)}</span>
									<span class="period">{getPlan('pro').period}</span>
								</div>
							</div>
							<ul class="plan-features">
								{#each getPlan('pro').features as feature}
									<li>{feature}</li>
								{/each}
							</ul>
							{#if $subscriptionStatus.currentPlan === 'Pro'}
								<button
									class="landing-button secondary full-width"
									on:click={handleManageSubscription}
									disabled={loadingStates.manage}
								>
									{#if loadingStates.manage}
										<div class="landing-loader"></div>
									{:else}
										Manage Subscription
									{/if}
								</button>
							{:else}
								<button
									class="landing-button primary full-width"
									on:click={() => handleUpgrade('pro')}
									disabled={loadingStates.pro}
								>
									{#if loadingStates.pro}
										<div class="landing-loader"></div>
									{:else}
										{isGuestAccount() ? 'Sign Up to Upgrade' : getPlan('pro').cta}
									{/if}
								</button>
							{/if}
						</div>
					</div>
				</div>
			{/if}
		</div>
	</div>
</div>

<!-- Confirmation Modal -->
{#if showConfirmation && pendingUpgrade}
	<div
		class="modal-overlay"
		on:click={cancelUpgrade}
		on:keydown={(e) => e.key === 'Escape' && cancelUpgrade()}
		role="dialog"
		aria-modal="true"
		aria-labelledby="modal-title"
		tabindex="-1"
	>
		<div
			class="modal-content landing-glass-card"
			on:click|stopPropagation
			on:keydown|stopPropagation
		>
			<div class="modal-header">
				<h3 id="modal-title">Confirm Upgrade</h3>
			</div>
			<div class="modal-body">
				<p>You're about to upgrade to the <strong>{pendingUpgrade.planName}</strong> plan.</p>
				<p class="price-info">Price: <strong>{formatPrice(pendingUpgrade.price)}/month</strong></p>
				<p class="billing-info">
					You'll be redirected to Stripe's secure checkout to complete your subscription.
				</p>
			</div>
			<div class="modal-actions">
				<button class="landing-button secondary" on:click={cancelUpgrade}>Cancel</button>
				<button class="landing-button primary" on:click={confirmUpgrade}
					>Continue to Checkout</button
				>
			</div>
		</div>
	</div>
{/if}

<style>
	/* Pricing-specific styles that build on landing system */
	.pricing-content {
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 2rem;
	}

	.pricing-hero {
		text-align: center;
		margin-bottom: 3rem;
	}

	.loading-message {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 1rem;
		text-align: center;
		color: var(--landing-text-secondary);
		padding: 2rem;
		font-size: 1.1rem;
	}

	.error-message {
		margin: 1.25rem 0;
		padding: 1rem 1.25rem;
		background-color: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border-radius: 8px;
		font-size: 0.9375rem;
		text-align: center;
		border: 1px solid rgba(239, 68, 68, 0.2);
	}

	.feedback-message {
		margin: 1.25rem 0;
		padding: 1rem 1.25rem;
		border-radius: 8px;
		font-size: 0.9375rem;
		text-align: center;
		font-weight: 500;
		animation: slideIn 0.3s ease-out;
	}

	.feedback-message.success {
		background-color: rgba(34, 197, 94, 0.1);
		color: #22c55e;
		border: 1px solid rgba(34, 197, 94, 0.2);
	}

	.feedback-message.error {
		background-color: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border: 1px solid rgba(239, 68, 68, 0.2);
	}

	@keyframes slideIn {
		from {
			opacity: 0;
			transform: translateY(-10px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.plans-section {
		margin-bottom: 3rem;
	}

	.plans-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 2rem;
		margin-top: 2rem;
	}

	@media (max-width: 1024px) {
		.plans-grid {
			grid-template-columns: 1fr;
			max-width: 450px;
			margin: 2rem auto 0;
		}
	}

	.plan-card {
		padding: 2rem;
		position: relative;
		transition: all 0.3s ease;
		display: flex;
		flex-direction: column;
	}

	.plan-card:hover {
		transform: translateY(-5px);
		border-color: var(--landing-border-focus);
	}

	.plan-card.featured {
		border-color: var(--landing-accent-blue);
	}

	.plan-card.current-plan {
		border-color: var(--landing-success);
		background: rgba(34, 197, 94, 0.05);
	}

	.popular-badge {
		position: absolute;
		top: -8px;
		left: 50%;
		transform: translateX(-50%);
		background: var(--landing-accent-blue);
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
		background: var(--landing-success);
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
		color: var(--landing-text-primary);
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
		color: var(--landing-text-primary);
	}

	.period {
		font-size: 1rem;
		color: var(--landing-text-secondary);
	}

	.plan-features {
		list-style: none;
		padding: 0;
		margin-bottom: 2rem;
		flex-grow: 1;
	}

	.plan-features li {
		padding: 0.5rem 0;
		color: var(--landing-text-secondary);
		position: relative;
		padding-left: 1.5rem;
	}

	.plan-features li::before {
		content: '✓';
		position: absolute;
		left: 0;
		color: var(--landing-success);
		font-weight: 600;
	}

	/* Button modifications for current plan state */
	.landing-button.current {
		background: rgba(34, 197, 94, 0.2);
		color: var(--landing-success);
		border: 1px solid rgba(34, 197, 94, 0.3);
	}

	/* Modal Styles */
	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: rgba(0, 0, 0, 0.7);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		animation: fadeIn 0.2s ease-out;
	}

	.modal-content {
		max-width: 500px;
		width: 90%;
		max-height: 90vh;
		overflow-y: auto;
		animation: scaleIn 0.2s ease-out;
	}

	.modal-header {
		padding: 1.5rem 1.5rem 0;
		border-bottom: 1px solid var(--landing-border);
	}

	.modal-header h3 {
		margin: 0 0 1rem 0;
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--landing-text-primary);
	}

	.modal-body {
		padding: 1.5rem;
	}

	.modal-body p {
		margin: 0 0 1rem 0;
		color: var(--landing-text-secondary);
		line-height: 1.5;
	}

	.modal-body p:last-child {
		margin-bottom: 0;
	}

	.price-info {
		font-size: 1.1rem;
		color: var(--landing-text-primary) !important;
	}

	.billing-info {
		font-size: 0.9rem;
		color: var(--landing-text-muted) !important;
	}

	.modal-actions {
		padding: 0 1.5rem 1.5rem;
		display: flex;
		gap: 1rem;
		justify-content: flex-end;
	}

	.modal-actions .landing-button {
		width: auto;
		min-width: 120px;
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	@keyframes scaleIn {
		from {
			opacity: 0;
			transform: scale(0.9);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	@media (max-width: 640px) {
		.pricing-content {
			padding: 0 1rem;
		}

		.plan-card {
			padding: 1.5rem;
		}

		.modal-actions {
			flex-direction: column;
		}

		.modal-actions .landing-button {
			width: 100%;
		}
	}
</style>
