<!-- authModal.svelte -->
<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Auth from './auth.svelte';

	const dispatch = createEventDispatcher();

	// Props
	export let visible: boolean = false;
	export let defaultMode: 'login' | 'signup' = 'login';
	export let requiredFeature: string = 'this feature'; // e.g., "watchlists", "strategies", etc.

	function closeModal() {
		dispatch('close');
	}

	function handleBackdropClick(event: MouseEvent) {
		if (event.target === event.currentTarget) {
			closeModal();
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			closeModal();
		}
	}

	// Handle successful authentication
	function handleAuthSuccess() {
		dispatch('success');
		closeModal();
		
		// Refresh the page to update auth state
		if (typeof window !== 'undefined') {
			window.location.reload();
		}
	}
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
{#if visible}
	<div 
		class="auth-modal-overlay"
		on:click={handleBackdropClick}
		on:keydown={handleKeydown}
		role="dialog"
		aria-modal="true"
		aria-labelledby="auth-modal-title"
	>
		<div class="auth-modal-container">
			<div class="auth-modal-content">
				<!-- Close button -->
				<button class="close-button" on:click={closeModal} aria-label="Close">
					<svg viewBox="0 0 24 24" width="24" height="24">
						<path d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z" fill="currentColor" />
					</svg>
				</button>

				<!-- Feature requirement header -->
				<div class="feature-header">
					<h2>Authentication Required</h2>
					<p>You need to sign in to access <strong>{requiredFeature}</strong></p>
				</div>

				<!-- Auth component -->
				<div class="auth-wrapper">
					<Auth 
						loginMenu={defaultMode === 'login'} 
						on:authSuccess={handleAuthSuccess}
					/>
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	.auth-modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.8);
		backdrop-filter: blur(1px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 10000;
		padding: 1rem;
	}

	.auth-modal-container {
		width: 100%;
		max-width: 500px;
		max-height: 90vh;
		overflow-y: auto;
		position: relative;
		margin: 0 auto;
	}

	.auth-modal-content {
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 12px;
		position: relative;
		width: 100%;
		overflow: hidden;
	}

	.close-button {
		position: absolute;
		top: 1rem;
		right: 1rem;
		background: rgba(0, 0, 0, 0.5);
		border: none;
		color: white;
		cursor: pointer;
		padding: 0.5rem;
		border-radius: 6px;
		transition: all 0.2s ease;
		z-index: 1001;
		backdrop-filter: blur(4px);
	}

	.close-button:hover {
		background: rgba(0, 0, 0, 0.7);
	}

	.feature-header {
		background: var(--ui-bg-secondary);
		padding: 1.5rem; /* Symmetric padding - close button is positioned absolutely */
		border-bottom: 1px solid var(--ui-border);
		text-align: center;
	}

	.feature-header h2 {
		color: var(--text-primary);
		font-size: 1.25rem;
		font-weight: 600;
		margin: 0 0 0.5rem 0;
	}

	.feature-header p {
		color: var(--text-secondary);
		font-size: 0.9rem;
		margin: 0;
	}

	.feature-header strong {
		color: var(--accent-color);
	}

	/* Override some auth.svelte styles to fit better in modal */
	.auth-wrapper :global(.page-wrapper) {
		min-height: auto;
		min-width: auto;
		background: transparent;
		position: static;
		overflow-y: visible;
	}

	.auth-wrapper :global(.auth-container) {
		min-height: auto;
		padding: 0;
		padding-top: 0;
	}

	.auth-wrapper :global(.auth-card) {
		border: none;
		border-radius: 0;
		background: transparent;
		max-width: none;
		margin: 0;
	}

	/* Hide the header component inside auth.svelte */
	.auth-wrapper :global(.page-wrapper > *:first-child) {
		display: none;
	}

	/* Responsive adjustments */
	@media (max-width: 480px) {
		.feature-header {
			padding: 1rem;
		}

		.feature-header h2 {
			font-size: 1.1rem;
		}

		.feature-header p {
			font-size: 0.85rem;
		}
	}
</style> 