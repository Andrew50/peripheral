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

	// Component loading state
	let isLoaded = false;

	// Function to determine if the current user is authenticated
	const isAuthenticated = (): boolean => {
		if (!browser) return false;
		const authToken = sessionStorage.getItem('authToken');
		return !!authToken;
	};
	const validateAuthentication = async (): Promise<boolean> => {
		console.log('🔍 [validateAuthentication] Starting validation...');

		if (!browser) {
			console.log('🔍 [validateAuthentication] Not in browser, returning false');
			return false;
		}

		const authToken = sessionStorage.getItem('authToken');
		if (!authToken) {
			console.log('🔍 [validateAuthentication] No auth token found');
			return false;
		}

		console.log(
			'🔍 [validateAuthentication] Found token, verifying with backend. Token preview:',
			authToken.substring(0, 20) + '...'
		);

		try {
			// Make a lightweight request to verify the token is still valid
			console.log('🔍 [validateAuthentication] Making verifyAuth request...');
			await privateRequest('verifyAuth', {});
			console.log('✅ [validateAuthentication] Token is valid!');
			return true;
		} catch (error) {
			console.log('❌ [validateAuthentication] Token is invalid:', error);
			// Token is invalid, clear it
			console.log('🧹 [validateAuthentication] Clearing invalid auth data...');
			sessionStorage.removeItem('authToken');
			sessionStorage.removeItem('profilePic');
			sessionStorage.removeItem('username');
			return false;
		}
	};

	// Initialize component
	async function initializeComponent() {
		const isAuth = isAuthenticated();

		if (isAuth) {
			await fetchSubscriptionStatus();
			console.log('📊 [initializeComponent] Subscription status fetch completed');
		} else {
			console.log(
				'ℹ️ [initializeComponent] User not authenticated, skipping subscription status fetch'
			);
		}
	}

	// Enhanced upgrade handler with individual loading states
	async function handleUpgrade(planKey: 'plus' | 'pro') {
		// Check if user is authenticated before allowing upgrade
		const isValidAuth = await validateAuthentication();

		if (!isValidAuth) {
			// Redirect to signup with plan information for deep linking
			goto(`/signup?plan=${planKey}&redirect=checkout`);
			return;
		}

		await processUpgrade(planKey);
	}

	// Process the actual upgrade
	async function processUpgrade(planKey: 'plus' | 'pro') {
		// Double-check authentication before processing payment
		const isValidAuth = await validateAuthentication();

		if (!isValidAuth) {
			console.log(
				'❌ [processUpgrade] Authentication failed during payment, redirecting to signup'
			);
			goto(`/signup?plan=${planKey}&redirect=checkout`);
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

			// Redirect immediately to checkout
			await redirectToCheckout(response.sessionId);
		} catch (error) {
			console.error('❌ [processUpgrade] Error creating checkout session:', error);
			feedbackMessage = 'Failed to start checkout. Please try again.';
			feedbackType = 'error';
			loadingStates[planKey] = false;
		}
	}

	// Enhanced manage subscription with individual loading
	async function handleManageSubscription() {
		// Check if user is authenticated before allowing management
		const isValidAuth = await validateAuthentication();
		if (!isValidAuth) {
			goto('/login');
			return;
		}

		loadingStates.manage = true;
		feedbackMessage = '';
		feedbackType = '';

		try {
			const response = await privateRequest<{ url: string }>('createCustomerPortal', {});

			// Redirect immediately to customer portal
			redirectToCustomerPortal(response.url);
		} catch (error) {
			console.error('Error opening customer portal:', error);
			feedbackMessage = 'Failed to open customer portal. Please try again.';
			feedbackType = 'error';
			loadingStates.manage = false;
		}
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
	onMount(async () => {
		if (browser) {
			document.title = 'Pricing & Plans - Peripheral';
			isLoaded = true;

			// Check for upgrade parameter from deep linking
			const urlParams = new URLSearchParams(window.location.search);
			const upgradePlan = urlParams.get('upgrade');

			// If upgrade parameter exists and user is authenticated, trigger checkout
			if (upgradePlan) {
				console.log('🎯 [onMount] Upgrade parameter found, validating authentication...');
				const isValidAuth = await validateAuthentication();
				console.log('🔍 [onMount] Auto-upgrade authentication result:', isValidAuth);

				if (isValidAuth) {
					console.log(
						'✅ [onMount] Auto-upgrade authentication confirmed, scheduling upgrade trigger'
					);
					// Small delay to ensure component is fully loaded
					setTimeout(() => {
						console.log('⏰ [onMount] Auto-upgrade timeout triggered, calling handleUpgrade');
						if (upgradePlan === 'plus' || upgradePlan === 'pro') {
							handleUpgrade(upgradePlan);
						} else {
							console.log('❌ [onMount] Invalid upgrade plan:', upgradePlan);
						}
					}, 500);
				} else {
					console.log(
						'❌ [onMount] Auto-upgrade authentication failed, user will need to authenticate manually'
					);
				}
			} else {
				console.log('ℹ️ [onMount] No upgrade parameter found, normal pricing page load');
			}
		} else {
			console.log('🖥️ [onMount] Not in browser environment (SSR)');
		}

		console.log('🔧 [onMount] Starting component initialization...');
		// Initialize component
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
			{:else if $subscriptionStatus.error && isAuthenticated()}
				<div class="error-message">{$subscriptionStatus.error}</div>
			{:else}
				<!-- Available Plans -->
				<div class="plans-section">
					<div class="plans-grid">
						<!-- Free Plan -->
						<div
							class="plan-card landing-glass-card {!$subscriptionStatus.isActive &&
							isAuthenticated()
								? 'current-plan'
								: ''}"
						>
							<div class="plan-header">
								{#if !$subscriptionStatus.isActive && isAuthenticated()}
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
							{#if !$subscriptionStatus.isActive && isAuthenticated()}
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
										{getPlan('plus').cta}
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
										{getPlan('pro').cta}
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

	@media (max-width: 640px) {
		.pricing-content {
			padding: 0 1rem;
		}

		.plan-card {
			padding: 1.5rem;
		}
	}
</style>
