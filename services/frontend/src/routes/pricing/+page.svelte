<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { redirectToCheckout, redirectToCustomerPortal } from '$lib/utils/helpers/stripe';
	import { subscriptionStatus, fetchSubscriptionStatus } from '$lib/utils/stores/stores';
	import { PRICING_CONFIG, getStripePrice, getPlan, formatPrice } from '$lib/config/pricing';

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

	// Run initialization on mount
	onMount(() => {
		// Check if user is authenticated
		if (isGuestAccount()) {
			goto('/login');
			return;
		}
		initializeComponent();
	});
</script>

<svelte:head>
	<title>Pricing & Plans - Atlantis Trading</title>
</svelte:head>

<div class="pricing-container">
	<div class="pricing-header">
		<h1>Pricing & Plans</h1>
		<p>Choose the perfect plan for your trading needs</p>
	</div>

	<!-- Feedback Messages -->
	{#if feedbackMessage}
		<div class="feedback-message {feedbackType}">
			{feedbackMessage}
		</div>
	{/if}

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
							class="btn btn-secondary"
							on:click={handleManageSubscription}
							disabled={loadingStates.manage}
						>
							{loadingStates.manage ? 'Loading...' : 'Manage Subscription'}
						</button>
					{:else}
						<button
							class="btn btn-primary"
							on:click={() => handleUpgrade('plus')}
							disabled={loadingStates.plus}
						>
							{loadingStates.plus ? 'Processing...' : getPlan('plus').cta}
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
							class="btn btn-secondary"
							on:click={handleManageSubscription}
							disabled={loadingStates.manage}
						>
							{loadingStates.manage ? 'Loading...' : 'Manage Subscription'}
						</button>
					{:else}
						<button
							class="btn btn-primary"
							on:click={() => handleUpgrade('pro')}
							disabled={loadingStates.pro}
						>
							{loadingStates.pro ? 'Processing...' : getPlan('pro').cta}
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
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
		<div class="modal-content" on:click|stopPropagation on:keydown|stopPropagation>
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
				<button class="btn btn-secondary" on:click={cancelUpgrade}> Cancel </button>
				<button class="btn btn-primary" on:click={confirmUpgrade}> Continue to Checkout </button>
			</div>
		</div>
	</div>
{/if}

<style>
	.pricing-container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
		font-family: 'Inter', sans-serif;
		min-height: 100vh;
		background-color: var(--c1);
		color: var(--f1);
	}

	.pricing-header {
		text-align: center;
		margin-bottom: 3rem;
	}

	.pricing-header h1 {
		font-size: 2.5rem;
		font-weight: 700;
		color: var(--f1);
		margin-bottom: 0.5rem;
	}

	.pricing-header p {
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

	.feedback-message {
		margin: 1.25rem 0;
		padding: 1rem 1.25rem;
		border-radius: 6px;
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
		background: var(--c1);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 12px;
		max-width: 500px;
		width: 90%;
		max-height: 90vh;
		overflow-y: auto;
		animation: scaleIn 0.2s ease-out;
	}

	.modal-header {
		padding: 1.5rem 1.5rem 0;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
	}

	.modal-header h3 {
		margin: 0 0 1rem 0;
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--f1);
	}

	.modal-body {
		padding: 1.5rem;
	}

	.modal-body p {
		margin: 0 0 1rem 0;
		color: var(--f2);
		line-height: 1.5;
	}

	.modal-body p:last-child {
		margin-bottom: 0;
	}

	.price-info {
		font-size: 1.1rem;
		color: var(--f1) !important;
	}

	.billing-info {
		font-size: 0.9rem;
		color: var(--f3) !important;
	}

	.modal-actions {
		padding: 0 1.5rem 1.5rem;
		display: flex;
		gap: 1rem;
		justify-content: flex-end;
	}

	.modal-actions .btn {
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
		.modal-actions {
			flex-direction: column;
		}

		.modal-actions .btn {
			width: 100%;
		}
	}
</style>
