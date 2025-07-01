<script lang="ts">
	import { generateSharedConversationLink } from '../chatHelpers';
	import { browser } from '$app/environment';
	import { onDestroy } from 'svelte';

	// Props from parent
	export let currentConversationId: string;
	export let sharedConversationId: string = '';
	export let isPublicViewing: boolean = false;

	// Internal state
	let showModal = false;
	let shareLink = '';
	let shareLoading = false;
	let shareCopied = false;
	let shareCopyTimeout: ReturnType<typeof setTimeout> | null = null;
	let shareModalRef: HTMLDivElement;

	// Expose function to open modal
	export function openModal() {
		const conversationIdToShare = currentConversationId || sharedConversationId;
		if (!conversationIdToShare) {
			console.error('No active conversation to share');
			return;
		}

		showModal = true;
		shareLoading = true;
		shareLink = '';
		shareCopied = false;

		generateShareLink(conversationIdToShare);
	}

	// Expose function to toggle modal
	export function toggleModal() {
		if (showModal) {
			// Modal is open, close it
			closeModal();
		} else {
			// Modal is closed, open it
			openModal();
		}
	}

	async function generateShareLink(conversationIdToShare: string) {
		try {
			// If we're in public viewing mode, we know the conversation is already public
			// so we can generate the link directly without calling the backend
			if (isPublicViewing && sharedConversationId) {
				shareLink = `${window.location.origin}/share/${sharedConversationId}`;
				console.log('Generated share link for public conversation:', shareLink);
			} else {
				// For authenticated users, we need to make the conversation public via backend
				const link = await generateSharedConversationLink(conversationIdToShare);
				if (link) {
					shareLink = link;
				} else {
					console.error('Failed to generate share link');
				}
			}
		} catch (error) {
			console.error('Error generating share link:', error);
		} finally {
			shareLoading = false;
		}
	}

	function closeModal() {
		showModal = false;
		shareLink = '';
		shareLoading = false;
		shareCopied = false;
		if (shareCopyTimeout) {
			clearTimeout(shareCopyTimeout);
			shareCopyTimeout = null;
		}
	}

	async function copyShareLink() {
		if (!shareLink) return;

		try {
			await navigator.clipboard.writeText(shareLink);
			shareCopied = true;

			if (shareCopyTimeout) {
				clearTimeout(shareCopyTimeout);
			}
			shareCopyTimeout = setTimeout(() => {
				shareCopied = false;
				shareCopyTimeout = null;
			}, 2000);
		} catch (error) {
			console.error('Failed to copy share link:', error);
		}
	}

	// Handle clicks outside modal
	function handleClickOutside(event: MouseEvent) {
		if (showModal && shareModalRef && !shareModalRef.contains(event.target as Node)) {
			closeModal();
		}
	}

	// Reactive statement for event listeners
	$: if (browser) {
		if (showModal) {
			// Add a small delay to prevent the opening click from immediately triggering close
			setTimeout(() => {
				document.addEventListener('click', handleClickOutside);
			}, 100);
		} else {
			document.removeEventListener('click', handleClickOutside);
		}
	}

	// Cleanup on destroy
	onDestroy(() => {
		if (shareCopyTimeout) {
			clearTimeout(shareCopyTimeout);
		}
		if (browser) {
			document.removeEventListener('click', handleClickOutside);
		}
	});
</script>

<!-- Share Modal -->
{#if showModal}
	<div class="share-modal-popup glass glass--rounded glass--responsive" bind:this={shareModalRef}>
		<div class="share-modal-header">
			<h4>Share Conversation</h4>
			<button class="close-btn" on:click={closeModal} aria-label="Close">
				<svg viewBox="0 0 24 24" width="16" height="16">
					<path
						d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z"
						fill="currentColor"
					/>
				</svg>
			</button>
		</div>

		<div class="share-modal-content">
			{#if shareLoading}
				<div class="share-loading">
					<p>Generating share link...</p>
				</div>
			{:else if shareLink}
				<div class="share-link-container">
					<p class="share-description">Anyone with this link can view this conversation.</p>

					<div class="share-link-field">
						<input type="text" value={shareLink} readonly class="share-link-input" />
						<button
							class="copy-link-btn glass glass--small glass--responsive {shareCopied
								? 'copied'
								: ''}"
							on:click={copyShareLink}
						>
							{#if shareCopied}
								<svg viewBox="0 0 24 24" width="14" height="14">
									<path
										d="M9,20.42L2.79,14.21L5.62,11.38L9,14.77L18.88,4.88L21.71,7.71L9,20.42Z"
										fill="currentColor"
									/>
								</svg>
								Copied!
							{:else}
								<svg viewBox="0 0 24 24" width="14" height="14">
									<path
										d="M19,21H8V7H19M19,5H8A2,2 0 0,0 6,7V21A2,2 0 0,0 8,23H19A2,2 0 0,0 21,21V7A2,2 0 0,0 19,5M16,1H4A2,2 0 0,0 2,3V17H4V3H16V1Z"
										fill="currentColor"
									/>
								</svg>
								Copy
							{/if}
						</button>
					</div>
				</div>
			{:else}
				<div class="share-error">
					<p>Failed to generate share link. Please try again.</p>
					<button
						class="retry-btn glass glass--small glass--responsive"
						on:click={() => generateShareLink(currentConversationId || sharedConversationId)}
					>
						Retry
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}
