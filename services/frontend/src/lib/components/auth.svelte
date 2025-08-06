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

	export let mode: 'login' | 'signup' | 'verify' = 'login';
	export let modalMode: boolean = false;
	export let inviteCode: string = '';
	let email = '';
	let password = '';
	let otpDigits = ['', '', '', '', ''];
	let inputRefs: HTMLInputElement[] = [];
	let otp = 0;
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
			if (mode === 'login') {
				document.title = 'Login | Peripheral';
			} else if (mode === 'signup') {
				document.title = 'Sign Up | Peripheral';
			} else {
				document.title = 'Verify | Peripheral';
			}
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

	function otpHandleInput(index: number, event: Event) {
		if (event.target === null) {
			return
		}
		const input = event.target as HTMLInputElement;
		const value = input.value;

		// Only allow numbers
		if (!/^\d$/.test(value)) {
			input.value = '';
			otpDigits[index] = '';
			return;
		}

		otpDigits[index] = value;
		
		// Auto-focus next input
		if (value && index < 4) {
			inputRefs[index + 1]?.focus();
		}
		
		// Dispatch complete event when all 5 digits are filled
		if (otpDigits.every(digit => digit !== '')) {
			verify(email, +otpDigits.join(''));
		}
	}

	function otpHandleKeydown(index: number, event: KeyboardEvent): void {
		// Handle backspace
		if (event.key === 'Backspace') {
		if (!otpDigits[index] && index > 0) {
			// If current field is empty, go to previous and clear it
			otpDigits[index - 1] = '';
			inputRefs[index - 1]?.focus();
		} else if (otpDigits[index]) {
			// Clear current field
			otpDigits[index] = '';
		}
		}
		
		// Handle arrow keys
		if (event.key === 'ArrowRight' && index < 4) {
		inputRefs[index + 1]?.focus();
		}
		if (event.key === 'ArrowLeft' && index > 0) {
		inputRefs[index - 1]?.focus();
		}
	}

	function otpHandlePaste(event: ClipboardEvent): void {
		event.preventDefault();
		const pastedData = event.clipboardData?.getData('text') || '';
		
		// Extract only digits and take first 5
		const digits = pastedData.replace(/\D/g, '').slice(0, 5);
		
		if (digits.length > 0) {
			// Clear current OTP
			otpDigits = ['', '', '', '', ''];
			
			// Fill with pasted digits
			for (let i = 0; i < digits.length; i++) {
				otpDigits[i] = digits[i];
			}
			
			// Focus the next empty field or the last field
			const nextIndex = Math.min(digits.length, 4);
			inputRefs[nextIndex]?.focus();
			
			// Dispatch complete if we have 5 digits
			if (digits.length === 5) {
				verify(email, +digits);
			}
		}
	}

	function otpHandleFocus(index: number): void {
		// Select all text when focusing
		inputRefs[index]?.select();
	}

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

	async function sendVerificationOTP(email: string) {
		try {
			await publicRequest('sendVerificationOTP', { email });
		} catch (error) {
			errorMessage.set('Failed to send verification code');
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
			if (typeof error === 'string' && error.toLowerCase().includes('email address not verified')) {
				mode = 'verify';
				console.log("wowo this is so cool")
				sendVerificationOTP(email);
				errorMessage.set('Please check your email to verify your account');
				return;
			}
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

			mode = 'verify';

			sendVerificationOTP(email);

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
		} finally {
			loading = false;
		}
	}

	async function verify(email: string, otp: number) {
		loading = true;

		try {
			const verifyData: any = { email: email, otp: otp };

			await publicRequest('verifyOTP', verifyData);
			await signIn(email, password);
		} catch (error) {
			console.log(error);
			let displayError = 'Failed to verify OTP. Please try again.';
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
				{#if mode === 'login'}
				 Sign into Peripheral
				{:else if mode === 'signup'}
				 Keep the Market within your Peripheral
				{:else}
					 Verify Email Address
				{/if}
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
				} else if (mode === 'signup') {
					signUp(email, password);
				} else {
					verify(email, otp);
				}
			}}
			class="auth-form"
		>
			{#if mode === 'verify'}
			<!-- Verification Code Section -->
			<div class="verification-container">
			  <!-- Email Reminder -->
			  <div class="email-reminder">
				<div class="email-icon">
				  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
					<path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
					<polyline points="22,6 12,13 2,6"/>
				  </svg>
				</div>
				<p class="email-text">We sent a code to <strong>{email}</strong></p>
			  </div>
		  
			  <!-- OTP Input Grid -->
			  <div class="otp-container">
				<div class="otp-grid">
				  {#each otpDigits as digit, index}
					<input
					  bind:this={inputRefs[index]}
					  type="text"
					  inputmode="numeric"
					  maxlength="1"
					  bind:value={otpDigits[index]}
					  on:input={(e) => otpHandleInput(index, e)}
					  on:keydown={(e) => otpHandleKeydown(index, e)}
					  on:paste={otpHandlePaste}
					  on:focus={() => otpHandleFocus(index)}
					  disabled={loading}
					  placeholder=""
					  class="otp-digit {digit ? 'filled' : ''} {loading ? 'loading' : ''}"
					/>
				  {/each}
				</div>
				
				<p class="otp-instruction">Enter the 5-digit verification code</p>
			  </div>
		  
			  <!-- Status Section -->
			  {#if loading}
				<div class="status-container verifying">
				  <div class="spinner"></div>
				  <span>Verifying your code...</span>
				</div>
			  {/if}
		  
			  <!-- Resend Section -->
			  <div class="resend-section">
				<p class="resend-text">Didn't receive the code?</p>
				<button
				  type="button"
				  class="resend-button"
				  on:click={() => sendVerificationOTP(email)}
				  disabled={loading}
				>
				  Resend Code
				</button>
			  </div>
			</div>
			{:else}

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

							<!-- Submit Button -->
			<div class="form-group">
				<button type="submit" class="submit-button" disabled={loading}>
					{#if loading}
						<div class="loader"></div>
					{:else}
						{#if mode === 'login'}
							Sign In
						{:else if mode === 'signup'}
							Create Account
						{/if}
					
					{/if}
				</button>
			</div>


			{/if}

			<!-- Error Message -->
			{#if errorMessageText}
				<p class="error-message">{errorMessageText}</p>
			{/if}

		</form>

		<!-- Toggle Auth Mode -->
		<div class="auth-toggle">
			{#if mode === 'login'}
				<p>
					Don't have an account?
					<a href="/signup" on:click={handleToggleMode} class="auth-link">Sign Up</a>
				</p>
			{:else if mode === 'signup'}
				<p>
					Already have an account?
					<a href="/login" on:click={handleToggleMode} class="auth-link">Sign In</a>
				</p>
			{:else}
				<p>
					Don't have an account?
					<a href="/signup" on:click={handleToggleMode} class="auth-link">Sign Up</a>
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
			Geist,
			Inter,
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
		padding: 16rem 2rem 2rem; /* Space for header */
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
		margin: 0 0 0.5rem;
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
		background: rgb(255 255 255 / 100%);
		border-radius: 12px;
		color: #000;
		font-family: Inter, sans-serif;
		font-size: 0.95rem;
		font-weight: 500;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
	}

	.google-login-button:hover:not(:disabled) {
		background: rgb(255 255 255 / 90%);
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
		color: rgb(255 255 255 / 60%);
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
		border: 1px solid rgb(255 255 255 / 100%);
		border-radius: 12px;
		background: rgb(255 255 255 / 10%);
		backdrop-filter: blur(10px);
		color: #fff;
		font-size: 0.95rem;
		font-family: Inter, sans-serif;
		transition: all 0.3s ease;
	}

	.auth-input::placeholder {
		color: #fff;
	}

	.auth-input:focus {
		outline: none;
		border-color: rgb(255 255 255 / 60%);
		background: rgb(255 255 255 / 15%);
		box-shadow: 0 0 0 3px rgb(255 255 255 / 10%);
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
		color: #000;
		border: none;
		border-radius: 12px;
		font-size: 0.95rem;
		font-weight: 600;
		font-family: Inter, sans-serif;
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
		box-shadow: 0 4px 12px rgb(11 46 51 / 30%);
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
		background: rgb(255 107 107 / 10%);
		padding: 0.75rem;
		border-radius: 8px;
		border: 1px solid rgb(255 107 107 / 30%);
		backdrop-filter: blur(10px);
	}

	/* Auth Toggle */
	.auth-toggle {
		text-align: center;
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid rgb(255 255 255 / 20%);
		color: rgb(255 255 255 / 80%);
		font-size: 0.875rem;
	}

	.auth-link {
		color: #f5f9ff;
		text-decoration: none;
		font-weight: 600;
		transition: all 0.3s ease;
	}

	.auth-link:hover {
		color: #fff;
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
		background: rgb(34 197 94 / 10%);
		border: 1px solid rgb(34 197 94 / 30%);
		color: #22c55e;
	}

	.invite-invalid {
		background: rgb(239 68 68 / 10%);
		border: 1px solid rgb(239 68 68 / 30%);
		color: #ef4444;
	}

	.invite-validating {
		background: rgb(255 255 255 / 10%);
		border: 1px solid rgb(255 255 255 / 30%);
		color: rgb(255 255 255 / 80%);
	}

	/* Loader */
	.loader {
		width: 20px;
		height: 20px;
		border: 2px solid rgb(0 0 0 / 30%);
		border-top: 2px solid #000;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	.mini-loader {
		width: 16px;
		height: 16px;
		border: 2px solid rgb(255 255 255 / 30%);
		border-top: 2px solid rgb(255 255 255 / 80%);
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
	@media (width <= 480px) {
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
		background: rgb(255 255 255 / 95%) !important;
		color: #000 !important;
		border: 1px solid rgb(255 255 255 / 30%) !important;
	}

	:global(.auth-page .chip:hover) {
		background: rgb(255 255 255 / 100%) !important;
		box-shadow: 0 4px 16px rgb(255 255 255 / 20%) !important;
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


	.verification-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2rem;
    width: 100%;
    padding: 1rem 0;
  }

  .email-reminder {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
    text-align: center;
  }

  .email-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
    color: white;
    border-radius: 12px;
    box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  }

  .email-text {
    color: #6b7280;
    font-size: 0.9rem;
    line-height: 1.5;
    margin: 0;
  }

  .email-text strong {
    color: #374151;
    font-weight: 600;
  }

  .otp-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
  }

  .otp-grid {
    display: flex;
    gap: 0.75rem;
    align-items: center;
  }

  .otp-digit {
    width: 56px;
    height: 56px;
    border: 2px solid #e5e7eb;
    border-radius: 12px;
    text-align: center;
    font-size: 1.5rem;
    font-weight: 600;
    color: #1f2937;
    background: #ffffff;
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
    outline: none;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
    position: relative;
  }

  .otp-digit::placeholder {
    color: transparent;
  }

  .otp-digit:hover:not(:disabled) {
    border-color: #3b82f6;
    box-shadow: 0 4px 12px rgba(59, 130, 246, 0.15);
    transform: translateY(-1px);
  }

  .otp-digit:focus {
    border-color: #3b82f6;
    box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1), 0 4px 12px rgba(59, 130, 246, 0.15);
    transform: translateY(-1px);
  }

  .otp-digit.filled {
    border-color: #10b981;
    background: linear-gradient(135deg, #ecfdf5 0%, #f0fdf4 100%);
    color: #065f46;
    box-shadow: 0 4px 12px rgba(16, 185, 129, 0.15);
  }

  .otp-digit.loading {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .otp-instruction {
    color: #6b7280;
    font-size: 0.875rem;
    margin: 0;
    text-align: center;
  }

  .status-container {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.875rem;
    font-weight: 500;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    background: #f8fafc;
    border: 1px solid #e2e8f0;
  }

  .status-container.verifying {
    color: #3b82f6;
    background: linear-gradient(135deg, #eff6ff 0%, #f0f9ff 100%);
    border-color: #bfdbfe;
  }

  .spinner {
    width: 16px;
    height: 16px;
    border: 2px solid #bfdbfe;
    border-top-color: #3b82f6;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .resend-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    text-align: center;
  }

  .resend-text {
    color: #6b7280;
    font-size: 0.875rem;
    margin: 0;
  }

  .resend-button {
    background: none;
    border: none;
    color: #3b82f6;
    font-size: 0.875rem;
    font-weight: 600;
    cursor: pointer;
    padding: 0.5rem 1rem;
    border-radius: 6px;
    transition: all 0.2s ease;
  }

  .resend-button:hover:not(:disabled) {
    background: #eff6ff;
    color: #1d4ed8;
  }

  .resend-button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  /* Mobile responsiveness */
  @media (max-width: 480px) {
    .verification-container {
      gap: 1.5rem;
    }

    .otp-grid {
      gap: 0.5rem;
    }

    .otp-digit {
      width: 48px;
      height: 48px;
      font-size: 1.25rem;
    }
  }
</style>
