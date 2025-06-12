<!-- account.svelte -->
<script lang="ts">
	import { publicRequest, privateRequest, base_url } from '$lib/utils/helpers/backend';
	import '$lib/styles/global.css';
	import { browser } from '$app/environment';

	import Header from '$lib/components/header.svelte';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';
	import { createEventDispatcher } from 'svelte';

	const dispatch = createEventDispatcher();

	export let loginMenu: boolean = false;
	let email = '';
	let username = '';
	let password = '';
	let errorMessage = writable('');
	let loading = false;
	let guestLoading = false;

	// Update error message display
	let errorMessageText = '';
	errorMessage.subscribe((value) => {
		errorMessageText = value;
	});

	// Add error handling for missing assets
	let googleIconUrl = '/google-icon.svg';
	let googleIconError = false;

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			if (loginMenu) {
				signIn(email, password);
			} else {
				signUp(email, username, password);
			}
		}
	}

	function handleGoogleIconError() {
		googleIconError = true;
	}

	interface Login {
		token: string;
		settings: string;
		profilePic: string;
		username: string;
	}

	async function signIn(email: string, password: string) {
		loading = true;
		try {
			// Block guest credentials when called directly from the login form
			// This prevents users from manually entering guest credentials in the login form
			// but still allows the guest login button to work via handleGuestLogin
			const isDirectFormSubmission = !guestLoading;
			if (isDirectFormSubmission && email === 'user' && password === 'pass') {
				throw new Error('Please use the "Continue as Guest" button to access the guest account');
			}

			const r = await publicRequest<Login>('login', { email: email, password: password });
			if (browser) {
				// Remove any existing guest session cleanup event listener
				window.removeEventListener('beforeunload', cleanupGuestAccount);

				// Clear any guest session flags
				sessionStorage.removeItem('isGuestSession');

				// Set the regular session data
				sessionStorage.setItem('authToken', r.token);
				sessionStorage.setItem('profilePic', r.profilePic);
				sessionStorage.setItem('username', r.username);
			}
			
			// Dispatch success event for modal usage
			dispatch('authSuccess', { type: 'login', user: r });
			
			goto('/app');
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

	async function signUp(email: string, username: string, password: string) {
		loading = true;
		try {
			// If this was a guest account, remove guest session cleanup
			if (browser) {
				window.removeEventListener('beforeunload', cleanupGuestAccount);
				sessionStorage.removeItem('isGuestSession');
				sessionStorage.removeItem('userId');
			}

			await publicRequest('signup', { email: email, username: username, password: password });
			await signIn(email, password);
		} catch (error) {
            console.log(error)
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

	/*async function handleGuestLogin() {
		guestLoading = true;
		try {
			// Use the dedicated guestLogin endpoint instead of the regular login
			const r = await publicRequest<Login>('guestLogin', {});

			if (browser) {
				sessionStorage.setItem('authToken', r.token);
				sessionStorage.setItem('profilePic', r.profilePic || '');
				sessionStorage.setItem('username', r.username);
				// Mark this as a guest session to handle cleanup on page close
				sessionStorage.setItem('isGuestSession', 'true');

				// Set up event listener for page unload to delete the guest account
				window.addEventListener('beforeunload', cleanupGuestAccount);
			}
			goto('/app');
		} catch (error) {
			if (error instanceof Error) {
				errorMessage.set(error.message);
			} else {
				errorMessage.set('Guest login failed. Please try again.');
			}
		} finally {
			guestLoading = false;
		}
	}*/

	// Function to clean up guest account when page is closed
	async function cleanupGuestAccount() {
		try {
			// Check if this is a guest session
			const isGuest = sessionStorage.getItem('isGuestSession') === 'true';

			if (isGuest) {
				try {
					// Use privateRequest with the keepalive option for page unload events
					// This ensures the request completes even when the page is unloading
					privateRequest(
						'deleteAccount',
						{ confirmation: 'DELETE' },
						false, // verbose
						true // keepalive
					);
				} catch (e) {
					console.error('Error sending account deletion request:', e);
				}

				// Clear session storage
				sessionStorage.removeItem('authToken');
				sessionStorage.removeItem('profilePic');
				sessionStorage.removeItem('username');
				sessionStorage.removeItem('isGuestSession');
			}
		} catch (error) {
			console.error('Error cleaning up guest account:', error);
		}
	}
</script>

<div class="page-wrapper">
	<Header />
	<div class="auth-container">
		<div class="auth-card responsive-shadow responsive-border content-padding">
			<h1>{loginMenu ? 'Welcome Back' : 'Create Account'}</h1>
			<p class="subtitle fluid-text">
				{loginMenu ? 'Sign in to access your account' : 'Start your trading journey today'}
			</p>

			<div class="auth-buttons-container">
				<button class="gsi-material-button responsive-shadow" on:click={handleGoogleLogin}>
					<div class="gsi-material-button-state"></div>
					<div class="gsi-material-button-content-wrapper">
						<div class="gsi-material-button-icon">
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
						<span class="gsi-material-button-contents">Sign in with Google</span>
						<span style="display: none;">Sign in with Google</span>
					</div>
				</button>

				<!--<button
					class="guest-button responsive-shadow"
					on:click={handleGuestLogin}
					disabled={guestLoading}
				>
					{#if guestLoading}
						<span class="loader"></span>
					{:else}
						Continue as Guest
					{/if}
				</button>-->
			</div>

			<div class="divider">
				<span class="fluid-text">or continue with email</span>
			</div>

			<form
				on:submit|preventDefault={() => {
					if (loginMenu) {
						signIn(email, password);
					} else {
						signUp(email, username, password);
					}
				}}
				class="auth-form"
			>
				<div class="form-group">
					<label for="email">Email</label>
					<input
						type="email"
						id="email"
						bind:value={email}
						required
						on:keydown={handleKeydown}
						placeholder="your.email@example.com"
						class="responsive-border"
					/>
				</div>

				{#if !loginMenu}
					<div class="form-group">
						<label for="username">Display Name</label>
						<input
							type="text"
							id="username"
							bind:value={username}
							required
							on:keydown={handleKeydown}
							placeholder="How others will see you"
							class="responsive-border"
						/>
					</div>
				{/if}

				<div class="form-group">
					<label for="password">Password</label>
					<input
						type="password"
						id="password"
						bind:value={password}
						required
						on:keydown={handleKeydown}
						class="responsive-border"
					/>
				</div>

				{#if errorMessageText}
					<p class="error fluid-text">{errorMessageText}</p>
				{/if}

				<button type="submit" class="submit-button responsive-shadow" disabled={loading}>
					{#if loading}
						<span class="loader"></span>
					{:else}
						{loginMenu ? 'Sign In' : 'Create Account'}
					{/if}
				</button>
			</form>

			<!-- Only show the toggle when appropriate -->
			{#if loginMenu}
				<p class="toggle-auth fluid-text">
					Don't have an account?
					<a href="/signup">Sign Up</a>
				</p>
			{:else}
				<p class="toggle-auth fluid-text">
					Already have an account?
					<a href="/login">Sign In</a>
				</p>
			{/if}
		</div>
	</div>
</div>

<style>
	.page-wrapper {
		min-height: 100vh;
		min-width: 100vw;
		background: var(--ui-bg-base);
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		overflow-y: auto;
	}

	.auth-container {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: clamp(1rem, 3vw, 2rem);
		padding-top: calc(60px + clamp(1rem, 3vw, 2rem));
		box-sizing: border-box;
	}

	.auth-card {
		background: var(--ui-bg-element-darker);
		border: 1px solid var(--ui-border);
		border-radius: clamp(8px, 1vw, 12px);
		width: 100%;
		margin: auto;
		max-width: 450px;
	}

	.auth-buttons-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		width: 100%;
		gap: clamp(1rem, 2vh, 1.5rem);
		margin-bottom: clamp(1rem, 2vh, 1.5rem);
	}

	h1 {
		color: var(--text-primary);
		text-align: center;
		margin-bottom: clamp(0.25rem, 1vh, 0.5rem);
		font-size: 1.5rem;
		font-weight: 600;
		text-transform: none;
		letter-spacing: normal;
	}

	.subtitle {
		color: var(--text-secondary);
		text-align: center;
		margin-bottom: clamp(1.5rem, 3vh, 2rem);
		font-size: 0.9rem;
	}

	.auth-form {
		display: flex;
		flex-direction: column;
		gap: clamp(1rem, 2vh, 1.25rem);
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: clamp(0.25rem, 0.5vh, 0.4rem);
	}

	label {
		color: var(--text-primary);
		font-size: 0.8rem;
		font-weight: 500;
	}

	input {
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-element-darker);
		color: var(--text-primary);
		padding: clamp(0.6rem, 1.2vh, 0.75rem);
		border-radius: clamp(4px, 0.5vw, 6px);
		font-size: 0.875rem;
	}

	input:focus {
		outline: none;
		border-color: var(--accent-color);
		box-shadow: 0 0 0 2px var(--accent-color-faded);
	}

	.submit-button {
		background: var(--accent-color);
		color: var(--text-on-accent, white);
		padding: clamp(0.75rem, 1.5vh, 0.9rem);
		border: none;
		border-radius: clamp(4px, 0.5vw, 6px);
		font-weight: 600;
		cursor: pointer;
		transition: all 0.2s ease;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.submit-button:hover:not(:disabled) {
		background: var(--accent-color-hover, #2563eb);
		transform: translateY(-1px);
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.2);
	}

	.submit-button:disabled {
		background: var(--accent-color-disabled, #3b82f6);
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error {
		color: var(--error-color, #ef4444);
		text-align: center;
		font-size: 0.8rem;
	}

	.toggle-auth {
		text-align: center;
		color: var(--text-secondary);
		margin-top: clamp(1.5rem, 3vh, 2rem);
		font-size: 0.85rem;
	}

	.toggle-auth a {
		color: var(--accent-color);
		font-weight: 500;
		text-decoration: none;
		transition: color 0.2s ease;
	}

	.toggle-auth a:hover {
		color: var(--accent-color-hover, #2563eb);
		text-decoration: underline;
	}

	.gsi-material-button {
		-moz-user-select: none;
		-webkit-user-select: none;
		-ms-user-select: none;
		-webkit-appearance: none;
		background-color: var(--ui-bg-element-darker, #131314);
		background-image: none;
		border: 1px solid var(--ui-border, #747775);
		-webkit-border-radius: 4px;
		border-radius: clamp(4px, 0.5vw, 6px);
		-webkit-box-sizing: border-box;
		box-sizing: border-box;
		color: var(--text-primary, #e3e3e3);
		cursor: pointer;
		font-family: 'Roboto', arial, sans-serif;
		height: clamp(36px, 5vh, 40px);
		letter-spacing: 0.25px;
		outline: none;
		overflow: hidden;
		padding: 0 clamp(8px, 1vw, 12px);
		position: relative;
		text-align: center;
		-webkit-transition:
			background-color 0.218s,
			border-color 0.218s,
			box-shadow 0.218s;
		transition:
			background-color 0.218s,
			border-color 0.218s,
			box-shadow 0.218s;
		vertical-align: middle;
		white-space: nowrap;
		width: 100%;
		max-width: 300px;
		min-width: min-content;
	}

	.gsi-material-button .gsi-material-button-icon {
		height: clamp(16px, 2vw, 20px);
		margin-right: clamp(8px, 1vw, 12px);
		min-width: clamp(16px, 2vw, 20px);
		width: clamp(16px, 2vw, 20px);
	}

	.gsi-material-button .gsi-material-button-content-wrapper {
		-webkit-align-items: center;
		align-items: center;
		display: flex;
		-webkit-flex-direction: row;
		flex-direction: row;
		-webkit-flex-wrap: nowrap;
		flex-wrap: nowrap;
		height: 100%;
		justify-content: space-between;
		position: relative;
		width: 100%;
	}

	.gsi-material-button .gsi-material-button-contents {
		-webkit-flex-grow: 1;
		flex-grow: 1;
		font-family: 'Roboto', arial, sans-serif;
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		vertical-align: top;
	}

	.gsi-material-button .gsi-material-button-state {
		-webkit-transition: opacity 0.218s;
		transition: opacity 0.218s;
		bottom: 0;
		left: 0;
		opacity: 0;
		position: absolute;
		right: 0;
		top: 0;
	}

	.gsi-material-button:disabled {
		cursor: default;
		background-color: rgba(var(--rgb-bg-element-darker), 0.6);
		border-color: rgba(var(--rgb-border), 0.4);
	}

	.gsi-material-button:disabled .gsi-material-button-state {
		background-color: rgba(var(--rgb-text-primary), 0.1);
	}

	.gsi-material-button:disabled .gsi-material-button-contents {
		opacity: 0.4;
	}

	.gsi-material-button:disabled .gsi-material-button-icon {
		opacity: 0.4;
	}

	.gsi-material-button:not(:disabled):active .gsi-material-button-state,
	.gsi-material-button:not(:disabled):focus .gsi-material-button-state {
		background-color: var(--text-primary);
		opacity: 0.12;
	}

	.gsi-material-button:not(:disabled):hover {
		background-color: var(--ui-bg-element-hover);
		border-color: var(--ui-border-hover);
		-webkit-box-shadow:
			0 1px 2px 0 rgba(0, 0, 0, 0.1),
			0 1px 3px 1px rgba(0, 0, 0, 0.08);
		box-shadow:
			0 1px 2px 0 rgba(0, 0, 0, 0.1),
			0 1px 3px 1px rgba(0, 0, 0, 0.08);
	}

	.gsi-material-button:not(:disabled):hover .gsi-material-button-state {
		background-color: var(--text-primary);
		opacity: 0.08;
	}

	.divider {
		display: flex;
		align-items: center;
		text-align: center;
		margin: clamp(1rem, 2vh, 1.5rem) 0;
	}

	.divider::before,
	.divider::after {
		content: '';
		flex: 1;
		border-bottom: 1px solid var(--ui-border);
	}

	.divider span {
		padding: 0 clamp(0.5rem, 1vw, 1rem);
		color: var(--text-secondary);
		font-size: 0.75rem;
		text-transform: uppercase;
	}

	.loader {
		width: clamp(16px, 2vw, 18px);
		height: clamp(16px, 2vw, 18px);
		border: 2px solid rgba(var(--rgb-text-on-accent, 255, 255, 255), 0.7);
		border-bottom-color: transparent;
		border-radius: 50%;
		display: inline-block;
		animation: rotation 1s linear infinite;
	}

	@keyframes rotation {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}
</style>
