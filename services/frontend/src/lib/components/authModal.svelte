<!-- authModal.svelte -->
<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Auth from './auth.svelte';

	const dispatch = createEventDispatcher();

	// Props
	export let visible: boolean = false;
	export let defaultMode: 'login' | 'signup' = 'login';

	// Internal state to track current mode
	let currentMode: 'login' | 'signup' = defaultMode;

	// Reset mode when modal becomes visible
	$: if (visible) {
		currentMode = defaultMode;
	}

	function toggleMode() {
		currentMode = currentMode === 'login' ? 'signup' : 'login';
	}

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

		if (typeof window !== 'undefined') {
			window.location.href = '/app';
		}
	}
</script>

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
						<path
							d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z"
							fill="currentColor"
						/>
					</svg>
				</button>

				<!-- Feature requirement header -->
				<div class="feature-header">
					<h2 id="auth-modal-title">
						{currentMode === 'login' ? 'Login to Peripheral' : 'Sign Up for Peripheral'}
					</h2>
				</div>

				<!-- Auth component -->
				<div class="auth-wrapper">
					<Auth
						loginMenu={currentMode === 'login'}
						modalMode={true}
						on:authSuccess={handleAuthSuccess}
						on:toggleMode={toggleMode}
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
		background: rgba(0, 0, 0, 0.75);
		backdrop-filter: blur(8px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 10000;
		padding: 1.5rem;
		animation: fadeIn 0.2s ease-out;
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}

	@keyframes slideUp {
		from {
			opacity: 0;
			transform: translateY(20px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.auth-modal-container {
		width: 100%;
		max-width: 380px;
		max-height: 90vh;
		overflow-y: auto;
		position: relative;
		margin: 0 auto;
		animation: slideUp 0.3s ease-out;
	}

	.auth-modal-content {
		background: #0f0f0f;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 16px;
		position: relative;
		width: 100%;
		overflow: hidden;
		box-shadow:
			0 20px 25px -5px rgba(0, 0, 0, 0.3),
			0 10px 10px -5px rgba(0, 0, 0, 0.2);
	}

	.close-button {
		position: absolute;
		top: 1rem;
		right: 1rem;
		background: rgba(255, 255, 255, 0.05);
		border: none;
		color: rgba(255, 255, 255, 0.4);
		cursor: pointer;
		padding: 0.5rem;
		border-radius: 50%;
		transition: all 0.2s ease;
		z-index: 1001;
		backdrop-filter: blur(4px);
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		opacity: 0.7;
	}

	.close-button:hover {
		background: rgba(255, 255, 255, 0.1);
		color: rgba(255, 255, 255, 0.8);
		opacity: 1;
		transform: scale(1.02);
	}

	.feature-header {
		background: linear-gradient(135deg, var(--ui-bg-secondary) 0%, var(--ui-bg-primary) 100%);
		padding: 2rem 2rem 1.5rem 2rem;
		text-align: center;
		position: relative;
	}

	.feature-header::after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 50%;
		transform: translateX(-50%);
		width: 60px;
		height: 1px;
		background: linear-gradient(90deg, transparent, var(--accent-color), transparent);
	}

	.feature-header h2 {
		color: #ffffff;
		font-size: 1.5rem;
		font-weight: 700;
		margin: 0;
		letter-spacing: -0.02em;
		text-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
		font-family: 'Inter', sans-serif;
		transition: all 0.2s ease;
	}

	.auth-wrapper {
		transition: opacity 0.2s ease;
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
		padding: 2rem;
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

	/* Minimal overrides - base styles are now standardized */
	.auth-wrapper :global(.error) {
		color: #ef4444;
		font-family: 'Inter', sans-serif;
	}

	/* Responsive adjustments */
	@media (max-width: 480px) {
		.auth-modal-overlay {
			padding: 1rem;
		}

		.feature-header {
			padding: 1.5rem 1.5rem 1rem 1.5rem;
		}

		.feature-header h2 {
			font-size: 1.25rem;
		}

		.auth-wrapper :global(.auth-card) {
			padding: 1.5rem;
		}
	}
</style>
