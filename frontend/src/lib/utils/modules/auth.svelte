<!-- account.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/core/backend';
	import '$lib/core/global.css';
	import { browser } from '$app/environment';

	import Header from '$lib/utils/modules/header.svelte';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';

	export let loginMenu: boolean = false;
	let username = '';
	let password = '';
	let errorMessage = writable('');
	let loading = false;

	// Update error message display
	let errorMessageText = '';
	errorMessage.subscribe((value) => {
		errorMessageText = value;
	});

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			if (loginMenu) {
				signIn(username, password);
			} else {
				signUp(username, password);
			}
		}
	}

	interface Login {
		token: string;
		settings: string;
		profilePic: string;
		username: string;
	}

	async function signIn(username: string, password: string) {
		loading = true;
		try {
			const r = await publicRequest<Login>('login', { username: username, password: password });
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

	async function signUp(username: string, password: string) {
		loading = true;
		try {
			await publicRequest('signup', { username: username, password: password });
			await signIn(username, password);
		} catch {
			errorMessage.set('Failed to create account');
			loading = false;
		}
	}

	async function handleGoogleLogin() {
		try {
			const response = await publicRequest<{ url: string }>('googleLogin', {});
			window.location.href = response.url;
		} catch (error) {
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

			<button class="google-button" on:click={handleGoogleLogin}>
				<img src="/google-icon.svg" alt="Google" />
				<span>Continue with Google</span>
			</button>

			<div class="divider">
				<span>or continue with email</span>
			</div>

			<form
				on:submit|preventDefault={() => {
					if (loginMenu) {
						signIn(username, password);
					} else {
						signUp(username, password);
					}
				}}
				class="auth-form"
				on:keydown={handleKeydown}
			>
				<div class="form-group">
					<label for="username">Username</label>
					<input type="text" id="username" bind:value={username} required autofocus />
				</div>

				<div class="form-group">
					<label for="password">Password</label>
					<input type="password" id="password" bind:value={password} required />
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
		background: linear-gradient(to bottom, #000000, #1a1a2e);
	}

	.auth-container {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2rem;
		padding-top: calc(80px + 2rem);
	}

	.auth-card {
		background: rgba(255, 255, 255, 0.05);
		padding: 2rem;
		border-radius: 12px;
		width: 100%;
		max-width: 400px;
		backdrop-filter: blur(10px);
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

	.google-button {
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.75rem;
		padding: 0.75rem;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
		color: white;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.3s ease;
	}

	.google-button:hover {
		background: rgba(255, 255, 255, 0.15);
	}

	.google-button img {
		width: 24px;
		height: 24px;
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
