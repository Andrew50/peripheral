import { loadStripe } from '@stripe/stripe-js';
import { browser } from '$app/environment';

// Load Stripe.js with publishable key from environment
export async function getStripe() {
	if (!browser) return null;

	const stripeKey = import.meta.env.VITE_PUBLIC_STRIPE_KEY || process.env.PUBLIC_STRIPE_KEY;
	if (!stripeKey) {
		console.warn('Stripe publishable key not found in environment variables');
		return null;
	}

	return await loadStripe(stripeKey);
}

// Redirect to Stripe Checkout
export async function redirectToCheckout(sessionId: string): Promise<void> {
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
export function redirectToCustomerPortal(portalUrl: string): void {
	if (browser) {
		window.location.href = portalUrl;
	}
}
