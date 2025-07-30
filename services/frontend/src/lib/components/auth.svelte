<!-- auth.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/utils/helpers/backend';
	import { browser } from '$app/environment';
	import type { LoginResponse } from '$lib/auth';
	import { setAuthCookies, setAuthSessionStorage } from '$lib/auth';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';
	import { createEventDispatcher, onMount } from 'svelte';
	import '$lib/styles/splash.css';
	import ChipSection from '$lib/landing/ChipSection.svelte';
	import SiteFooter from '$lib/components/SiteFooter.svelte';

	const dispatch = createEventDispatcher();

	export let mode: 'login' | 'signup' = 'login';
	export let modalMode: boolean = false;
	export let inviteCode: string = '';
	let email = '';
	let password = '';
	let errorMessage = writable('');
	let loading = false;
	let isLoaded = false;
	let inviteValidation = { isValid: false, planName: '', trialDays: 0, validated: false };

	// Deep linking parameters
	let redirectPlan: string | null = null;
	let redirectType: string | null = null;

	// Update error message display
	let errorMessageText = '';
	errorMessage.subscribe((value) => {
		errorMessageText = value;
	});

	// Clear error message and reset form when switching between login/signup
	$: if (mode) {
		errorMessage.set('');
	}

	onMount(() => {
		if (browser) {
			document.title = mode === 'login' ? 'Login | Peripheral' : 'Sign Up | Peripheral';
			isLoaded = true;

			// Check for redirect parameters
			const urlParams = new URLSearchParams(window.location.search);
			redirectPlan = urlParams.get('plan');
			redirectType = urlParams.get('redirect');

			// Validate invite code if present
			if (inviteCode && inviteCode.trim() !== '' && mode === 'signup') {
				validateInviteCode(inviteCode.trim());
			}
		}
	});

	// Watch for changes to inviteCode and validate
	$: if (inviteCode && inviteCode.trim() !== '' && mode === 'signup' && browser) {
		validateInviteCode(inviteCode.trim());
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			if (mode === 'login') {
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
		dispatch('authSuccess', { type: mode, user });

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
				setAuthCookies(r.token, r.profilePic);
				setAuthSessionStorage(r.token, r.profilePic);
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
		// Prevent signup if invite code is present but invalid
		if (
			inviteCode &&
			inviteCode.trim() !== '' &&
			(!inviteValidation.validated || !inviteValidation.isValid)
		) {
			errorMessage.set('Please wait for invite code validation or use a valid invite code');
			return;
		}

		loading = true;
		try {
			const signupData: any = { email: email, password: password };

			// Include invite code if provided and valid
			if (inviteCode && inviteCode.trim() !== '' && inviteValidation.isValid) {
				signupData.inviteCode = inviteCode.trim();
			}

			await publicRequest('signup', signupData);
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

			// Store invite code for after Google auth
			if (inviteCode && inviteCode.trim() !== '') {
				sessionStorage.setItem('inviteCode', inviteCode.trim());
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

	async function validateInviteCode(code: string) {
		if (inviteValidation.validated && inviteValidation.isValid) {
			return; // Already validated successfully
		}

		try {
			const response = await publicRequest<{ code: string; planName: string; trialDays: number }>(
				'validateInvite',
				{ code }
			);
			inviteValidation = {
				isValid: true,
				planName: response.planName,
				trialDays: response.trialDays,
				validated: true
			};
			// Clear any previous error messages
			errorMessage.set('');
		} catch (error) {
			let displayError = 'Invalid invite code';
			if (typeof error === 'string') {
				const prefix = /^Server error: \d+ - /;
				displayError = error.replace(prefix, '');
			} else if (error instanceof Error) {
				const prefix = /^Server error: \d+ - /;
				displayError = error.message.replace(prefix, '');
			}

			inviteValidation = {
				isValid: false,
				planName: '',
				trialDays: 0,
				validated: true
			};
			errorMessage.set(displayError);
		}
	}

	// Exported method to set invite code from parent component
	export function setInviteCode(code: string) {
		inviteCode = code;
	}
</script>

<div class="auth-page">
	<!-- Main Auth Content -->
	<div class="auth-container">
		<!-- Header -->
		<div class="auth-header">
			<h1 class="auth-title">
				{mode === 'login' ? 'Sign into Peripheral' : 'Keep the Market within your Peripheral'}
			</h1>
		</div>

		<!-- Invite Code Status (only show in signup mode with invite) -->
		{#if mode === 'signup' && inviteCode && inviteCode.trim() !== ''}
			<div class="invite-status">
				{#if inviteValidation.validated}
					{#if inviteValidation.isValid}
						<div class="invite-valid">
							<svg
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								xmlns="http://www.w3.org/2000/svg"
							>
								<path
									d="M20 6L9 17L4 12"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
								/>
							</svg>
							Valid invite for {inviteValidation.planName} ({inviteValidation.trialDays} day trial)
						</div>
					{:else}
						<div class="invite-invalid">
							<svg
								width="16"
								height="16"
								viewBox="0 0 24 24"
								fill="none"
								xmlns="http://www.w3.org/2000/svg"
							>
								<path
									d="M18 6L6 18"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
								/>
								<path
									d="M6 6L18 18"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
								/>
							</svg>
							Invalid invite code
						</div>
					{/if}
				{:else}
					<div class="invite-validating">
						<div class="mini-loader"></div>
						Validating invite code...
					</div>
				{/if}
			</div>
		{/if}

		<!-- Auth Form -->
		<form
			on:submit|preventDefault={() => {
				if (mode === 'login') {
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
					<span>{mode === 'login' ? 'Login with Google' : 'Sign up with Google'}</span>
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
				<button type="submit" class="submit-button" disabled={loading}>
					{#if loading}
						<div class="loader"></div>
					{:else}
						{mode === 'login' ? 'Sign In' : 'Create Account'}
					{/if}
				</button>
			</div>
		</form>

		<!-- Toggle Auth Mode -->
		<div class="auth-toggle">
			{#if mode === 'login'}
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

	<!-- Ideas Chips Section -->
	<ChipSection />

	<!-- Footer -->
	<SiteFooter />
</div>

<style>
	/* Critical global styles - applied immediately to prevent layout shift */
	:global(*) {
		box-sizing: border-box;
	}

	:global(html) {
		-ms-overflow-style: none; /* IE and Edge */
		background: transparent !important; /* Override any global backgrounds */
		margin: 0;
		padding: 0;
	}

	:global(body) {
		-ms-overflow-style: none; /* IE and Edge */
		background: transparent !important; /* Override any global backgrounds */
		margin: 0;
		padding: 0;
	}

	/* Apply the same gradient background as landing page */
	.auth-page {
		width: 100%;
		min-height: 100vh;
		background: linear-gradient(180deg, #010022 0%, #02175f 100%);
		color: #f5f9ff;
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
		position: relative;
		overflow-x: hidden; /* Prevent horizontal scroll */
	}

	/* Auth-specific styles that build on splash system */
	.auth-container {
		width: 100%;
		max-width: 550px;
		margin: 0 auto;
		padding: 16rem 2rem 2rem 2rem; /* Space for header */
		background: transparent;
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
		color: #f5f9ff;
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
		border-radius: 12px;
		color: #000000;
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
		background: rgba(255, 255, 255, 0.9);
		transform: translateY(-1px);
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
		color: rgba(255, 255, 255, 0.6);
		font-size: 0.875rem;
		font-weight: 500;
	}

	.form-group {
		width: 100%;
	}

	/* Input styling */
	.auth-input {
		width: 100%;
		height: 52px;
		padding: 0 1rem;
		border: 1px solid rgba(255, 255, 255, 1);
		border-radius: 12px;
		background: rgba(255, 255, 255, 0.1);
		backdrop-filter: blur(10px);
		color: #ffffff;
		font-size: 0.95rem;
		font-family: 'Inter', sans-serif;
		transition: all 0.3s ease;
	}

	.auth-input::placeholder {
		color: #ffffff;
	}

	.auth-input:focus {
		outline: none;
		border-color: rgba(255, 255, 255, 0.6);
		background: rgba(255, 255, 255, 0.15);
		box-shadow: 0 0 0 3px rgba(255, 255, 255, 0.1);
	}

	.auth-input:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Submit button */
	.submit-button {
		width: 100%;
		height: 52px;
		background: #f5f9ff;
		color: #000000;
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
		background: #e0e0e0;
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(11, 46, 51, 0.3);
	}

	.submit-button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	/* Error message */
	.error-message {
		color: #ff6b6b;
		font-size: 0.875rem;
		margin: 0.5rem 0;
		text-align: center;
		background: rgba(255, 107, 107, 0.1);
		padding: 0.75rem;
		border-radius: 8px;
		border: 1px solid rgba(255, 107, 107, 0.3);
		backdrop-filter: blur(10px);
	}

	/* Auth Toggle */
	.auth-toggle {
		text-align: center;
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid rgba(255, 255, 255, 0.2);
		color: rgba(255, 255, 255, 0.8);
		font-size: 0.875rem;
	}

	.auth-link {
		color: #f5f9ff;
		text-decoration: none;
		font-weight: 600;
		transition: all 0.3s ease;
	}

	.auth-link:hover {
		color: #ffffff;
		text-decoration: underline;
	}

	/* Invite Status */
	.invite-status {
		width: 100%;
		margin-bottom: 1.5rem;
		text-align: center;
	}

	.invite-valid,
	.invite-invalid,
	.invite-validating {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		border-radius: 8px;
		font-size: 0.875rem;
		font-weight: 500;
		backdrop-filter: blur(10px);
	}

	.invite-valid {
		background: rgba(34, 197, 94, 0.1);
		border: 1px solid rgba(34, 197, 94, 0.3);
		color: #22c55e;
	}

	.invite-invalid {
		background: rgba(239, 68, 68, 0.1);
		border: 1px solid rgba(239, 68, 68, 0.3);
		color: #ef4444;
	}

	.invite-validating {
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.3);
		color: rgba(255, 255, 255, 0.8);
	}

	/* Loader */
	.loader {
		width: 20px;
		height: 20px;
		border: 2px solid rgba(0, 0, 0, 0.3);
		border-top: 2px solid #000000;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	.mini-loader {
		width: 16px;
		height: 16px;
		border: 2px solid rgba(255, 255, 255, 0.3);
		border-top: 2px solid rgba(255, 255, 255, 0.8);
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
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

	/* Ensure ChipSection is visible */
	:global(.auth-page .chip-section) {
		margin-top: 4rem;
		width: 100%;
		overflow: visible;
		position: relative;
		z-index: 10;
	}

	/* Override chip styles for dark background */
	:global(.auth-page .chip) {
		background: rgba(255, 255, 255, 0.95) !important;
		color: #000000 !important;
		border: 1px solid rgba(255, 255, 255, 0.3) !important;
	}

	:global(.auth-page .chip:hover) {
		background: rgba(255, 255, 255, 1) !important;
		box-shadow: 0 4px 16px rgba(255, 255, 255, 0.2) !important;
	}

	/* Ensure chip rows are visible */
	:global(.auth-page .chip-rows) {
		width: 100%;
		position: relative;
	}

	/* Ensure SiteFooter is at the bottom */
	:global(.auth-page .landing-footer) {
		margin-top: auto;
		position: relative;
		z-index: 10;
	}
</style>
