<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { publicRequest } from '$lib/core/backend';

	interface GoogleCallbackResponse {
		token: string;
	}

	onMount(async () => {
		const urlParams = new URLSearchParams(window.location.search);
		const code = urlParams.get('code');
		const state = urlParams.get('state');

		if (code && state) {
			try {
				const response = await publicRequest<GoogleCallbackResponse>('googleCallback', {
					code,
					state
				});
				sessionStorage.setItem('authToken', response.token);
				goto('/app');
			} catch (error) {
				console.error('Google authentication failed:', error);
				goto('/login');
			}
		} else {
			goto('/login');
		}
	});
</script>

<div class="loading">Authenticating...</div>

<style>
	.loading {
		display: flex;
		justify-content: center;
		align-items: center;
		height: 100vh;
		font-size: 1.2em;
	}
</style>
