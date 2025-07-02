<!-- auth.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/utils/helpers/backend';
	import '$lib/styles/global.css';
	import { browser } from '$app/environment';
	import type { LoginResponse } from '$lib/auth';
	import { setAuthCookies, setAuthSessionStorage } from '$lib/auth';

	import Header from '$lib/components/header.svelte';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';
	import { createEventDispatcher } from 'svelte';

	const dispatch = createEventDispatcher();

	export let loginMenu: boolean = false;
	export let modalMode: boolean = false;
	let email = '';
	let password = '';
	let errorMessage = writable('');
	let loading = false;

	// Update error message display
	let errorMessageText = '';
	errorMessage.subscribe((value) => {
		errorMessageText = value;
	});

	// Clear error message and reset form when switching between login/signup
	$: if (loginMenu !== undefined) {
		errorMessage.set('');
	}

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

	async function signIn(email: string, password: string) {
		loading = true;
		try {
			const r = await publicRequest<LoginResponse>('login', { email: email, password: password });
			if (browser) {
				// Set auth data using centralized utilities
				setAuthCookies(r.token, r.profilePic, r.username);
				setAuthSessionStorage(r.token, r.profilePic, r.username);
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

<div class="page-wrapper">
	<Header />
	<div class="auth-container">
		<div class="auth-card">
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
				<div class="form-group">
					<button class="gsi-material-button" on:click={handleGoogleLogin} type="button">
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
							<span class="gsi-material-button-contents">Continue with Google</span>
							<span style="display: none;">Sign in with Google</span>
						</div>
					</button>
				</div>

				<div class="form-group">
					<input
						type="email"
						id="email"
						bind:value={email}
						required
						on:keydown={handleKeydown}
						placeholder="Email"
					/>
				</div>

				<div class="form-group">
					<input
						type="password"
						id="password"
						bind:value={password}
						required
						on:keydown={handleKeydown}
						placeholder="Password"
					/>
				</div>

				{#if errorMessageText}
					<p class="error">{errorMessageText}</p>
				{/if}

				<div class="form-group">
					<button type="submit" class="submit-button" disabled={loading}>
						{#if loading}
							<span class="loader"></span>
						{:else}
							{loginMenu ? 'Sign In' : 'Create Account'}
						{/if}
					</button>
				</div>
			</form>

			<!-- Only show the toggle when appropriate -->
			{#if loginMenu}
				<p class="toggle-auth">
					Don't have an account?
					<a href="/signup" on:click={handleToggleMode}>Sign Up</a>
				</p>
			{:else}
				<p class="toggle-auth">
					Already have an account?
					<a href="/login" on:click={handleToggleMode}>Sign In</a>
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
		padding: clamp(1.5rem, 4vw, 2.5rem);
		box-shadow:
			0 10px 25px rgba(0, 0, 0, 0.2),
			0 4px 10px rgba(0, 0, 0, 0.1);
	}

	.auth-form {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		width: 100%;
		align-items: stretch;
		margin: 0;
		padding: 0;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		width: 100%;
		align-items: stretch;
	}

	input {
		border: 1px solid rgba(255, 255, 255, 0.1);
		background: rgba(255, 255, 255, 0.05);
		color: #ffffff;
		padding: 0 1rem;
		border-radius: 8px;
		font-size: 0.95rem;
		font-family: 'Inter', sans-serif;
		height: 48px;
		width: 100%;
		box-sizing: border-box;
		transition: all 0.2s ease;
		margin: 0;
		display: block;
	}

	input::placeholder {
		color: rgba(255, 255, 255, 0.5);
	}

	input:focus {
		outline: none;
		border-color: #3b82f6;
		background: rgba(255, 255, 255, 0.08);
		box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
	}

	.submit-button {
		background: #3b82f6;
		color: #ffffff;
		padding: 0;
		border: none;
		border-radius: 8px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 0.2s ease;
		font-family: 'Inter', sans-serif;
		height: 48px;
		width: 100%;
		box-sizing: border-box;
		font-size: 0.95rem;
		margin: 0;
		display: block;
	}

	.submit-button:hover:not(:disabled) {
		background: #2563eb;
	}

	.submit-button:active {
		background: #1d4ed8;
	}

	.submit-button:disabled {
		background: #3b82f6;
		opacity: 0.6;
		cursor: not-allowed;
	}

	.error {
		color: #ef4444;
		text-align: center;
		font-size: 0.875rem;
		font-family: 'Inter', sans-serif;
		margin: 0.5rem 0;
	}

	.toggle-auth {
		text-align: center;
		color: rgba(255, 255, 255, 0.7);
		margin-top: 1.5rem;
		padding-top: 1.5rem;
		border-top: 1px solid rgba(255, 255, 255, 0.1);
		font-size: 0.875rem;
		font-family: 'Inter', sans-serif;
	}

	.toggle-auth a {
		color: #3b82f6;
		font-weight: 600;
		text-decoration: none;
		transition: color 0.2s ease;
	}

	.toggle-auth a:hover {
		color: #2563eb;
		text-decoration: underline;
	}

	.gsi-material-button {
		-moz-user-select: none;
		-webkit-user-select: none;
		-ms-user-select: none;
		-webkit-appearance: none;
		background: rgba(255, 255, 255, 0.05);
		background-image: none;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 8px;
		box-sizing: border-box;
		color: #ffffff;
		cursor: pointer;
		font-family: 'Inter', sans-serif;
		height: 48px;
		letter-spacing: normal;
		outline: none;
		overflow: hidden;
		padding: 0;
		position: relative;
		text-align: center;
		transition: all 0.2s ease;
		vertical-align: middle;
		white-space: nowrap;
		width: 100%;
		max-width: none;
		min-width: auto;
		display: flex;
		align-items: center;
		justify-content: center;
		margin: 0;
	}

	.gsi-material-button .gsi-material-button-icon {
		height: 20px;
		margin-right: 8px;
		min-width: 20px;
		width: 20px;
	}

	.gsi-material-button .gsi-material-button-content-wrapper {
		align-items: center;
		display: flex;
		flex-direction: row;
		flex-wrap: nowrap;
		height: 100%;
		justify-content: center;
		position: relative;
		width: auto;
	}

	.gsi-material-button .gsi-material-button-contents {
		flex-grow: 0;
		font-family: 'Inter', sans-serif;
		font-weight: 500;
		font-size: 0.95rem;
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
		background: rgba(255, 255, 255, 0.1);
		border-color: rgba(255, 255, 255, 0.2);
	}

	.gsi-material-button:not(:disabled):hover .gsi-material-button-state {
		background-color: var(--text-primary);
		opacity: 0.08;
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

	/* Mobile responsiveness */
	@media (max-width: 480px) {
		.auth-container {
			padding: 1rem;
			padding-top: calc(60px + 1rem);
		}

		.auth-card {
			padding: 1.5rem;
			border-radius: 8px;
		}

		.gsi-material-button .gsi-material-button-contents {
			font-size: 0.9rem;
		}

		input,
		.submit-button {
			height: 44px;
			font-size: 0.9rem;
		}

		.toggle-auth {
			margin-top: 1rem;
			padding-top: 1rem;
		}
	}
</style>
