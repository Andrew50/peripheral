<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { publicRequest } from '$lib/core/backend';

	interface GoogleCallbackResponse {
		token: string;
		profilePic: string;
		username: string;
	}

	let errorMessage = '';

	onMount(async () => {
		const urlParams = new URLSearchParams(window.location.search);
		const code = urlParams.get('code');
		const state = urlParams.get('state');

		// Verify the state parameter matches what we stored
		const storedState = sessionStorage.getItem('googleAuthState');

		if (!code) {
			console.error('No authorization code received');
			errorMessage = 'Authorization failed: No code received';
			setTimeout(() => goto('/login'), 3000);
			return;
		}

		if (!state || state !== storedState) {
			console.error('State mismatch or missing state parameter');
			errorMessage = 'Authorization failed: Invalid state parameter';
			setTimeout(() => goto('/login'), 3000);
			return;
		}

		try {
			const response = await publicRequest<GoogleCallbackResponse>('googleCallback', {
				code,
				state
			});

			sessionStorage.setItem('authToken', response.token);
			sessionStorage.setItem('profilePic', response.profilePic || '');
			sessionStorage.setItem('username', response.username || '');

			// Log what was stored

			// Clean up stored state
			sessionStorage.removeItem('googleAuthState');

			goto('/app');
		} catch (error) {
			console.error('Google authentication failed:', error);
			errorMessage = typeof error === 'string' ? error : 'Authentication failed';
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
		<div class="loading">Authenticating...</div>
	{/if}
</div>

<style>
	.container {
		display: flex;
		justify-content: center;
		align-items: center;
		height: 100vh;
		font-size: 1.2em;
	}

	.loading {
		text-align: center;
	}

	.error {
		color: #e74c3c;
		text-align: center;
		max-width: 400px;
		padding: 20px;
		background-color: rgba(0, 0, 0, 0.05);
		border-radius: 8px;
	}
</style>
