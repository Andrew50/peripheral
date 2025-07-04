import { loadStripe } from '@stripe/stripe-js';
import { browser } from '$app/environment';

// Load Stripe.js with publishable key from environment
export async function getStripe() {
	if (!browser) return null;

	// Try multiple sources for the Stripe key:
	// 1. Build-time environment variable (preferred for Vite)
	// 2. Runtime environment variable (Node.js process.env)
	// 3. Global window object (if set by server)
	let stripeKey = import.meta.env.VITE_PUBLIC_STRIPE_KEY;

	if (!stripeKey && typeof process !== 'undefined' && process.env) {
		stripeKey = process.env.PUBLIC_STRIPE_KEY;
	}

	if (!stripeKey && typeof window !== 'undefined' && (window as any).__STRIPE_KEY__) {
		stripeKey = (window as any).__STRIPE_KEY__;
	}

	if (!stripeKey) {
		console.warn('Stripe publishable key not found in environment variables');
		console.warn('Checked: VITE_PUBLIC_STRIPE_KEY, PUBLIC_STRIPE_KEY, window.__STRIPE_KEY__');
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
