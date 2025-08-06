<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { publicRequest } from '$lib/utils/helpers/backend';
	import type { GoogleCallbackResponse } from '$lib/auth';
	import { setAuthCookies, setAuthSessionStorage } from '$lib/auth';

	let errorMessage = '';

	onMount(async () => {
		const urlParams = new URLSearchParams(window.location.search);
		const code = urlParams.get('code');
		const state = urlParams.get('state');
		const error = urlParams.get('error');

		// Handle OAuth errors first
		if (error) {
			console.error('OAuth error received:', error);
			errorMessage = 'Authentication failed. Please try again.';
			setTimeout(() => goto('/login'), 3000);
			return;
		}

		// Verify the state parameter matches what we stored
		const storedState = sessionStorage.getItem('googleAuthState');

		if (!code) {
			console.error('No authorization code received');
			errorMessage = 'Authentication failed. Please try again.';
			setTimeout(() => goto('/login'), 3000);
			return;
		}

		// More lenient state checking for development
		const isDevelopment =
			window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';

		if (!state) {
			console.error('No state parameter in callback');
			errorMessage = 'Authentication failed. Please try again.';
			setTimeout(() => goto('/login'), 3000);
			return;
		}

		if (!storedState) {
			console.error('No stored state found in sessionStorage');
			// In development, we can be more lenient, but still log the issue
			if (!isDevelopment) {
				errorMessage = 'Authentication failed. Please try again.';
				setTimeout(() => goto('/login'), 3000);
				return;
			}
			console.warn('Development mode: proceeding without state verification');
		} else if (state !== storedState) {
			console.error('State mismatch', { received: state, stored: storedState });
			// In development, log but don't fail immediately
			if (!isDevelopment) {
				errorMessage = 'Authentication failed. Please try again.';
				setTimeout(() => goto('/login'), 3000);
				return;
			}
			console.warn('Development mode: proceeding despite state mismatch');
		}

		try {
			// Retrieve stored invite code if it exists
			const storedInviteCode = sessionStorage.getItem('inviteCode');

			const requestData: any = {
				code,
				state
			};

			// Include invite code if it was stored
			if (storedInviteCode) {
				requestData.inviteCode = storedInviteCode;
			}

			const response = await publicRequest<GoogleCallbackResponse>('googleCallback', requestData);

			// Set auth data using centralized utilities
			setAuthCookies(response.token, response.profilePic);
			setAuthSessionStorage(response.token, response.profilePic);

			// Clean up stored state and invite code
			sessionStorage.removeItem('googleAuthState');
			if (storedInviteCode) {
				sessionStorage.removeItem('inviteCode');
			}

			// Handle deep linking from stored parameters
			const redirectPlan = sessionStorage.getItem('redirectPlan');
			const redirectType = sessionStorage.getItem('redirectType');

			if (redirectType === 'checkout' && redirectPlan) {
				// Clean up stored redirect parameters
				sessionStorage.removeItem('redirectPlan');
				sessionStorage.removeItem('redirectType');
				// Redirect to pricing page with plan parameter to trigger checkout
				goto(`/pricing?upgrade=${redirectPlan}`);
			} else {
				// Default redirect to app
				goto('/app');
			}
		} catch (error) {
			console.error('Google authentication failed:', error);

			// Extract specific error message from backend response
			let displayError = 'Authentication failed. Please try again.';
			if (typeof error === 'string') {
				// It usually comes prefixed like "Server error: 400 - actual message"
				const prefix = /^Server error: \d+ - /;
				displayError = error.replace(prefix, '');
			} else if (error instanceof Error) {
				const prefix = /^Server error: \d+ - /;
				displayError = error.message.replace(prefix, '');
			}

			errorMessage = displayError;
			setTimeout(() => goto('/login'), 3000);
		}
	});
</script>

<div class="container">
	{#if errorMessage}
		<div class="error">
			<h2>Authentication Error</h2>
			<p>{errorMessage}</p>
			<p>Redirecting to login page...</p>
		</div>
	{:else}
		<!-- Enhanced loading state -->
		<div class="loading-container">
			<div class="spinner"></div>
			<p>Authenticating with Google...</p>
			<p>Please wait.</p>
		</div>
	{/if}
</div>

<style>
	/* Critical CSS to prevent white flash and scrollbar issues */
	:global(*) {
		box-sizing: border-box;
	}

	:global(html),
	:global(body) {
		width: 100%;
		height: 100%;
		margin: 0;
		padding: 0;
		overflow: hidden;
		background-color: #1a1c21;
		color: #f9fafb;
		font-family:
			Inter,
			-apple-system,
			BlinkMacSystemFont,
			'Segoe UI',
			Roboto,
			sans-serif;
		position: fixed;
		top: 0;
		left: 0;

		/* Prevent overscroll/bounce effects */
		overscroll-behavior: none;

		/* Prevent pull-to-refresh on mobile */
		overscroll-behavior-y: none;
	}

	.container {
		display: flex;
		justify-content: center;
		align-items: center;
		min-height: 100vh;
		width: 100%;
		background-color: #1a1c21;
		color: #f9fafb;
		font-family: Inter, sans-serif;
		padding: 1rem;
		box-sizing: border-box;
		position: relative;
		z-index: 1;
	}

	.loading-container {
		text-align: center;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}

	.loading-container p {
		margin: 0;
		color: #9ca3af;
		font-size: 1rem;
	}

	.loading-container p:first-of-type {
		color: #f9fafb;
		font-size: 1.1rem;
		font-weight: 500;
	}

	.error {
		color: #ef4444;
		text-align: center;
		max-width: 450px;
		padding: 1.5rem 2rem;
		background-color: rgb(45 49 57 / 80%);
		border-radius: 6px;
		border: 1px solid #ef4444;
	}

	.error h2 {
		margin-top: 0;
		margin-bottom: 1rem;
		color: #f9fafb;
	}

	.error p {
		margin-bottom: 0.5rem;
		color: #9ca3af;
	}

	/* Simple CSS Spinner */
	.spinner {
		width: 40px;
		height: 40px;
		border: 4px solid #374151;
		border-top: 4px solid #3b82f6;
		border-radius: 50%;
		animation: spin 1s linear infinite;
		margin-bottom: 0.5rem;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}

		100% {
			transform: rotate(360deg);
		}
	}
</style>
