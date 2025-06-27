<script lang="ts">
	import type { Instance } from '$lib/utils/types/types';
	import { queryChart } from '$lib/features/chart/interface';
	import { createEventDispatcher, tick, onMount, onDestroy } from 'svelte';
	import { browser } from '$app/environment';

	export let instance: Instance;
	export let visible: boolean = false;

	const dispatch = createEventDispatcher();
	let toggleButton: HTMLButtonElement;
	let regularButton: HTMLButtonElement;
	let toggleContainer: HTMLDivElement;
	let isDocumentListenerActive = false;
	let isClosing = false;

	function setRegular() {
		if (instance && instance.extendedHours) {
			const updatedInstance = { ...instance, extendedHours: false };
			// Start animation immediately for responsive UI
			setTimeout(() => {
				isClosing = true;
			}, 350); // Start fade-out slightly before slide completes
			// Actually close after fade-out and dispatch change
			setTimeout(() => {
				dispatch('change');
				dispatch('close');
			}, 600); // Total time: slide (300ms) + fade (250ms) + buffer (50ms)
			// Make server call asynchronously to avoid blocking animation
			setTimeout(() => {
				queryChart(updatedInstance, true);
			}, 0);
		}
	}

	function setExtended() {
		if (instance && !instance.extendedHours) {
			const updatedInstance = { ...instance, extendedHours: true };
			// Start animation immediately for responsive UI
			setTimeout(() => {
				isClosing = true;
			}, 350); // Start fade-out slightly before slide completes
			// Actually close after fade-out and dispatch change
			setTimeout(() => {
				dispatch('change');
				dispatch('close');
			}, 600); // Total time: slide (300ms) + fade (250ms) + buffer (50ms)
			// Make server call asynchronously to avoid blocking animation
			setTimeout(() => {
				queryChart(updatedInstance, true);
			}, 0);
		}
	}

	function handleKeyDown(event: KeyboardEvent) {
		// Always stop propagation to prevent other handlers from interfering
		event.stopPropagation();

		if (event.key === 'Tab' || event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			const updatedInstance = { ...instance, extendedHours: !instance.extendedHours };
			// Start animation immediately for responsive UI
			setTimeout(() => {
				isClosing = true;
			}, 350);
			// Actually close after fade-out and dispatch change
			setTimeout(() => {
				dispatch('change');
				dispatch('close');
			}, 600);
			// Make server call asynchronously to avoid blocking animation
			setTimeout(() => {
				queryChart(updatedInstance, true);
			}, 0);
		} else if (event.key === 'Escape') {
			event.preventDefault();
			dispatch('close');
		}
	}

	function handleOverlayClick() {
		dispatch('close');
	}

	function handleClickOutside(event: MouseEvent) {
		if (!visible || !toggleContainer) return;

		const target = event.target as Node;
		if (toggleContainer && !toggleContainer.contains(target)) {
			dispatch('close');
		}
	}

	function addDocumentListener() {
		if (!isDocumentListenerActive && browser) {
			document.removeEventListener('click', handleClickOutside); // Remove any existing
			document.addEventListener('click', handleClickOutside);
			isDocumentListenerActive = true;
		}
	}

	function removeDocumentListener() {
		if (isDocumentListenerActive && browser) {
			document.removeEventListener('click', handleClickOutside);
			isDocumentListenerActive = false;
		}
	}

	// Auto-focus the currently active button when it becomes visible
	$: if (visible && toggleButton && regularButton) {
		isClosing = false; // Reset closing state when becoming visible
		tick().then(() => {
			if (instance?.extendedHours) {
				toggleButton.focus(); // Focus Extended button
			} else {
				regularButton.focus(); // Focus Regular button
			}
		});
	}

	// Add/remove click outside listener when visibility changes
	$: if (visible) {
		// Add listener after a short delay to prevent immediate closure
		setTimeout(() => {
			addDocumentListener();
		}, 100);
	} else {
		removeDocumentListener();
	}

	onDestroy(() => {
		removeDocumentListener();
	});
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
<!-- svelte-ignore a11y-no-static-element-interactions	-->
{#if visible}
	<div
		class="extended-hours-overlay"
		class:closing={isClosing}
		on:click={handleOverlayClick}
		on:keydown={handleKeyDown}
		role="dialog"
		aria-label="Extended Hours Toggle"
		tabindex="-1"
	>
		<div bind:this={toggleContainer} class="extended-hours-toggle" on:click|stopPropagation>
			<div class="segmented-control glass glass--rounded glass--responsive">
				<div class="sliding-indicator" class:extended={instance?.extendedHours}></div>
				<button
					bind:this={regularButton}
					class="segment-button regular"
					class:active={!instance?.extendedHours}
					on:click|stopPropagation={setRegular}
					on:keydown|stopPropagation={handleKeyDown}
					aria-label="Set to regular hours"
					aria-pressed={!instance?.extendedHours ? 'true' : 'false'}
				>
					Regular
				</button>
				<button
					bind:this={toggleButton}
					class="segment-button extended"
					class:active={instance?.extendedHours}
					on:click|stopPropagation={setExtended}
					on:keydown|stopPropagation={handleKeyDown}
					aria-label="Set to extended hours"
					aria-pressed={instance?.extendedHours ? 'true' : 'false'}
				>
					Extended
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.extended-hours-overlay {
		position: fixed;
		top: 20px;
		left: 50%;
		transform: translateX(-50%);
		background: transparent;
		z-index: 99999;
		opacity: 0;
		animation: slideInFromTop 0.2s ease-out forwards;
	}

	.extended-hours-overlay.closing {
		animation: slideOutToTop 0.25s ease-in forwards;
	}

	@keyframes slideInFromTop {
		from {
			opacity: 0;
			transform: translateX(-50%) translateY(-20px);
		}
		to {
			opacity: 1;
			transform: translateX(-50%) translateY(0);
		}
	}

	@keyframes slideOutToTop {
		from {
			opacity: 1;
			transform: translateX(-50%) translateY(0);
		}
		to {
			opacity: 0;
			transform: translateX(-50%) translateY(-20px);
		}
	}

	.extended-hours-toggle {
		position: relative;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		animation: scaleIn 0.2s ease-out;
	}

	@keyframes scaleIn {
		from {
			transform: scale(0.9);
			opacity: 0;
		}
		to {
			transform: scale(1);
			opacity: 1;
		}
	}

	.segmented-control {
		/* Glass effect now provided by global .glass classes */
		position: relative;
		display: flex;
		padding: 4px;
		width: 200px;
		height: 40px;
	}

	.sliding-indicator {
		position: absolute;
		top: 4px;
		left: 4px;
		width: calc(50% - 4px);
		height: calc(100% - 8px);
		background: rgba(255, 255, 255, 0.2);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 6px;
		transition: transform 0.3s ease;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
		backdrop-filter: blur(8px);
		z-index: 1;
	}

	.sliding-indicator.extended {
		transform: translateX(100%);
	}

	.segment-button {
		flex: 1;
		position: relative;
		z-index: 2;
		background: transparent;
		border: none;
		color: rgba(255, 255, 255, 0.7);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: color 0.3s ease;
		border-radius: 6px;
		display: flex;
		align-items: center;
		justify-content: center;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		user-select: none;
		outline: none;
	}

	.segment-button.active {
		color: rgba(255, 255, 255, 1);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 1);
	}

	.segment-button:hover:not(.active) {
		color: rgba(255, 255, 255, 0.9);
	}

	@media (max-width: 768px) {
		.extended-hours-overlay {
			top: 15px;
		}

		.segmented-control {
			width: 180px;
			height: 36px;
		}

		.segment-button {
			font-size: 12px;
		}
	}
</style>
