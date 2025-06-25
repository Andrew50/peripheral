<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import { mobileBannerStore } from '$lib/stores/mobileBanner';
	import { browser } from '$app/environment';

	let bannerVisible = false;
	let bannerElement: HTMLDivElement;

	// Storage key for dismissed banner
	const STORAGE_KEY = 'atlantis-mobile-banner-dismissed';

	onMount(() => {
		if (browser) {
			// Check if banner was previously dismissed
			const dismissed = localStorage.getItem(STORAGE_KEY);
			if (!dismissed) {
				// Show banner if on mobile and not previously dismissed
				const checkMobile = () => {
					return (
						window.innerWidth <= 768 ||
						/Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
							navigator.userAgent
						)
					);
				};

				if (checkMobile()) {
					bannerVisible = true;
					mobileBannerStore.set({ visible: true });
				}
			}
		}
	});

	// No need for body class management since banner is in normal document flow

	function dismissBanner() {
		bannerVisible = false;
		mobileBannerStore.set({ visible: false });

		if (browser) {
			// Store dismissal in localStorage
			localStorage.setItem(STORAGE_KEY, 'true');
		}
	}

	// Listen to store changes
	$: if ($mobileBannerStore) {
		bannerVisible = $mobileBannerStore.visible;
	}
</script>

{#if bannerVisible}
	<div
		bind:this={bannerElement}
		class="mobile-banner show-on-mobile"
		transition:fade={{ duration: 300 }}
		role="banner"
		aria-label="Mobile notification banner"
	>
		<div class="banner-content">
			<div class="banner-text">
				<svg class="banner-icon" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path
						d="M12 2L2 7L12 12L22 7L12 2Z"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
					<path
						d="M2 17L12 22L22 17"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
					<path
						d="M2 12L12 17L22 12"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
				<span class="banner-message"> For the best experience, try Atlantis on desktop. </span>
			</div>
			<button
				class="dismiss-button"
				on:click={dismissBanner}
				aria-label="Dismiss banner"
				title="Dismiss"
			>
				<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path
						d="M18 6L6 18"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
					<path
						d="M6 6L18 18"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
		</div>
	</div>
{/if}

<style>
	.mobile-banner {
		position: static;
		width: 100%;
		background: var(--ui-bg-primary);
		border-bottom: 1px solid var(--ui-border);
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
		padding: var(--space-sm) var(--space-md);
	}

	.banner-content {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-sm);
		max-width: 100%;
	}

	.banner-text {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		flex: 1;
		min-width: 0; /* Allow text to shrink */
	}

	.banner-icon {
		width: 20px;
		height: 20px;
		color: var(--text-secondary);
		flex-shrink: 0;
	}

	.banner-message {
		font-size: var(--font-sm);
		color: var(--text-primary);
		line-height: 1.4;
		flex: 1;
		min-width: 0;
	}

	.dismiss-button {
		background: transparent;
		border: none;
		color: var(--text-secondary);
		cursor: pointer;
		padding: var(--space-xs);
		border-radius: var(--radius-sm);
		display: flex;
		align-items: center;
		justify-content: center;
		transition: all 0.2s ease;
		flex-shrink: 0;
	}

	.dismiss-button:hover {
		background: var(--ui-bg-secondary);
		color: var(--text-primary);
	}

	.dismiss-button:focus {
		outline: 2px solid var(--ui-accent);
		outline-offset: 2px;
	}

	.dismiss-button svg {
		width: 16px;
		height: 16px;
	}

	/* Mobile-specific adjustments */
	@media (max-width: 640px) {
		.mobile-banner {
			padding: var(--space-xs) var(--space-sm);
		}

		.banner-message {
			font-size: var(--font-xs);
		}

		.banner-icon {
			width: 18px;
			height: 18px;
		}

		.dismiss-button svg {
			width: 14px;
			height: 14px;
		}
	}

	/* Ensure banner doesn't show on desktop */
	@media (min-width: 769px) {
		.mobile-banner {
			display: none !important;
		}
	}

	/* No global styles needed since banner is in normal document flow */
</style>
