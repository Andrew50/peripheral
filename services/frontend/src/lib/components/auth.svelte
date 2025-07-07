<!-- auth.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/utils/helpers/backend';
	import '$lib/styles/global.css';
	import '$lib/styles/landing.css';
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
			document.title = loginMenu ? 'Login - Peripheral' : 'Sign Up - Peripheral';
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

	function navigateToHome() {
		goto('/');
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

<!-- Use landing page design system -->
<div class="landing-background landing-reset">
	<!-- Background Effects -->
	<div class="landing-background-animation">
		<div class="landing-gradient-orb landing-orb-1"></div>
		<div class="landing-gradient-orb landing-orb-2"></div>
		<div class="landing-gradient-orb landing-orb-3"></div>
		<div class="landing-static-gradient"></div>
	</div>

	<!-- Main Auth Content -->
	<div class="landing-container centered" style="padding-top: 120px;">
		<div class="auth-container">
			<div
				class="landing-glass-card auth-card"
				class:landing-fade-in={true}
				class:loaded={isLoaded}
			>
				<!-- Header -->
				<div class="auth-header">
					<h1 class="auth-title">
						{loginMenu ? 'Welcome back' : 'Get started'}
					</h1>
					<p class="landing-subtitle">
						{loginMenu ? 'Sign in to your account' : 'Create your account to start trading'}
					</p>
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
					class="landing-form"
				>
					<!-- Google Login Button -->
					<div class="landing-form-group">
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
						<span>or</span>
					</div>

					<!-- Email Input -->
					<div class="landing-form-group">
						<input
							type="email"
							id="email"
							bind:value={email}
							required
							on:keydown={handleKeydown}
							placeholder="Email"
							class="landing-input"
							disabled={loading}
						/>
					</div>

					<!-- Password Input -->
					<div class="landing-form-group">
						<input
							type="password"
							id="password"
							bind:value={password}
							required
							on:keydown={handleKeydown}
							placeholder="Password"
							class="landing-input"
							disabled={loading}
						/>
					</div>

					<!-- Error Message -->
					{#if errorMessageText}
						<p class="landing-text-error">{errorMessageText}</p>
					{/if}

					<!-- Submit Button -->
					<div class="landing-form-group">
						<button
							type="submit"
							class="landing-button primary large full-width"
							disabled={loading}
						>
							{#if loading}
								<div class="landing-loader"></div>
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
							<a href="/signup" on:click={handleToggleMode} class="landing-link">Sign Up</a>
						</p>
					{:else}
						<p>
							Already have an account?
							<a href="/login" on:click={handleToggleMode} class="landing-link">Sign In</a>
						</p>
					{/if}
				</div>
			</div>
		</div>
	</div>
</div>

<style>
	/* Auth-specific styles that build on landing system */
	.auth-container {
		width: 100%;
		max-width: 450px;
		margin: 0 auto;
	}

	.auth-card {
		padding: 2.5rem;
	}

	.auth-header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.auth-title {
		font-size: 2rem;
		font-weight: 700;
		margin: 0 0 0.5rem 0;
		color: var(--landing-text-primary);
	}

	/* Google Login Button */
	.google-login-button {
		width: 100%;
		height: 48px;
		background: rgba(255, 255, 255, 0.05);
		border: 1px solid var(--landing-border);
		border-radius: 8px;
		color: var(--landing-text-primary);
		font-family: 'Inter', sans-serif;
		font-size: 0.95rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		backdrop-filter: blur(5px);
	}

	.google-login-button:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.08);
		border-color: rgba(255, 255, 255, 0.2);
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
		background: var(--landing-border);
	}

	.auth-divider span {
		background: var(--landing-glass-bg);
		color: var(--landing-text-secondary);
		padding: 0 1rem;
		font-size: 0.875rem;
		position: relative;
		z-index: 1;
	}

	/* Auth Toggle */
	.auth-toggle {
		text-align: center;
		margin-top: 2rem;
		padding-top: 2rem;
		border-top: 1px solid var(--landing-border);
		color: var(--landing-text-secondary);
		font-size: 0.875rem;
	}

	/* Responsive Design */
	@media (max-width: 480px) {
		.auth-card {
			padding: 2rem 1.5rem;
			margin: 1rem;
		}

		.auth-title {
			font-size: 1.75rem;
		}

		.google-login-button,
		.landing-input {
			height: 44px;
			font-size: 0.9rem;
		}
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
</style>
