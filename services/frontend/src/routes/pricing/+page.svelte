<script lang="ts">
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { redirectToCheckout, redirectToCustomerPortal } from '$lib/utils/helpers/stripe';
	import {
		subscriptionStatus,
		fetchSubscriptionStatus,
		fetchUserUsage
	} from '$lib/utils/stores/stores';
	import {
		fetchPricingConfiguration,
		getPlan,
		getCreditProduct,
		getStripePriceForPlan,
		getStripePriceForCreditProduct,
		formatPrice,
		getPlanFeaturesForPlan,
		type DatabasePlan,
		type DatabaseCreditProduct
	} from '$lib/config/pricing';
	import SiteHeader from '$lib/components/SiteHeader.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';
	import '$lib/styles/splash.css';

	// Individual loading states for better UX
	let loadingStates: Record<string, boolean> = {
		plus: false,
		pro: false,
		manage: false,
		cancel: false,
		reactivate: false,
		credits100: false,
		credits250: false,
		credits1000: false
	};

	// Success/error feedback
	let feedbackMessage = '';
	let feedbackType: 'success' | 'error' | '' = '';

	// Component loading state
	let isLoaded = false;

	// Pricing data state
	let plans: DatabasePlan[] = [];
	let creditProducts: DatabaseCreditProduct[] = [];
	let pricingLoading = true;
	let pricingError = '';

	// Selected billing period state – allows users to toggle between monthly and yearly pricing
	let billingPeriod: 'month' | 'year' = 'month';

	// Include Free plan regardless of selected billing period so it always shows
	$: filteredPlans = plans.filter(
		(plan) => plan.plan_name.toLowerCase() === 'free' || plan.billing_period === billingPeriod
	);

	// Function to determine if the current user is authenticated
	const isAuthenticated = (): boolean => {
		if (!browser) return false;
		const authToken = sessionStorage.getItem('authToken');
		return !!authToken;
	};

	// Helper function to safely check if a plan is the current plan (only when authenticated)
	// This implements a conservative approach: only highlight current plan when user is logged in
	// and subscription data is fully loaded without errors
	const isCurrentPlan = (planDisplayName: string): boolean => {
		return (
			isAuthenticated() &&
			$subscriptionStatus.currentPlan === planDisplayName &&
			!$subscriptionStatus.loading &&
			!$subscriptionStatus.error
		);
	};

	// Helper function to check if the current plan is being canceled
	const isCurrentPlanCanceling = (planDisplayName: string): boolean => {
		return (
			isAuthenticated() &&
			$subscriptionStatus.currentPlan === planDisplayName &&
			$subscriptionStatus.isCanceling &&
			!$subscriptionStatus.loading &&
			!$subscriptionStatus.error
		);
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

	// Load pricing configuration
	async function loadPricingConfiguration() {
		try {
			pricingLoading = true;
			pricingError = '';

			const config = await fetchPricingConfiguration();
			plans = config.plans
				.filter((plan) => plan.is_active)
				.sort((a, b) => a.sort_order - b.sort_order);
			console.log(config.creditProducts);

			// Initialize dynamic loading states for any new plans (e.g., yearly tiers)
			plans.forEach((plan) => {
				const key = plan.plan_name.toLowerCase();
				if (!(key in loadingStates)) {
					loadingStates[key] = false;
				}
			});

			creditProducts = config.creditProducts
				.filter((product) => product.is_active)
				.sort((a, b) => a.sort_order - b.sort_order);

			console.log('✅ [loadPricingConfiguration] Pricing configuration loaded:', {
				plans,
				creditProducts
			});
		} catch (error) {
			console.error('❌ [loadPricingConfiguration] Failed to load pricing configuration:', error);
			pricingError = 'Failed to load pricing information. Please refresh the page.';
		} finally {
			pricingLoading = false;
		}
	}

	// Initialize component
	async function initializeComponent() {
		const isAuth = isAuthenticated();

		// Load pricing configuration first
		await loadPricingConfiguration();

		if (isAuth) {
			await fetchSubscriptionStatus();
			console.log('📊 [initializeComponent] Subscription status fetch completed');
		} else {
			console.log(
				'ℹ️ [initializeComponent] User not authenticated, skipping subscription status fetch'
			);
		}
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
			feedbackMessage = 'Active subscription required to purchase credits';
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
		if (!(planKey in loadingStates)) {
			loadingStates[planKey] = false;
		}

		loadingStates[planKey] = true;
		feedbackMessage = '';
		feedbackType = '';

		try {
			const priceId = await getStripePriceForPlan(planKey);
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
			loadingStates[planKey] = false;
		}
	}

	// Process credit purchase
	async function processCreditPurchase(creditKey: string) {
		loadingStates[creditKey] = true;
		feedbackMessage = '';
		feedbackType = '';

		try {
			const creditProduct = await getCreditProduct(creditKey);
			if (!creditProduct) {
				throw new Error(`Credit product not found: ${creditKey}`);
			}

			const priceId = await getStripePriceForCreditProduct(creditKey);
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
			loadingStates[creditKey] = false;
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

	// Handle cancel subscription
	async function handleCancelSubscription() {
		if (
			!confirm(
				'Are you sure you want to cancel your subscription? You will retain access until the end of the current billing period.'
			)
		) {
			return;
		}

		loadingStates.cancel = true;
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
		} finally {
			loadingStates.cancel = false;
		}
	}

	// Handle reactivate subscription
	async function handleReactivateSubscription() {
		loadingStates.reactivate = true;
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
		} finally {
			loadingStates.reactivate = false;
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

	// Helper to get display name without "(Yearly)" suffix for yearly billing plans
	const getPlanDisplayName = (plan: DatabasePlan): string => {
		return plan.billing_period === 'year'
			? plan.display_name.replace(/\s*\(Yearly\)\s*$/i, '').trim()
			: plan.display_name;
	};

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
					feedbackMessage = 'Credits purchased successfully!';
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
<SiteHeader />
<!-- Use landing page design system -->
 <!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="landing-background landing-reset">

	<!-- Main Pricing Content -->
	<div class="landing-container" style="padding-top: 120px;">
		<div class="pricing-content landing-fade-in" class:loaded={isLoaded}>
			<!-- Hero Section -->
			<div class="pricing-hero">
				<h1 class="landing-title">Frictionless Trading</h1>
				<p class="landing-subtitle">Leverage Peripheral to envision, enhance, and execute your trading ideas.</p>
			</div>

			<!-- Feedback Messages -->
			{#if feedbackMessage}
				<div class="feedback-message {feedbackType}">
					{feedbackMessage}
				</div>
			{/if}

			{#if pricingLoading}
				<div class="loading-message">
					<div class="landing-loader"></div>
					<span></span>
				</div>
			{:else if pricingError}
				<div class="error-message">{pricingError}</div>
			{:else if $subscriptionStatus.loading}
				<div class="loading-message">
					<div class="landing-loader"></div>
					<span></span>
				</div>
			{:else if $subscriptionStatus.error && isAuthenticated()}
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

				<!-- Available Plans -->
				<div class="plans-section">
					<div class="plans-grid">
						{#each filteredPlans as plan}
							<div
								class="plan-card {isCurrentPlan(plan.display_name)
									? 'current-plan'
									: ''} {isCurrentPlanCanceling(plan.display_name)
									? 'canceling-plan'
									: ''} {plan.is_popular ? 'featured' : ''}"
							>
								<div class="plan-header">
									{#if isCurrentPlanCanceling(plan.display_name)}
										<div class="canceling-badge">Canceling</div>
									{:else if isCurrentPlan(plan.display_name)}
										<div class="current-badge">Current Plan</div>
									{:else if plan.is_popular}
										<div class="popular-badge">Most Popular</div>
									{/if}
									<h3>{getPlanDisplayName(plan)}</h3>
									<div class="plan-price">
										<span class="price">{formatPrice(plan.price_cents, plan.billing_period)}</span>
										<span class="period">/month</span>
									</div>
								</div>
								<ul class="plan-features">
									{#each getPlanFeaturesForPlan(plan) as feature}
										<li>{feature}</li>
									{/each}
								</ul>
								{#if isCurrentPlanCanceling(plan.display_name)}
									<button
										class="landing-button primary full-width"
										on:click={handleReactivateSubscription}
										disabled={loadingStates.reactivate}
									>
										{#if loadingStates.reactivate}
											<div class="landing-loader"></div>
										{:else}
											Reactivate Subscription
										{/if}
									</button>
									{#if $subscriptionStatus.currentPeriodEnd}
										<p class="canceling-note">
											Your subscription will remain active until {new Date(
												$subscriptionStatus.currentPeriodEnd * 1000
											).toLocaleDateString()}
										</p>
									{/if}
								{:else if isCurrentPlan(plan.display_name)}
									<button
										class="landing-button secondary full-width"
										on:click={handleCancelSubscription}
										disabled={loadingStates.cancel}
										style="background-color: #dc2626; color: white;"
									>
										{#if loadingStates.cancel}
											<div class="landing-loader"></div>
										{:else}
											Cancel Subscription
										{/if}
									</button>
								{:else if plan.plan_name.toLowerCase() === 'free'}
									{#if !$subscriptionStatus.isActive && isAuthenticated()}
										<button class="landing-button primary full-width current" disabled>
											Current Plan
										</button>
									{:else if $subscriptionStatus.isActive}
										<button class="landing-button secondary full-width" disabled>
											Downgrade not available
										</button>
									{:else}
										<button class="subscribe-button" disabled>
											Get Started Free
										</button>
									{/if}
								{:else}
									<button
										class="subscribe-button {plan.plan_name.toLowerCase() === 'pro' ? 'pro' : ''}"
										on:click={() => handleUpgrade(plan.plan_name.toLowerCase())}
										disabled={loadingStates[plan.plan_name.toLowerCase()]}
									>
										{#if loadingStates[plan.plan_name.toLowerCase()]}
											<div class="landing-loader"></div>
										{:else}
											Subscribe
										{/if}
									</button>
								{/if}
							</div>
						{/each}
					</div>
				</div>

				<!-- Credit Products Section -->
				<div class="credits-section">
					<div class="credits-header">
						<h2 class="landing-subtitle">Add More Credits</h2>
						<p class="credits-description">
							Purchase additional credits to extend your usage beyond your monthly allocation.
						</p>
					</div>
					<div class="credits-grid">
						{#each creditProducts as product}
							<div
								class="credit-card {!$subscriptionStatus.isActive
									? 'disabled'
									: ''} {product.is_popular ? 'featured' : ''}"
								title={!$subscriptionStatus.isActive
									? 'Active subscription required to purchase credits'
									: ''}
							>
								<div class="credit-header">
									{#if product.is_popular}
										<div class="popular-badge">Best Value</div>
									{/if}
									<h3>{product.display_name}</h3>
									<div class="credit-price">
										<span class="price">{formatPrice(product.price_cents, 'month')}</span>
									</div>
									<p class="credit-description">{product.description || ''}</p>
								</div>
								<button
									class="landing-button primary full-width"
									on:click={() => handleCreditPurchase(product.product_key)}
									disabled={loadingStates[product.product_key] || !$subscriptionStatus.isActive}
								>
									{#if loadingStates[product.product_key]}
										<div class="landing-loader"></div>
									{:else if !$subscriptionStatus.isActive}
										Subscription Required
									{:else}
										Purchase {product.credit_amount} Credits
									{/if}
								</button>
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</div>
</div>

<SiteFooter />

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


	.plan-card.featured {
		border-color: var(--landing-accent-blue);
	}

	/* Conservative current plan highlighting - subtle visual indicators */
	.plan-card.current-plan {
		border-color: var(--landing-success);
		background: rgba(34, 197, 94, 0.02);
		position: relative;
	}

	.plan-card.current-plan::before {
		content: '';
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 3px;
		background: var(--landing-success);
		border-radius: 8px 8px 0 0;
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

	/* Credit Products Styles */
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
	.credit-card.disabled .price,
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

	.credit-price {
		display: flex;
		align-items: baseline;
		justify-content: center;
		gap: 0.25rem;
		margin-bottom: 1rem;
	}

	.credit-price .price {
		font-size: 2.5rem;
		font-weight: 700;
		color: var(--landing-text-primary);
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
		border-radius: 25px;
		padding: 4px;
		margin: 0 auto 2rem;
		width: fit-content;
		overflow: hidden;
	}

	.slider-background {
		position: absolute;
		top: 4px;
		left: 4px;
		width: calc(50% - 4px);
		height: calc(100% - 8px);
		background: #f5f9ff;
		border-radius: 21px;
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
		border: 2px solid #f5f9ff;
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


	@media (max-width: 640px) {
		.slider-option {
			padding: 0.625rem 1rem;
			font-size: 0.8125rem;
			min-width: 90px;
		}
	}
</style>
