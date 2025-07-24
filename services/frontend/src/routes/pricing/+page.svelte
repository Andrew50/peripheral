<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import {
		subscriptionStatus,
		fetchSubscriptionStatus,
		fetchUserUsage
	} from '$lib/utils/stores/stores';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import '$lib/styles/splash.css';
	import { getAuthState } from '$lib/auth';

	// ===== SERVER DATA =====
	export let data: {
		plans: any[];
		creditProducts: any[];
		environment: string;
		pricingError?: string;
	};

	// ===== TYPE DEFINITIONS =====
	interface DatabasePlan {
		id: number;
		product_key: string;
		queries_limit: number;
		alerts_limit: number;
		strategy_alerts_limit: number;
		realtime_charts: boolean;
		sub_minute_charts: boolean;
		multi_chart: boolean;
		multi_strategy_screening: boolean;
		watchlist_alerts: boolean;
		prices: Array<{
			id: number;
			price_cents: number;
			stripe_price_id_live: string | null;
			stripe_price_id_test: string | null;
			billing_period: string;
		}> | null;
		created_at: string;
		updated_at: string;
	}

	interface DatabaseCreditProduct {
		id: number;
		product_key: string;
		credit_amount: number;
		price_cents: number;
		stripe_price_id_test: string | null;
		stripe_price_id_live: string | null;
		created_at: string;
		updated_at: string;
	}

	// ===== INLINED HELPER FUNCTIONS =====

	// Load Stripe.js with publishable key
	async function getStripe() {
		if (!browser) return null;
		const stripeKey = (import.meta as any).env.VITE_PUBLIC_STRIPE_KEY;
		if (!stripeKey) {
			console.warn('Stripe publishable key not found in environment variables');
			return null;
		}
		try {
			const stripeModule = await import('@stripe/stripe-js');
			return await stripeModule.loadStripe(stripeKey);
		} catch (error) {
			console.error('Failed to load Stripe:', error);
			return null;
		}
	}

	// Redirect to Stripe Checkout
	async function redirectToCheckout(sessionId: string): Promise<void> {
		const stripe = await getStripe();
		if (!stripe) {
			throw new Error('Stripe failed to load');
		}
		const result = await stripe.redirectToCheckout({ sessionId });
		if (result.error) {
			throw new Error(result.error.message);
		}
	}

	// Open Stripe Customer Portal
	function redirectToCustomerPortal(portalUrl: string): void {
		if (browser) {
			window.location.href = portalUrl;
		}
	}

	// Format price in cents to display price
	function formatPrice(priceCents: number, billingPeriod: string): string {
		if (priceCents === 0) return '$0';
		if (billingPeriod === 'year') {
			return `$${(priceCents / 100 / 12).toFixed(2).replace(/\.00$/, '')}`;
		}
		return `$${(priceCents / 100).toFixed(2).replace(/\.00$/, '')}`;
	}

	// Get the appropriate price for a plan based on billing period
	function getPlanPrice(plan: DatabasePlan, billingPeriod: string): number {
		if (plan.product_key.toLowerCase() === 'free') {
			return 0;
		}
		if (!plan.prices || plan.prices.length === 0) {
			return 0;
		}
		const apiPeriod = billingPeriod === 'month' ? 'monthly' : 'yearly';
		const priceInfo = plan.prices.find((p) => p.billing_period === apiPeriod);
		if (priceInfo) {
			return priceInfo.price_cents;
		}
		return plan.prices[0].price_cents;
	}

	// Get the appropriate price ID based on environment and billing period
	function getPriceId(
		plan: DatabasePlan,
		environment: string,
		billingPeriod: string = 'month'
	): string | null {
		if (plan.product_key.toLowerCase() === 'free') {
			return null;
		}
		if (!plan.prices || plan.prices.length === 0) {
			return null;
		}
		const apiPeriod = billingPeriod === 'month' ? 'monthly' : 'yearly';
		const priceInfo = plan.prices.find((p) => p.billing_period === apiPeriod);
		if (priceInfo) {
			return environment === 'test'
				? priceInfo.stripe_price_id_test
				: priceInfo.stripe_price_id_live;
		}
		return null;
	}

	// Get the appropriate credit product price ID based on environment
	function getCreditPriceId(
		creditProduct: DatabaseCreditProduct,
		environment: string
	): string | null {
		return environment === 'test'
			? creditProduct.stripe_price_id_test
			: creditProduct.stripe_price_id_live;
	}

	// Get plan display name
	function getPlanDisplayName(plan: DatabasePlan): string {
		const displayNames: Record<string, string> = {
			free: 'Free',
			plus: 'Plus',
			pro: 'Pro',
			enterprise: 'Enterprise'
		};
		return displayNames[plan.product_key.toLowerCase()] || plan.product_key;
	}

	// Get credit product display name
	function getCreditProductDisplayName(creditProduct: DatabaseCreditProduct): string {
		const displayNames: Record<string, string> = {
			credits100: '100 Queries',
			credits250: '250 Queries',
			credits1000: '1000 Queries'
		};
		return displayNames[creditProduct.product_key] || `${creditProduct.credit_amount} Queries`;
	}

	// Get plan features dynamically from plan data
	function getPlanFeatures(plan: DatabasePlan): string[] {
		const features: string[] = [];

		// 1. Queries limit
		if (plan.queries_limit > 0) {
			features.push(`${plan.queries_limit} queries/mo`);
		}

		// 2. Data quality - based on realtime_charts
		if (plan.realtime_charts) {
			features.push('Realtime data');
		} else {
			features.push('Delayed data');
		}

		// 3. Strategy screening - simplified to just "Realtime strategy screening"
		if (plan.strategy_alerts_limit > 0) {
			features.push('Realtime strategy screening');
		}

		// 4. Number of active strategies
		if (plan.strategy_alerts_limit > 0) {
			features.push(`${plan.strategy_alerts_limit} active strategies`);
		}

		// 5. Number of active alerts
		if (plan.alerts_limit > 0) {
			features.push(`${plan.alerts_limit} news or price alerts`);
		}

		// 6. Watchlist alerts
		if (plan.watchlist_alerts) {
			features.push('Watchlist alerts');
		}

		// Sub-minute charts (keeping this as it's not multi chart)
		if (plan.sub_minute_charts) {
			features.push('Sub-minute charts');
		}

		return features;
	}

	// ===== COMPONENT STATE =====

	// Single loading state instead of object
	let busyKey: string | null = null;

	// Success/error feedback
	let feedbackMessage = '';
	let feedbackType: 'success' | 'error' | '' = '';

	// Component loading state
	let isLoaded = false;

	// Pricing data from server
	$: plans = data.plans as DatabasePlan[];
	$: creditProducts = data.creditProducts as DatabaseCreditProduct[];
	$: environment = data.environment;
	$: pricingError = data.pricingError || '';

	// Selected billing period state
	let billingPeriod: 'month' | 'year' = 'year';

	// Create filtered plans that include Free plan and match billing period
	$: filteredPlans = plans.filter((plan) => {
		// Always include Free plan
		if (plan.product_key?.toLowerCase() === 'free') {
			return true;
		}
		// For other plans, check if they have prices for the selected billing period
		if (!plan.prices || plan.prices.length === 0) {
			return false;
		}
		const apiPeriod = billingPeriod === 'month' ? 'monthly' : 'yearly';
		return plan.prices.some((p) => p.billing_period === apiPeriod);
	});

	// Auth state - check immediately to prevent flash
	let isAuthenticated = getAuthState();

	// Function to determine if the current user is authenticated
	const isAuthenticatedFn = (): boolean => {
		return isAuthenticated;
	};

	// Helper function to safely check if a plan is the current plan
	const isCurrentPlan = (plan: DatabasePlan): boolean => {
		const planDisplayName = getPlanDisplayName(plan);
		return (
			isAuthenticatedFn() &&
			$subscriptionStatus.currentPlan?.split(' ')[0] === planDisplayName?.split(' ')[0] &&
			!$subscriptionStatus.loading &&
			!$subscriptionStatus.error
		);
	};

	// Helper function to check if the current plan is being canceled
	const isCurrentPlanCanceling = (plan: DatabasePlan): boolean => {
		const planDisplayName = getPlanDisplayName(plan);
		return (
			isAuthenticatedFn() &&
			$subscriptionStatus.currentPlan === planDisplayName &&
			$subscriptionStatus.isCanceling &&
			!$subscriptionStatus.loading &&
			!$subscriptionStatus.error
		);
	};

	// Helper to check if a Pro plan should show "Upgrade" instead of "Subscribe"
	const isUpgradeEligible = (plan: DatabasePlan): boolean => {
		return (
			isAuthenticatedFn() &&
			$subscriptionStatus.currentPlan?.toLowerCase().includes('plus') &&
			plan.product_key.toLowerCase().includes('pro')
		);
	};

	// Helper function to check if a plan is popular (using backend data)
	const isPlanPopular = (plan: DatabasePlan): boolean => {
		// Move "Most Popular" to Pro plan instead of Plus
		return plan.product_key?.toLowerCase() === 'pro';
	};

	// Helper function to check if a credit product is popular (using backend data)
	const isCreditProductPopular = (creditProduct: DatabaseCreditProduct): boolean => {
		// For now, hardcode credits1000 as popular until backend provides this data
		return creditProduct.product_key === 'credits1000';
	};

	// Helper function to validate authentication
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
		// Always fetch subscription status regardless of authentication
		await fetchSubscriptionStatus();
		console.log('📊 [initializeComponent] Subscription status fetch completed');
	}

	// Enhanced upgrade handler with individual loading states (supports dynamic plan keys)
	async function handleUpgrade(planKey: string) {
		// Check if user is authenticated before allowing upgrade
		const isValidAuth = await validateAuthentication();

		if (!isValidAuth) {
			// Redirect to signup with plan information for deep linking
			goto(`/signup?plan=${planKey}&redirect=checkout`);
			return;
		}

		await processUpgrade(planKey);
	}

	// Enhanced credit purchase handler
	async function handleCreditPurchase(creditKey: string) {
		// Check if user is authenticated before allowing purchase
		const isValidAuth = await validateAuthentication();

		if (!isValidAuth) {
			goto('/login');
			return;
		}

		// Check if user has active subscription
		if (!$subscriptionStatus.isActive) {
			feedbackMessage = 'Active subscription required to purchase queries';
			feedbackType = 'error';
			return;
		}

		await processCreditPurchase(creditKey);
	}

	// Process the actual upgrade (supports dynamic plan keys)
	async function processUpgrade(planKey: string) {
		// Double-check authentication before processing payment
		const isValidAuth = await validateAuthentication();

		if (!isValidAuth) {
			console.log(
				'❌ [processUpgrade] Authentication failed during payment, redirecting to signup'
			);
			goto(`/signup?plan=${planKey}&redirect=checkout`);
			return;
		}

		// Ensure we have a loading state entry for this plan
		if (busyKey === planKey) {
			return; // Prevent double clicks
		}

		busyKey = planKey;
		feedbackMessage = '';
		feedbackType = '';

		try {
			// Find the plan by key
			const plan = plans.find((p) => p.product_key?.toLowerCase() === planKey.toLowerCase());
			if (!plan) {
				throw new Error(`Plan not found: ${planKey}`);
			}

			const priceId = getPriceId(plan, environment, billingPeriod);
			if (!priceId) {
				throw new Error(`No Stripe price ID found for plan: ${planKey}`);
			}

			const response = await privateRequest<{ sessionId: string; url: string }>(
				'createCheckoutSession',
				{ priceId }
			);

			// Redirect immediately to checkout
			try {
				await redirectToCheckout(response.sessionId);
			} catch (redirectError) {
				console.warn(
					'Stripe.js redirect failed, falling back to direct URL redirect',
					redirectError
				);
				window.location.href = response.url;
			}
		} catch (error) {
			console.error('❌ [processUpgrade] Error creating checkout session:', error);
			feedbackMessage = 'Failed to start checkout. Please try again.';
			feedbackType = 'error';
			busyKey = null; // Clear busy state on error
		}
	}

	// Process credit purchase
	async function processCreditPurchase(creditKey: string) {
		if (busyKey === creditKey) {
			return; // Prevent double clicks
		}

		busyKey = creditKey;
		feedbackMessage = '';
		feedbackType = '';

		try {
			const creditProduct = creditProducts.find((p) => p.product_key === creditKey);
			if (!creditProduct) {
				throw new Error(`Credit product not found: ${creditKey}`);
			}

			const priceId = getCreditPriceId(creditProduct, environment);
			if (!priceId) {
				throw new Error(`No Stripe price ID found for credit product: ${creditKey}`);
			}

			const response = await privateRequest<{ sessionId: string; url: string }>(
				'createCreditCheckoutSession',
				{
					priceId: priceId,
					creditAmount: creditProduct.credit_amount
				}
			);

			// Redirect immediately to checkout
			try {
				await redirectToCheckout(response.sessionId);
			} catch (redirectError) {
				console.warn(
					'Stripe.js redirect failed, falling back to direct URL redirect',
					redirectError
				);
				window.location.href = response.url;
			}
		} catch (error) {
			console.error('❌ [processCreditPurchase] Error creating credit checkout session:', error);
			feedbackMessage = 'Failed to start checkout. Please try again.';
			feedbackType = 'error';
			busyKey = null; // Clear busy state on error
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

		busyKey = 'manage';
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
			busyKey = null; // Clear busy state on error
		}
	}

	// Handle cancel subscription
	async function handleCancelSubscription() {
		if (
			!confirm(
				'Are you sure you want to cancel your subscription? You will retain access until the end of the current billing period.'
			)
		) {
			return;
		}

		busyKey = 'cancel';
		feedbackMessage = '';
		feedbackType = '';

		try {
			await privateRequest('cancelSubscription', {});
			await fetchSubscriptionStatus();
			feedbackMessage =
				'Your subscription will remain active until the end of your current billing period.';
			feedbackType = 'success';
		} catch (error) {
			console.error('Error cancelling subscription:', error);
			feedbackMessage = 'Failed to cancel subscription. Please try again.';
			feedbackType = 'error';
			busyKey = null; // Clear busy state on error
		}
	}

	// Handle reactivate subscription
	async function handleReactivateSubscription() {
		busyKey = 'reactivate';
		feedbackMessage = '';
		feedbackType = '';

		try {
			await privateRequest('reactivateSubscription', {});
			await fetchSubscriptionStatus();
			feedbackMessage = 'Your subscription has been reactivated successfully.';
			feedbackType = 'success';
		} catch (error) {
			console.error('Error reactivating subscription:', error);
			feedbackMessage = 'Failed to reactivate subscription. Please try again.';
			feedbackType = 'error';
			busyKey = null; // Clear busy state on error
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

	// Run initialization on mount
	onMount(async () => {
		if (browser) {
			document.title = 'Pricing | Peripheral';
			isLoaded = true;

			// Async initialization function
			async function init() {
				// Check for Stripe checkout success session_id parameter
				const urlParams = new URLSearchParams(window.location.search);
				const sessionId = urlParams.get('session_id');
				const creditsPurchased = urlParams.get('credits_purchased');

				if (sessionId) {
					console.log(
						'🎯 [pricing onMount] Stripe checkout success detected, session_id:',
						sessionId
					);

					// Clear the session_id from URL for cleaner UX
					urlParams.delete('session_id');
					const newUrl = `${window.location.pathname}${urlParams.toString() ? '?' + urlParams.toString() : ''}`;
					window.history.replaceState({}, '', newUrl);

					// Defer verification until after page is fully loaded
					// This ensures verification happens AFTER the redirect to the pricing page
					setTimeout(async () => {
						console.log(
							'⏰ [pricing onMount] Deferred verification starting for session:',
							sessionId
						);
						await verifyAndUpdateSubscriptionStatus(sessionId);
					}, 100); // Small delay to ensure page is fully rendered
					return; // Exit early since we handled checkout verification
				}

				if (creditsPurchased) {
					console.log(
						'🎯 [pricing onMount] Credit purchase success detected, session_id:',
						creditsPurchased
					);

					// Clear the credits_purchased from URL for cleaner UX
					urlParams.delete('credits_purchased');
					const newUrl = `${window.location.pathname}${urlParams.toString() ? '?' + urlParams.toString() : ''}`;
					window.history.replaceState({}, '', newUrl);

					// Refresh user usage and show success message
					await fetchUserUsage();
					feedbackMessage = 'Queries purchased successfully!';
					feedbackType = 'success';
					return;
				}

				// Check for upgrade parameter from deep linking
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

				// Initialize component for normal flow
				initializeComponent();
			}

			// Start async initialization
			init();
		} else {
			console.log('🖥️ [onMount] Not in browser environment (SSR)');
		}

		console.log('🔧 [onMount] Component mount completed');
	});

	// Stripe-recommended pattern: verify checkout session and update subscription status
	async function verifyAndUpdateSubscriptionStatus(sessionId: string) {
		console.log(
			'🔍 [pricing verifyAndUpdateSubscriptionStatus] Starting verification for session:',
			sessionId
		);

		try {
			// Verify the checkout session directly with Stripe via our backend
			const verificationResult = await privateRequest<{
				status: string;
				isActive: boolean;
				currentPlan: string;
				hasCustomer: boolean;
				hasSubscription: boolean;
				currentPeriodEnd: number | null;
				subscriptionCreditsRemaining: number;
				purchasedCreditsRemaining: number;
				totalCreditsRemaining: number;
				subscriptionCreditsAllocated: number;
			}>('verifyCheckoutSession', { sessionId });
			console.log(
				'✅ [pricing verifyAndUpdateSubscriptionStatus] Verification result:',
				verificationResult
			);

			// Refresh subscription status to ensure UI is up to date
			await fetchSubscriptionStatus();

			console.log(
				'🎉 [pricing verifyAndUpdateSubscriptionStatus] Subscription verification completed'
			);
		} catch (error) {
			console.error(
				'❌ [pricing verifyAndUpdateSubscriptionStatus] Error verifying checkout session:',
				error
			);
			// Fallback to simple refresh
			console.log(
				'🔄 [pricing verifyAndUpdateSubscriptionStatus] Falling back to simple subscription refresh'
			);
			await fetchSubscriptionStatus();
		}
	}
</script>

<svelte:head>
	<title>Pricing | Peripheral</title>
</svelte:head>
<SiteHeader {isAuthenticated} />
<!-- Use landing page design system -->
<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="page-wrapper">
	<!-- Main Pricing Content -->
	<div class="landing-container">
		<div class="pricing-content landing-fade-in" class:loaded={isLoaded}>
			<!-- Hero Section -->
			<div class="pricing-hero">
				<h1 class="landing-title">Frictionless Trading</h1>
				<p class="landing-subtitle">
					Leverage Peripheral to envision, enhance, and execute your trading ideas.
				</p>
			</div>

			<!-- Feedback Messages -->
			{#if feedbackMessage}
				<div class="feedback-message {feedbackType}">
					{feedbackMessage}
				</div>
			{/if}

			{#if pricingError}
				<div class="error-message">{pricingError}</div>
			{:else if $subscriptionStatus.loading}
				<div class="loading-message">
					<div class="landing-loader"></div>
					<span></span>
				</div>
			{:else if $subscriptionStatus.error && isAuthenticatedFn()}
				<div class="error-message">{$subscriptionStatus.error}</div>
			{:else}
				<!-- Billing Period Slider Toggle -->
				<div class="billing-slider" class:yearly={billingPeriod === 'year'}>
					<div class="slider-background"></div>
					<button
						class="slider-option {billingPeriod === 'month' ? 'active' : ''}"
						on:click={() => (billingPeriod = 'month')}
					>
						Billed Monthly
					</button>
					<button
						class="slider-option {billingPeriod === 'year' ? 'active' : ''}"
						on:click={() => (billingPeriod = 'year')}
					>
						Billed Yearly
					</button>
				</div>
				<div class="billing-subtitle">
					<p>Save up to 20% by paying yearly - 2 months free</p>
				</div>

				<!-- Available Plans -->
				<div class="plans-section">
					<div class="plans-grid">
						{#each filteredPlans as plan}
							<div
								class="plan-card {isCurrentPlan(plan)
									? 'current-plan'
									: ''} {isCurrentPlanCanceling(plan) ? 'canceling-plan' : ''} {isPlanPopular(plan)
									? 'featured'
									: ''} {plan.product_key?.toLowerCase() === 'free' ? 'free-plan' : ''}"
							>
								<div class="plan-header">
									{#if isCurrentPlanCanceling(plan)}
										<div class="canceling-badge">Canceling</div>
									{:else if isPlanPopular(plan)}
										<div class="popular-badge">Most Popular</div>
									{/if}
									<h3>{getPlanDisplayName(plan)}</h3>
									<div class="plan-price">
										<span class="price"
											>{formatPrice(getPlanPrice(plan, billingPeriod), billingPeriod)}</span
										>
										<span class="period">/month</span>
									</div>
								</div>
								<ul class="plan-features">
									{#each getPlanFeatures(plan) as feature}
										<li>{feature}</li>
									{/each}
								</ul>
								{#if isCurrentPlanCanceling(plan)}
									<button
										class="landing-button primary full-width"
										on:click={handleReactivateSubscription}
										disabled={busyKey === 'reactivate'}
									>
										{#if busyKey === 'reactivate'}
											<div class="landing-loader"></div>
										{/if}
										Reactivate Subscription
									</button>
									{#if $subscriptionStatus.currentPeriodEnd}
										<p class="canceling-note">
											Your subscription will remain active until {new Date(
												$subscriptionStatus.currentPeriodEnd * 1000
											).toLocaleDateString()}
										</p>
									{/if}
								{:else if isCurrentPlan(plan)}
									<button class="subscribe-button active-subscription" on:click={handleCancelSubscription} disabled>
										{#if busyKey === 'cancel'}
											<div class="landing-loader"></div>
										{/if}
										Active Subscription
									</button>
								{:else if plan.product_key?.toLowerCase() === 'free'}
									{#if !$subscriptionStatus.isActive && isAuthenticatedFn()}
										<button class="subscribe-button current" disabled> Current Plan </button>
									{:else if $subscriptionStatus.isActive}
										<button class="subscribe-button" disabled> Downgrade not available </button>
									{:else}
										<button class="subscribe-button" disabled> Get Started Free </button>
									{/if}
								{:else}
									<button
										class="subscribe-button {plan.product_key?.toLowerCase().includes('pro')
											? 'pro'
											: ''}"
										on:click={() => handleUpgrade(plan.product_key?.toLowerCase() || '')}
										disabled={busyKey === (plan.product_key?.toLowerCase() || '')}
									>
										{#if busyKey === (plan.product_key?.toLowerCase() || '')}
											<div class="landing-loader"></div>
										{/if}
										{#if isUpgradeEligible(plan)}
											Upgrade
										{:else}
											Subscribe
										{/if}
									</button>
								{/if}
							</div>
						{/each}
					</div>
				</div>

				<!-- Query Products Section -->
				<div class="credits-section">
					<div class="credits-header">
						<h2 class="landing-subtitle">Add More Queries</h2>
						<p class="credits-description">
							Purchase additional queries to extend your usage beyond your monthly allocation.
						</p>
					</div>
					<div class="credits-grid">
						{#each creditProducts as product}
							<div
								class="credit-card {!$subscriptionStatus.isActive
									? 'disabled'
									: ''} {isCreditProductPopular(product) ? 'featured' : ''}"
								title={!$subscriptionStatus.isActive
									? 'Active subscription required to purchase queries'
									: ''}
							>
								<div class="credit-header">
									{#if isCreditProductPopular(product)}
										<div class="popular-badge">Best Value</div>
									{/if}
									<h3>{getCreditProductDisplayName(product)}</h3>
									<div class="credit-amount">
										<span class="amount">{product.credit_amount}</span>
										<span class="label">Queries</span>
									</div>
									<p class="credit-description">Additional queries for your account</p>
								</div>
								<button
									class="landing-button primary full-width"
									on:click={() => handleCreditPurchase(product.product_key)}
									disabled={busyKey === product.product_key}
								>
									{#if busyKey === product.product_key}
										<div class="landing-loader"></div>
									{/if}
									{#if !$subscriptionStatus.isActive}
										Subscription Required
									{:else}
										Purchase {product.credit_amount} Queries
									{/if}
								</button>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</div>
	<SiteFooter />
</div>


<style>
	/* Critical global styles - applied immediately to prevent layout shift */
	:global(*) {
		box-sizing: border-box;
	}

	:global(html) {
		-ms-overflow-style: none; /* IE and Edge */
	}

	:global(body) {
		-ms-overflow-style: none; /* IE and Edge */
		background: transparent !important; /* Override any global backgrounds */
	}

	:global(html) {
		background: transparent !important; /* Override any global backgrounds */
	}

	/* Override width restrictions from global landing styles */
	:global(.landing-container) {
		max-width: none !important;
		width: 100% !important;

	}

	/* Apply the same gradient background as landing page */
	.page-wrapper {
		width: 100%;
		min-height: 100vh;
		background: linear-gradient(
			180deg,
			#010022 0%,
			#02175F 100%

		);
	}

	/* Landing container should have transparent background like landing page */
	.landing-container {
		position: relative;
		width: 100%;
		background: transparent;
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
		min-height: 100vh;
	}

	/* Pricing-specific styles that build on landing system */
	.pricing-content {
		max-width: 1200px;
		margin: 0 auto;
		padding: 0 2rem;
		background: transparent;
	}

	.pricing-hero {
		text-align: center;
		margin-bottom: 3rem;
		padding-top: 1rem; /* Space for header */
	}

	.landing-subtitle {
		color: #f5f9ff;
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
		display: flex;
		flex-direction: column;
		border: 2px solid rgba(255, 255, 255, 0.8);
		border-radius: 24px;
	}

	.plan-card.featured {
		border-color: 2px solid #f5f9ff;
	}

	/* Conservative current plan highlighting - subtle visual indicators */
	.plan-card.current-plan {
		border-color: #f5f9ff;
		background: rgba(34, 197, 94, 0.02);
		position: relative;
	}

	/* Free plan styling - visible border */
	.plan-card.free-plan {
		border: 2px solid rgba(255, 255, 255, 0.6);
	}

	/* Canceling plan styling */
	.plan-card.canceling-plan {
		border-color: var(--landing-warning, #f59e0b);
		background: rgba(245, 158, 11, 0.02);
		position: relative;
	}

	.plan-card.canceling-plan::before {
		content: '';
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 3px;
		background: var(--landing-warning, #f59e0b);
		border-radius: 8px 8px 0 0;
	}
	.popular-badge {
		position: absolute;
		top: -14px;
		left: 50%;
		transform: translateX(-50%);
		background: #f5f9ff;
		color: #000000;
		font-family: 'Geist', 'Inter', sans-serif;
		font-size: 0.875rem;
		font-weight: 600;
		padding: 0.3rem 0.875rem;
		border-radius: 14px;
	}

	/* Subtle current plan badge - smaller and less prominent than popular badge */
	.current-badge {
		position: absolute;
		top: -8px;
		left: 50%;
		transform: translateX(-50%);
		background: var(--landing-success);
		color: white;
		font-size: 0.6875rem;
		font-weight: 500;
		padding: 0.1875rem 0.625rem;
		border-radius: 10px;
		opacity: 0.9;
	}

	/* Canceling badge */
	.canceling-badge {
		position: absolute;
		top: -8px;
		left: 50%;
		transform: translateX(-50%);
		background: var(--landing-warning, #f59e0b);
		color: white;
		font-size: 0.6875rem;
		font-weight: 500;
		padding: 0.1875rem 0.625rem;
		border-radius: 10px;
		opacity: 0.9;
	}

	.canceling-note {
		font-size: 0.8125rem;
		color: var(--landing-warning, #f59e0b);
		text-align: center;
		margin-top: 0.75rem;
		font-style: italic;
	}

	.plan-header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.plan-header h3 {
		font-size: 1.5rem;
		font-weight: 600;
		color: #f5f9ff;
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
		color: #f5f9ff;
	}

	.period {
		font-size: 1rem;
		color: #f5f9ff;
	}

	.plan-features {
		list-style: none;
		padding: 0;
		margin-bottom: 2rem;
		flex-grow: 1;
	}

	.plan-features li {
		padding: 0.5rem 0;
		color: #f5f9ff;
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

	/* Query Products Styles */
	.credits-section {
		margin-bottom: 3rem;
	}

	.credits-header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.credits-description {
		color: var(--landing-text-secondary);
		font-size: 1.1rem;
		max-width: 600px;
		margin: 0 auto;
		line-height: 1.6;
	}

	.credits-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 2rem;
		margin-top: 2rem;
	}

	@media (max-width: 1024px) {
		.credits-grid {
			grid-template-columns: 1fr;
			max-width: 450px;
			margin: 2rem auto 0;
		}
	}

	.credit-card {
		padding: 2rem;
		position: relative;
		transition: all 0.3s ease;
		display: flex;
		flex-direction: column;
		min-height: 250px;
	}

	.credit-card:hover {
		transform: translateY(-5px);
		border-color: var(--landing-border-focus);
	}

	.credit-card.featured {
		border-color: var(--landing-accent-blue);
	}

	.credit-card.disabled {
		opacity: 0.6;
		background: rgba(255, 255, 255, 0.02);
		border-color: rgba(255, 255, 255, 0.05);
		cursor: not-allowed;
	}

	.credit-card.disabled:hover {
		transform: none;
		border-color: rgba(255, 255, 255, 0.05);
	}

	.credit-card.disabled h3,
	.credit-card.disabled .amount,
	.credit-card.disabled .label,
	.credit-card.disabled .credit-description {
		color: var(--landing-text-secondary);
		opacity: 0.7;
	}

	.credit-card.disabled .popular-badge {
		opacity: 0.5;
	}

	.credit-header {
		text-align: center;
		margin-bottom: 2rem;
		flex-grow: 1;
	}

	.credit-header h3 {
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--landing-text-primary);
		margin-bottom: 1rem;
	}

	.credit-amount {
		display: flex;
		align-items: baseline;
		justify-content: center;
		gap: 0.25rem;
		margin-bottom: 1rem;
	}

	.credit-amount .amount {
		font-size: 2.5rem;
		font-weight: 700;
		color: var(--landing-text-primary);
	}

	.credit-amount .label {
		font-size: 1rem;
		color: var(--landing-text-secondary);
	}

	.credit-description {
		color: var(--landing-text-secondary);
		font-size: 0.9375rem;
		line-height: 1.5;
	}

	.logo-button {
		background: none;
		border: none;
		padding: 0;
		cursor: pointer;
		display: flex;
		align-items: center;
		transition: opacity 0.2s ease;
	}

	.logo-button:hover {
		opacity: 0.8;
	}

	.logo-button:focus {
		outline: 2px solid var(--landing-accent-blue);
		outline-offset: 2px;
		border-radius: 4px;
	}

	/* Billing Slider Styles */
	.billing-slider {
		position: relative;
		display: flex;
		background: var(--color-dark);
		border-radius: 28px;
		padding: 4px;
		/* Increased bottom margin for extra space before plan cards */
		margin: 0 auto 0.5rem; /* keep slight separation; subtitle will add its own spacing */
		width: fit-content;
		overflow: hidden;
		/* Shrink slider by 10% */
		transform: scale(0.9);
		transform-origin: center;
	}

	/* Billing subtitle directly under the slider */
	.billing-subtitle {
		text-align: center;
		margin: 1rem auto 3rem;
		max-width: 600px;
	}

	.billing-subtitle p {
		font-size: 0.9375rem;
		font-weight: 500;
		color: #f5f9ff;
		font-family: 'Geist', 'Inter', sans-serif;
		line-height: 1.5;
	}

	/* Enlarge the hero title specifically for the pricing page */
	.pricing-hero .landing-title {
		margin: 6rem 0 0 0;
		font-size: clamp(3rem, 7vw, 4.5rem);
		font-family: 'Inter', sans-serif;
		color: #f5f9ff;
	}

	.slider-background {
		position: absolute;
		top: 4px;
		left: 4px;
		width: calc(50% - 4px);
		height: calc(100% - 8px);
		background: #f5f9ff;
		border-radius: 28px;
		transition: transform 0.2s ease;
		z-index: 1;
		box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
	}

	.billing-slider.yearly .slider-background {
		transform: translateX(100%);
	}

	.slider-option {
		position: relative;
		z-index: 2;
		background: none;
		border: none;
		padding: 0.875rem 2rem;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
		border-radius: 16px;
		min-width: 120px;
		font-family: 'Inter', sans-serif;
		color: #f5f9ff;
		display: flex;
		align-items: center;
		justify-content: center;
		text-align: center;
		white-space: nowrap;
	}
	.slider-option.active {
		color: #000000;
	}

	/* Subscribe Button Styles – visually consistent with slider options */
	.subscribe-button {
		position: relative;
		z-index: 1;
		background: none;
		border: 2px solid rgba(255, 255, 255, 0.3);
		padding: 0.875rem 2rem;
		font-size: 1rem;
		font-weight: 600;
		cursor: pointer;
		border-radius: 24px;
		min-width: 120px;
		font-family: 'Geist', 'Inter', sans-serif;
		color: #f5f9ff;
		display: flex;
		align-items: center;
		justify-content: center;
		text-align: center;
		white-space: nowrap;
		gap: 0.5rem;
		transition: all 0.2s ease;
	}

	.subscribe-button:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.1);
		border-color: rgba(255, 255, 255, 0.4);
		transform: translateY(-1px);
	}

	.subscribe-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Pro plan subscribe button overrides */
	.subscribe-button.pro {
		background: #f5f9ff;
		color: #000000;
		border: 2px solid transparent;
	}

	.subscribe-button.pro:hover:not(:disabled) {
		background: #e0e0e0;
	}

	.subscribe-button.pro:active:not(:disabled) {
		background: #e0e0e0;
	}

	/* Greyed-out styling for disabled/current subscribe buttons (Free plan, downgrade) */
	.subscribe-button.current {
		background: rgba(255, 255, 255, 0.1);
		color: rgba(255, 255, 255, 0.7);
		border-color: rgba(255, 255, 255, 0.15);
		cursor: not-allowed;
	}

	/* Active subscription button - green text */
	.subscribe-button.active-subscription {
		color: var(--landing-success);
	}

	@media (max-width: 640px) {
		.slider-option {
			padding: 0.625rem 1rem;
			font-size: 0.8125rem;
			min-width: 90px;
		}
	}
</style>
