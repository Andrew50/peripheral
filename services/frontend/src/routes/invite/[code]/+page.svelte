<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Auth from '$lib/components/auth.svelte';
	import { publicRequest } from '$lib/utils/helpers/backend';

	let inviteCode = '';
	let authComponent: Auth;
	let loading = true;
	let error = '';
	let validationError = '';

	onMount(async () => {
		inviteCode = $page.params.code;

		try {
			// Validate the invite code with the backend. It will throw if invalid.
			await publicRequest('validateInvite', { code: inviteCode });

			// Pre-fill the invite code in the auth component if it supports it
			if (authComponent) {
				authComponent.setInviteCode(inviteCode);
			}
		} catch (err) {
			if (typeof err === 'string') {
				validationError = err;
			} else if (err instanceof Error) {
				validationError = err.message;
			} else {
				validationError = 'Invalid invite code.';
			}
		} finally {
			loading = false;
		}
	});

	function handleAuthSuccess() {
		// Redirect to app after successful signup
		goto('/app');
	}

	function handleAuthError(event: CustomEvent) {
		error = event.detail.error;
	}
</script>

<svelte:head>
	<title>Join with Invite Code - Peripheral</title>
	<meta
		name="description"
		content="Join Peripheral with your exclusive invite code and start your free trial."
	/>
</svelte:head>

<main class="min-h-screen bg-gradient-to-b from-gray-50 to-white">
	<div class="container mx-auto px-4 py-8">
		<div class="max-w-md mx-auto">
			{#if loading}
				<div class="text-center">
					<div class="animate-spin rounded-full h-32 w-32 border-b-2 border-blue-600 mx-auto"></div>
					<p class="mt-4 text-gray-600">Loading invite...</p>
				</div>
			{:else}
				<div class="bg-white rounded-lg shadow-lg p-8">
					{#if validationError}
						<div class="text-center">
							<h2 class="text-2xl font-semibold text-red-600 mb-2">Invalid Invite Code</h2>
							<p class="text-gray-600">{validationError}</p>
						</div>
					{:else}
						{#if error}
							<div class="mb-4 p-4 bg-red-50 border border-red-200 rounded-md">
								<p class="text-red-700 text-sm">{error}</p>
							</div>
						{/if}
						<Auth
							bind:this={authComponent}
							mode="signup"
							{inviteCode}
							on:success={handleAuthSuccess}
							on:error={handleAuthError}
						/>

						<div class="mt-6 text-center">
							<p class="text-sm text-gray-500">
								Already have an account?
								<a href="/login" class="text-blue-600 hover:text-blue-500 font-medium">
									Sign in instead
								</a>
							</p>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</main>

<style>
	/* Custom styles for the invite page */
	main {
		background-image: radial-gradient(
				circle at 25% 25%,
				rgba(59, 130, 246, 0.1) 0%,
				transparent 50%
			),
			radial-gradient(circle at 75% 75%, rgba(168, 85, 247, 0.1) 0%, transparent 50%);
	}
</style>
