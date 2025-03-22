<script lang="ts" context="module">
	import { privateRequest } from '$lib/core/backend';
	import type { Instance } from '$lib/core/types';
	import { writable, type Writable } from 'svelte/store';

	// Create a store to hold the LLM summary data
	export interface LLMSummaryState {
		ticker: string;
		summary: string;
		loading: boolean;
		error: string | null;
	}

	export const llmSummaryStore: Writable<LLMSummaryState> = writable({
		ticker: '',
		summary: '',
		loading: false,
		error: null
	});

	// Function to open the LLM summary window
	export function openLLMSummary(): void {
		// Import dynamically to avoid circular dependencies
		import('$lib/core/stores').then(({ dispatchMenuChange }) => {
			// This will trigger the bottom window to open
			dispatchMenuChange.set('llm-summary');
		});
	}

	export function getLLMSummary(instance: Instance): void {
		const ticker = instance.ticker;
		const timestamp = instance.timestamp;
		const price = instance.price;

		// Update store to show loading state
		llmSummaryStore.set({
			ticker: ticker || '',
			summary: '',
			loading: true,
			error: null
		});

		// Open the summary window
		openLLMSummary();

		// Make the API request
		privateRequest<{ summary: string }>('getStockMovementSummary', {
			ticker,
			timestamp,
			price
		})
			.then((s) => {
				const summary = s.summary;
				console.log('LLM summary:', summary);

				// Update store with the result
				llmSummaryStore.update((state: LLMSummaryState) => ({
					...state,
					summary,
					loading: false
				}));
			})
			.catch((error: Error) => {
				console.error('Error fetching LLM summary:', error);

				// Update store with error
				llmSummaryStore.update((state: LLMSummaryState) => ({
					...state,
					loading: false,
					error: error.message || 'Failed to fetch summary'
				}));
			});
	}
</script>

<div class="llm-summary-container">
	{#if $llmSummaryStore.loading}
		<div class="loading">
			<div class="spinner"></div>
			<p>Fetching summary for {$llmSummaryStore.ticker}...</p>
		</div>
	{:else if $llmSummaryStore.error}
		<div class="error">
			<h3>Error</h3>
			<p>{$llmSummaryStore.error}</p>
		</div>
	{:else if $llmSummaryStore.summary}
		<div class="summary">
			<h3>Summary for {$llmSummaryStore.ticker}</h3>
			<div class="summary-content">
				{$llmSummaryStore.summary}
			</div>
		</div>
	{:else}
		<div class="empty">
			<p>No summary available</p>
		</div>
	{/if}
</div>

<style>
	.llm-summary-container {
		padding: 16px;
		height: 100%;
		overflow-y: auto;
	}

	.loading {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		height: 100%;
	}

	.spinner {
		width: 40px;
		height: 40px;
		border: 4px solid var(--c3);
		border-top: 4px solid var(--c1);
		border-radius: 50%;
		animation: spin 1s linear infinite;
		margin-bottom: 16px;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}

	.summary h3 {
		margin-top: 0;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--c3);
	}

	.summary-content {
		line-height: 1.5;
		white-space: pre-line;
	}

	.error {
		color: #e74c3c;
	}
</style>
