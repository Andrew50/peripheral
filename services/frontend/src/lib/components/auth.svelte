<!-- auth.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/utils/helpers/backend';
	import '$lib/styles/splash.css';
	import { browser } from '$app/environment';
	import type { LoginResponse } from '$lib/auth';
	import { setAuthCookies, setAuthSessionStorage } from '$lib/auth';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';
	import { createEventDispatcher, onMount } from 'svelte';
	import { page } from '$app/stores';

	const dispatch = createEventDispatcher();

	export let loginMenu: boolean = false;
	export let modalMode: boolean = false;
	let email = '';
	let password = '';
	let errorMessage = writable('');
	let loading = false;
	let isLoaded = false;

	// Deep linking parameters
	let redirectPlan: string | null = null;
	let redirectType: string | null = null;

	// Update error message display
	let errorMessageText = '';
	errorMessage.subscribe((value) => {
		errorMessageText = value;
	});

	// Clear error message and reset form when switching between login/signup
	$: if (loginMenu !== undefined) {
		errorMessage.set('');
	}

	onMount(() => {
		if (browser) {
			document.title = loginMenu ? 'Login | Peripheral' : 'Sign Up |	 Peripheral';
			isLoaded = true;

			// Check for redirect parameters
			const urlParams = new URLSearchParams(window.location.search);
			redirectPlan = urlParams.get('plan');
			redirectType = urlParams.get('redirect');
		}
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			if (loginMenu) {
				signIn(email, password);
			} else {
				signUp(email, password);
			}
		}
	}

	function handleToggleMode(event: Event) {
		if (modalMode) {
			event.preventDefault();
			dispatch('toggleMode');
		}
		// If not in modal mode, let the default link behavior happen
	}
	// Handle successful authentication with deep linking
	function handleAuthSuccess(user: LoginResponse) {
		// Dispatch success event for modal usage
		dispatch('authSuccess', { type: loginMenu ? 'login' : 'signup', user });

		// Handle deep linking
		if (redirectType === 'checkout' && redirectPlan) {
			// Redirect to pricing page with plan parameter to trigger checkout
			goto(`/pricing?upgrade=${redirectPlan}`);
		} else {
			// Default redirect to app
			goto('/app');
		}
	}

	async function signIn(email: string, password: string) {
		loading = true;
		try {
			const r = await publicRequest<LoginResponse>('login', { email: email, password: password });
			if (browser) {
				// Set auth data using centralized utilities
				setAuthCookies(r.token, r.profilePic, r.username);
				setAuthSessionStorage(r.token, r.profilePic, r.username);
			}

			handleAuthSuccess(r);
		} catch (error) {
			let displayError = 'Login failed. Please try again.';
			if (typeof error === 'string') {
				// Extract the core message sent from the backend
				// It usually comes prefixed like "Server error: 400 - actual message"
				const prefix = /^Server error: \d+ - /;
				displayError = error.replace(prefix, '');
			} else if (error instanceof Error) {
				const prefix = /^Server error: \d+ - /;
				displayError = error.message.replace(prefix, '');
			}
			errorMessage.set(displayError);
		} finally {
			loading = false;
		}
	}

	async function signUp(email: string, password: string) {
		loading = true;
		try {
			await publicRequest('signup', { email: email, password: password });
			await signIn(email, password);
		} catch (error) {
			console.log(error);
			let displayError = 'Failed to create account. Please try again.';
			if (typeof error === 'string') {
				// Extract the core message sent from the backend
				const prefix = /^Server error: \d+ - /;
				displayError = error.replace(prefix, '');
			} else if (error instanceof Error) {
				const prefix = /^Server error: \d+ - /;
				displayError = error.message.replace(prefix, '');
			}
			errorMessage.set(displayError);
			loading = false;
		}
	}

	async function handleGoogleLogin() {
		try {
			// Get and log the current origin
			const currentOrigin = window.location.origin;

			// Store redirect parameters for after Google auth
			if (redirectPlan && redirectType) {
				sessionStorage.setItem('redirectPlan', redirectPlan);
				sessionStorage.setItem('redirectType', redirectType);
			}

			// Pass the current origin to the backend
			const response = await publicRequest<{ url: string; state: string }>('googleLogin', {
				redirectOrigin: currentOrigin
			});

			// Store the state in sessionStorage to verify on return
			if (response.state) {
				sessionStorage.setItem('googleAuthState', response.state);
			}

			// Redirect to Google's OAuth page
			window.location.href = response.url;
		} catch (error) {
			console.error('Failed to initialize Google login:', error);
			errorMessage.set('Failed to initialize Google login');
		}
	}
</script>

<!-- Use consistent light theme design -->
<div class="auth-page">
	<!-- Main Auth Content -->
	<div class="auth-container" style="padding-top: 200px;">
		<!-- Header -->
		<div class="auth-header">
			<h1 class="auth-title">
				{loginMenu ? 'Sign into Peripheral' : 'Research, analyze, and execute with Peripheral'}
			</h1>
		</div>

		<!-- Auth Form -->
		<form
			on:submit|preventDefault={() => {
				if (loginMenu) {
					signIn(email, password);
				} else {
					signUp(email, password);
				}
			}}
			class="auth-form"
		>
			<!-- Google Login Button -->
			<div class="form-group">
				<button
					class="google-login-button"
					on:click={handleGoogleLogin}
					type="button"
					disabled={loading}
				>
					<div class="google-icon">
						<svg
							version="1.1"
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 48 48"
							xmlns:xlink="http://www.w3.org/1999/xlink"
							style="display: block;"
						>
							<path
								fill="#EA4335"
								d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"
							></path>
							<path
								fill="#4285F4"
								d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"
							></path>
							<path
								fill="#FBBC05"
								d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"
							></path>
							<path
								fill="#34A853"
								d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"
							></path>
							<path fill="none" d="M0 0h48v48H0z"></path>
						</svg>
					</div>
					<span>Continue with Google</span>
				</button>
			</div>

			<!-- Divider -->
			<div class="auth-divider">
				<span>OR</span>
			</div>

			<!-- Email Input -->
			<div class="form-group">
				<input
					type="email"
					id="email"
					bind:value={email}
					required
					on:keydown={handleKeydown}
					placeholder="Email"
					class="auth-input"
					disabled={loading}
				/>
			</div>

			<!-- Password Input -->
			<div class="form-group">
				<input
					type="password"
					id="password"
					bind:value={password}
					required
					on:keydown={handleKeydown}
					placeholder="Password"
					class="auth-input"
					disabled={loading}
				/>
			</div>

			<!-- Error Message -->
			{#if errorMessageText}
				<p class="error-message">{errorMessageText}</p>
			{/if}

			<!-- Submit Button -->
			<div class="form-group">
				<button
					type="submit"
					class="submit-button"
					disabled={loading}
				>
					{#if loading}
						<div class="loader"></div>
					{:else}
						{loginMenu ? 'Sign In' : 'Create Account'}
					{/if}
				</button>
			</div>
		</form>

		<!-- Toggle Auth Mode -->
		<div class="auth-toggle">
			{#if loginMenu}
				<p>
					Don't have an account?
					<a href="/signup" on:click={handleToggleMode} class="auth-link">Sign Up</a>
				</p>
			{:else}
				<p>
					Already have an account?
					<a href="/login" on:click={handleToggleMode} class="auth-link">Sign In</a>
				</p>
			{/if}
		</div>
	</div>
</div>

<style>
	/* Use splash.css color system for consistency */
	.auth-page {
		width: 100%;
		min-height: 70vh;
		background: linear-gradient(135deg, var(--color-light) 0%, var(--color-accent) 100%);
		color: var(--color-dark);
		font-family: 'Geist', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		display: flex;
		flex-direction: column;
		position: relative;
	}

	/* Auth-specific styles that build on splash system */
	.auth-container {
		width: 100%;
		max-width: 550px;
		margin: 0 auto;
		padding-left: 2rem;
		padding-right: 2rem;
		padding-bottom: 0;
		padding-top: 200px;
	}

	.auth-header {
		text-align: center;
		margin-bottom: 2.5rem;
		width: 100%;
	}

	.auth-title {
		width: 100%;
		display: block;
		text-align: center;
		font-size: 2rem;
		font-weight: 700;
		margin: 0 0 0.5rem 0;
		color: var(--color-dark);
		line-height: 1.2;
	}

	.auth-form {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		align-items: center;
		width: 100%;
	}

	/* Google Login Button */
	.google-login-button {
		width: 100%;
		height: 52px;
		background: rgba(255, 255, 255, 1);
		border: 1.5px solid #000000;
		border-radius: 12px;
		color: var(--color-dark);
		font-family: 'Inter', sans-serif;
		font-size: 0.95rem;
		font-weight: 500;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
	}

	.google-login-button:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.8);
		box-shadow: 0 4px 12px rgba(79, 124, 130, 0.15);
	}

	.google-login-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.google-icon {
		width: 20px;
		height: 20px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	/* Divider */
	.auth-divider {
		position: relative;
		text-align: center;
		margin: 1.5rem 0;
	}

	.auth-divider::before {
		content: '';
		position: absolute;
		top: 50%;
		left: 0;
		right: 0;
		height: 1px;
		background: linear-gradient(90deg, transparent, var(--color-primary), transparent);
		opacity: 0.3;
	}

	.auth-divider span {
		background: rgba(255, 255, 255, 0.95);
		color: var(--color-primary);
		padding: 0 1rem;
		font-size: 0.875rem;
		font-weight: 500;
		position: relative;
		z-index: 1;
	}

	.form-group {
		width: 100%;
	}

	/* Input styling */
	.auth-input {
		width: 100%;
		height: 52px;
		padding: 0 1rem;
		border: 1.5px solid #000000;
		border-radius: 12px;
		background: rgba(255, 255, 255, 1);
		color: var(--color-dark);
		font-size: 0.95rem;
		font-family: 'Inter', sans-serif;
		transition: all 0.3s ease;
	}

	.auth-input::placeholder {
		color: var(--color-primary);
		opacity: 0.6;
	}

	.auth-input:focus {
		outline: none;
		border-color: var(--color-primary);
		background: rgba(255, 255, 255, 0.9);
		box-shadow: 0 0 0 3px rgba(79, 124, 130, 0.1);
	}

	.auth-input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Submit button */
	.submit-button {
		width: 100%;
		height: 52px;
		background: var(--color-dark);
		color: white;
		border: none;
		border-radius: 12px;
		font-size: 0.95rem;
		font-weight: 600;
		font-family: 'Inter', sans-serif;
		cursor: pointer;
		transition: all 0.3s ease;
		display: flex;
		align-items: center;
		justify-content: center;
		margin-top: 0.5rem;
	}

	.submit-button:hover:not(:disabled) {
		background: #08262a;
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(11, 46, 51, 0.3);
	}

	.submit-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Error message */
	.error-message {
		color: #dc2626;
		font-size: 0.875rem;
		margin: 0.5rem 0;
		text-align: center;
		background: rgba(220, 38, 38, 0.1);
		padding: 0.75rem;
		border-radius: 8px;
		border: 1px solid rgba(220, 38, 38, 0.2);
	}

	/* Auth Toggle */
	.auth-toggle {
		text-align: center;
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid rgba(79, 124, 130, 0.2);
		color: var(--color-primary);
		font-size: 0.875rem;
	}

	.auth-link {
		color: var(--color-dark);
		text-decoration: none;
		font-weight: 600;
		transition: all 0.3s ease;
	}

	.auth-link:hover {
		color: #08262a;
		text-decoration: underline;
	}

	/* Loader */
	.loader {
		width: 20px;
		height: 20px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top: 2px solid white;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		0% { transform: rotate(0deg); }
		100% { transform: rotate(360deg); }
	}

	/* Responsive Design */
	@media (max-width: 480px) {
		.auth-title {
			font-size: 1.75rem;
		}

		.google-login-button,
		.auth-input,
		.submit-button {
			height: 48px;
			font-size: 0.9rem;
		}
	}
</style>
