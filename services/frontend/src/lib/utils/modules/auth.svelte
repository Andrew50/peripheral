<!-- account.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/core/backend';
	import '$lib/core/global.css';
	import { browser } from '$app/environment';

	import Header from '$lib/utils/modules/header.svelte';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';

	export let loginMenu: boolean = false;
	let email = '';
	let username = '';
	let password = '';
	let errorMessage = writable('');
	let loading = false;

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
			const r = await publicRequest<Login>('login', { email: email, password: password });
			if (browser) {
				sessionStorage.setItem('authToken', r.token);
				sessionStorage.setItem('profilePic', r.profilePic);
				sessionStorage.setItem('username', r.username);
			}
			goto('/app');
		} catch (error) {
			if (error instanceof Error) {
				errorMessage.set(error.message);
			} else {
				errorMessage.set('Login failed. Please try again.');
			}
		} finally {
			loading = false;
		}
	}

	async function signUp(email: string, username: string, password: string) {
		loading = true;
		try {
			await publicRequest('signup', { email: email, username: username, password: password });
			await signIn(email, password);
		} catch (error) {
			if (error instanceof Error) {
				errorMessage.set(error.message);
			} else {
				errorMessage.set('Failed to create account');
			}
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
			<h1>{loginMenu ? 'Welcome Back' : 'Create Account'}</h1>
			<p class="subtitle">
				{loginMenu ? 'Sign in to access your account' : 'Start your trading journey today'}
			</p>

			<button class="gsi-material-button" on:click={handleGoogleLogin}>
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

			<div class="divider">
				<span>or continue with email</span>
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
						autofocus
						placeholder="your.email@example.com"
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
					/>
				</div>

				{#if errorMessageText}
					<p class="error">{errorMessageText}</p>
				{/if}

				<button type="submit" class="submit-button" disabled={loading}>
					{#if loading}
						<span class="loader"></span>
					{:else}
						{loginMenu ? 'Sign In' : 'Create Account'}
					{/if}
				</button>
			</form>

			<!-- Only show the toggle when appropriate -->
			{#if loginMenu}
				<p class="toggle-auth">
					Don't have an account?
					<a href="/signup">Sign Up</a>
				</p>
			{:else}
				<p class="toggle-auth">
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
		background: linear-gradient(to bottom, #000000, #1a1a2e);
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
		padding: 2rem;
		padding-top: calc(80px + 2rem);
		box-sizing: border-box;
	}

	.auth-card {
		background: rgba(255, 255, 255, 0.05);
		padding: 2rem;
		border-radius: 12px;
		width: 100%;
		max-width: 400px;
		backdrop-filter: blur(10px);
		margin: auto;
	}

	h1 {
		color: white;
		text-align: center;
		margin-bottom: 0.5rem;
	}

	.subtitle {
		color: #94a3b8;
		text-align: center;
		margin-bottom: 2rem;
	}

	.auth-form {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	label {
		color: #e2e8f0;
		font-size: 0.9rem;
	}

	input {
		padding: 0.75rem;
		border-radius: 6px;
		border: 1px solid rgba(255, 255, 255, 0.1);
		background: rgba(255, 255, 255, 0.05);
		color: white;
		font-size: 1rem;
	}

	input:focus {
		outline: none;
		border-color: #3b82f6;
	}

	.submit-button {
		background: #3b82f6;
		color: white;
		padding: 1rem;
		border: none;
		border-radius: 6px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.3s ease;
	}

	.submit-button:hover:not(:disabled) {
		background: #2563eb;
	}

	.submit-button:disabled {
		opacity: 0.7;
		cursor: not-allowed;
	}

	.error {
		color: #ef4444;
		text-align: center;
		font-size: 0.9rem;
	}

	.toggle-auth {
		text-align: center;
		color: #94a3b8;
		margin-top: 2rem;
	}

	.gsi-material-button {
		-moz-user-select: none;
		-webkit-user-select: none;
		-ms-user-select: none;
		-webkit-appearance: none;
		background-color: #131314;
		background-image: none;
		border: 1px solid #747775;
		-webkit-border-radius: 4px;
		border-radius: 4px;
		-webkit-box-sizing: border-box;
		box-sizing: border-box;
		color: #e3e3e3;
		cursor: pointer;
		font-family: 'Roboto', arial, sans-serif;
		font-size: 14px;
		height: 40px;
		letter-spacing: 0.25px;
		outline: none;
		overflow: hidden;
		padding: 0 12px;
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
		max-width: 400px;
		min-width: min-content;
		border-color: #8e918f;
	}

	.gsi-material-button .gsi-material-button-icon {
		height: 20px;
		margin-right: 12px;
		min-width: 20px;
		width: 20px;
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
		background-color: #13131461;
		border-color: #8e918f1f;
	}

	.gsi-material-button:disabled .gsi-material-button-state {
		background-color: #e3e3e31f;
	}

	.gsi-material-button:disabled .gsi-material-button-contents {
		opacity: 38%;
	}

	.gsi-material-button:disabled .gsi-material-button-icon {
		opacity: 38%;
	}

	.gsi-material-button:not(:disabled):active .gsi-material-button-state,
	.gsi-material-button:not(:disabled):focus .gsi-material-button-state {
		background-color: white;
		opacity: 12%;
	}

	.gsi-material-button:not(:disabled):hover {
		-webkit-box-shadow:
			0 1px 2px 0 rgba(60, 64, 67, 0.3),
			0 1px 3px 1px rgba(60, 64, 67, 0.15);
		box-shadow:
			0 1px 2px 0 rgba(60, 64, 67, 0.3),
			0 1px 3px 1px rgba(60, 64, 67, 0.15);
	}

	.gsi-material-button:not(:disabled):hover .gsi-material-button-state {
		background-color: white;
		opacity: 8%;
	}

	.divider {
		display: flex;
		align-items: center;
		text-align: center;
		margin: 1.5rem 0;
	}

	.divider::before,
	.divider::after {
		content: '';
		flex: 1;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
	}

	.divider span {
		padding: 0 1rem;
		color: #94a3b8;
		font-size: 0.875rem;
	}

	.loader {
		width: 20px;
		height: 20px;
		border: 2px solid #ffffff;
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
