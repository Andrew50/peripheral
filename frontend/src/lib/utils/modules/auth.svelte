<!-- account.svelte -->
<script lang="ts">
	import { publicRequest } from '$lib/core/backend';
	import '$lib/core/global.css';

	import Header from '$lib/utils/modules/header.svelte';
	import { goto } from '$app/navigation';
	import { writable } from 'svelte/store';

	export let loginMenu: boolean = false;
	let username = '';
	let password = '';
	let errorMessage = writable('');

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
		publicRequest<Login>('login', { username: username, password: password })
			.then((r: Login) => {
				sessionStorage.setItem('authToken', r.token);
				sessionStorage.setItem('profilePic', r.profilePic);
				sessionStorage.setItem('username', r.username);
				goto('/app');
			})
			.catch((error: string) => {
				errorMessage.set(error);
			});
	}

	async function signUp(username: string, password: string) {
		try {
			await publicRequest('signup', { username: username, password: password });
			await signIn(username, password); // Automatically sign in after account creation
		} catch {
			errorMessage.set('Failed to create account');
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

<div class="page">
	<Header />
	<main>
		<div class="center-container">
			<button class="google-btn" on:click={handleGoogleLogin}>
				<img src="/google-icon.svg" alt="Google" />
				Sign in with Google
			</button>

			<div class="divider">
				<span>or</span>
			</div>

			<input autofocus placeholder="Username" bind:value={username} on:keydown={handleKeydown} />
			<input
				type="password"
				placeholder="Password"
				bind:value={password}
				on:keydown={handleKeydown}
			/>
			{#if loginMenu}
				<button on:click={() => signIn(username, password)}>Sign In</button>
			{:else}
				<button on:click={() => signUp(username, password)} class="color2">Create Account</button>
			{/if}
			<p class="error-message">{$errorMessage}</p>
		</div>
	</main>
</div>

<style>
	.google-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		background: white;
		color: #757575;
		border: 1px solid #ddd;
		padding: 10px;
		border-radius: 4px;
		cursor: pointer;
		width: 100%;
	}

	.google-btn img {
		width: 18px;
		height: 18px;
	}

	.divider {
		display: flex;
		align-items: center;
		text-align: center;
		margin: 20px 0;
	}

	.divider::before,
	.divider::after {
		content: '';
		flex: 1;
		border-bottom: 1px solid #ddd;
	}

	.divider span {
		padding: 0 10px;
		color: #757575;
		font-size: 12px;
	}

	button {
		width: 100%;
	}
</style>
