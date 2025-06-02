<script lang="ts">
	import Header from '$lib/components/header.svelte';
	import '$lib/styles/global.css';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';

	if (browser) {
		document.title = 'Atlantis';
	}

	let email = '';
	let isSubmitting = false;
	let isSubmitted = false;
	let errorMessage = '';

	async function handleSubmit(e: Event) {
		e.preventDefault();
		isSubmitting = true;
		errorMessage = '';

		try {
			const response = await fetch('/api/waitlist', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({ email })
			});

			const data = await response.json();

			if (response.ok) {
				isSubmitted = true;
				email = '';
			} else if (response.status === 409) {
				errorMessage = 'This email is already on the waitlist!';
			} else {
				errorMessage = data.error || 'Something went wrong. Please try again.';
			}
		} catch (error) {
			errorMessage = 'Network error. Please check your connection and try again.';
		} finally {
			isSubmitting = false;
		}
	}

	onMount(() => {
		// Add animation class after mount
		if (browser) {
			document.body.classList.add('loaded');
		}
	});
</script>

<main class="main-container">
	<div class="background-animation">
		<!-- Static gradient background -->
		<div class="static-gradient"></div>
	</div>

	<Header />
	
	<section class="waitlist-section">
		<div class="content-wrapper">
			<div class="badge">COMING SOON</div>
			
			<h1 class="title">
				<span class="gradient-text">Atlantis</span>
			</h1>
			
			<p class="subtitle">
				The new best way to trade.<br />
			</p>

			{#if !isSubmitted}
				<form class="waitlist-form" on:submit={handleSubmit}>
					<div class="form-group">
						<input
							type="email"
							bind:value={email}
							placeholder="Enter your email"
							required
							class="email-input"
							disabled={isSubmitting}
						/>
						<button 
							type="submit" 
							class="submit-button"
							disabled={isSubmitting || !email}
						>
							{isSubmitting ? 'Joining...' : 'Join Waitlist'}
						</button>
					</div>
					{#if errorMessage}
						<p class="error-message">{errorMessage}</p>
					{/if}
				</form>
			{:else}
				<div class="success-message">
					<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="success-icon">
						<path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
					</svg>
					<h3>You're on the list!</h3>
					<p>We'll notify you when Atlantis is ready to launch.</p>
				</div>
			{/if}

		</div>
	</section>
</main>

<style>
	@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');

	/* Scoped styles - no global body modifications */
	.main-container {
		position: relative;
		width: 100vw;
		height: 100vh;
		margin: 0;
		padding: 0;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		background: #0a0b0d;
		font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		color: #f9fafb;
	}

	/* Prevent scrolling on the body when this page is loaded */
	:global(body) {
		overflow: hidden !important;
		height: 100vh !important;
	}

	/* Background animation container */
	.background-animation {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		z-index: 0;
		overflow: hidden;
		background: #0a0b0d;
	}

	/* Static gradient background */
	.static-gradient {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background: 
			radial-gradient(circle at 20% 20%, rgba(59, 130, 246, 0.15) 0%, transparent 50%),
			radial-gradient(circle at 80% 80%, rgba(139, 92, 246, 0.1) 0%, transparent 50%),
			radial-gradient(circle at 40% 60%, rgba(16, 185, 129, 0.08) 0%, transparent 50%),
			linear-gradient(135deg, #0a0b0d 0%, #111827 50%, #0a0b0d 100%);
		z-index: 0;
		pointer-events: none;
	}

	/* Main content */
	.waitlist-section {
		position: relative;
		z-index: 10;
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;
		min-height: calc(100vh - 80px); /* Account for header */
		padding: 2rem;
	}

	.content-wrapper {
		max-width: 600px;
		width: 100%;
		text-align: center;
		animation: fadeInUp 1s ease-out;
		background: rgba(10, 11, 13, 0.6);
		backdrop-filter: blur(20px);
		border-radius: 2rem;
		padding: 3rem;
		border: 1px solid rgba(255, 255, 255, 0.1);
		box-shadow: 0 25px 50px rgba(0, 0, 0, 0.5);
	}

	@keyframes fadeInUp {
		from {
			opacity: 0;
			transform: translateY(30px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	.badge {
		display: inline-block;
		padding: 0.375rem 1rem;
		background: transparent;
		border: 1px solid rgba(59, 130, 246, 0.2);
		border-radius: 100px;
		font-size: 0.75rem;
		font-weight: 500;
		letter-spacing: 1.5px;
		color: #60a5fa;
		margin-bottom: 2rem;
		text-transform: uppercase;
	}

	.title {
		font-size: clamp(3rem, 8vw, 5rem);
		font-weight: 700;
		margin: 0 0 1rem 0;
		letter-spacing: -0.02em;
		line-height: 1.1;
	}

	.gradient-text {
		display: inline-block;
		font-size: clamp(3rem, 8vw, 5rem) !important;
		font-weight: 700 !important;
		background: linear-gradient(135deg, #3b82f6 0%, #6366f1 25%, #8b5cf6 50%, #ec4899 75%, #3b82f6 100%);
		background-size: 200% 200%;
		-webkit-background-clip: text;
		background-clip: text;
		-webkit-text-fill-color: transparent;
		animation: gradient-shift 8s ease infinite;
	}

	@keyframes gradient-shift {
		0%, 100% {
			background-position: 0% 50%;
		}
		25% {
			background-position: 100% 50%;
		}
		50% {
			background-position: 100% 100%;
		}
		75% {
			background-position: 0% 100%;
		}
	}

	.subtitle {
		font-size: clamp(1.1rem, 3vw, 1.4rem);
		color: #9ca3af;
		margin-bottom: 3rem;
		line-height: 1.6;
		font-weight: 300;
	}

	/* Form styles */
	.waitlist-form {
		margin-bottom: 3rem;
	}

	.form-group {
		display: flex;
		gap: 0.75rem;
		max-width: 450px;
		margin: 0 auto;
		flex-wrap: wrap;
		justify-content: center;
	}

	.email-input {
		flex: 1;
		min-width: 250px;
		padding: 0.875rem 1.25rem;
		background: rgba(255, 255, 255, 0.03);
		border: 1px solid rgba(255, 255, 255, 0.05);
		border-radius: 0.5rem;
		color: #fff;
		font-size: 0.95rem;
		transition: all 0.3s ease;
		backdrop-filter: blur(5px);
	}

	.email-input::placeholder {
		color: #6b7280;
		font-weight: 300;
	}

	.email-input:focus {
		outline: none;
		border-color: rgba(59, 130, 246, 0.5);
		background: rgba(255, 255, 255, 0.05);
		box-shadow: 0 0 0 1px rgba(59, 130, 246, 0.3);
	}

	.email-input:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.submit-button {
		padding: 0.875rem 2rem;
		background: #3b82f6;
		border: none;
		border-radius: 0.5rem;
		color: #fff;
		font-size: 0.95rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.3s ease;
		white-space: nowrap;
		position: relative;
		overflow: hidden;
	}

	.submit-button:before {
		content: '';
		position: absolute;
		top: 0;
		left: -100%;
		width: 100%;
		height: 100%;
		background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent);
		transition: left 0.5s ease;
	}

	.submit-button:hover:not(:disabled):before {
		left: 100%;
	}

	.submit-button:hover:not(:disabled) {
		background: #2563eb;
		transform: translateY(-1px);
		box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
	}

	.submit-button:disabled {
		opacity: 0.7;
		cursor: not-allowed;
		transform: none;
	}

	/* Success message */
	.success-message {
		background: rgba(34, 197, 94, 0.1);
		border: 1px solid rgba(34, 197, 94, 0.3);
		border-radius: 1rem;
		padding: 2rem;
		backdrop-filter: blur(10px);
		animation: fadeIn 0.5s ease-out;
	}

	.success-icon {
		width: 3rem;
		height: 3rem;
		color: #22c55e;
		margin-bottom: 1rem;
	}

	.success-message h3 {
		color: #22c55e;
		margin: 0 0 0.5rem 0;
		font-size: 1.5rem;
	}

	.success-message p {
		color: #9ca3af;
		margin: 0;
	}

	@keyframes fadeIn {
		from {
			opacity: 0;
			transform: scale(0.95);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	/* Error message */
	.error-message {
		color: #ef4444;
		margin-top: 1rem;
		font-size: 0.875rem;
	}

	/* Features preview */
	.features-preview {
		display: flex;
		gap: 2rem;
		justify-content: center;
		flex-wrap: wrap;
		margin-top: 4rem;
		padding-top: 2rem;
		border-top: 1px solid rgba(255, 255, 255, 0.1);
	}

	.feature-item {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		color: #9ca3af;
		font-size: 0.875rem;
		transition: color 0.3s ease;
	}

	.feature-item:hover {
		color: #60a5fa;
	}

	.feature-item svg {
		width: 1.25rem;
		height: 1.25rem;
		stroke: currentColor;
	}

	/* Mobile responsiveness */
	@media (max-width: 640px) {
		.waitlist-section {
			padding: 1rem;
		}

		.form-group {
			flex-direction: column;
			gap: 1rem;
		}

		.email-input {
			min-width: 100%;
		}

		.submit-button {
			width: 100%;
		}

		.features-preview {
			flex-direction: column;
			align-items: center;
			gap: 1rem;
		}

		.badge {
			font-size: 0.75rem;
			padding: 0.375rem 0.75rem;
		}

		.content-wrapper {
			padding: 2rem;
		}
	}
</style>
