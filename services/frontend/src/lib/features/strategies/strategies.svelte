<script lang="ts">
	import { onMount } from 'svelte';
	import { writable } from 'svelte/store';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import '$lib/styles/global.css';

	// Simple Strategy interface for the new prompt-based system
	interface Strategy {
		strategyId: number;
		name: string;
		description: string;
		prompt: string;
		pythonCode: string;
		score?: number;
		version?: string;
		createdAt?: string;
		isAlertActive?: boolean;
	}

	// Stores
	const loading = writable(false);
	const strategies = writable<Strategy[]>([]);
	const creating = writable(false);
	let newPrompt = '';

	// Load strategies on mount
	onMount(loadStrategies);

	async function loadStrategies() {
		loading.set(true);
		try {
			const data = await privateRequest<Strategy[]>('getStrategies', {});
			strategies.set(data || []);
		} catch (error) {
			console.error('Error loading strategies:', error);
			strategies.set([]);
		} finally {
			loading.set(false);
		}
	}

	async function createStrategy() {
		if (!newPrompt.trim()) {
			alert('Please enter a description of the pattern you want to find.');
			return;
		}

		creating.set(true);
		try {
			const result = await privateRequest<Strategy>('createStrategyFromPrompt', {
				query: newPrompt.trim(),
				strategyId: -1 // -1 for new strategy
			});

			// Add the new strategy to the list
			strategies.update((list) => [result, ...list]);
			newPrompt = '';
		} catch (error: any) {
			console.error('Error creating strategy:', error);
			alert(`Failed to create strategy: ${error.message || 'Unknown error'}`);
		} finally {
			creating.set(false);
		}
	}

	async function toggleAlert(strategyId: number, currentState: boolean) {
		try {
			await privateRequest('setAlert', {
				strategyId: strategyId,
				active: !currentState
			});

			// Update the local state
			strategies.update((list) =>
				list.map((s) => (s.strategyId === strategyId ? { ...s, isAlertActive: !currentState } : s))
			);
		} catch (error: any) {
			console.error('Error toggling alert:', error);
			alert(`Failed to update alert: ${error.message || 'Unknown error'}`);
		}
	}

	async function deleteStrategy(strategyId: number, name: string) {
		if (!confirm(`Are you sure you want to delete "${name}"?`)) return;

		try {
			await privateRequest('deleteStrategy', { strategyId });
			strategies.update((list) => list.filter((s) => s.strategyId !== strategyId));
		} catch (error: any) {
			console.error('Error deleting strategy:', error);
			alert(`Failed to delete strategy: ${error.message || 'Unknown error'}`);
		}
	}

	function formatDate(dateStr: string | undefined): string {
		if (!dateStr) return 'Unknown';
		try {
			return new Date(dateStr).toLocaleDateString();
		} catch {
			return 'Unknown';
		}
	}
</script>

<div class="strategies-container">
	<div class="header">
		<h2>AI Trading Strategies</h2>
		<p class="subtitle">Create intelligent pattern recognition strategies using natural language</p>
	</div>

	<!-- Create New Strategy -->
	<div class="create-section">
		<h3>Create New Strategy</h3>
		<div class="create-form">
			<textarea
				bind:value={newPrompt}
				placeholder="Describe the market pattern you want to find... 

Examples:
• Find stocks that are breaking out of consolidation with high volume
• Identify oversold stocks in the technology sector with RSI below 30
• Detect stocks with unusual options activity and positive earnings momentum
• Find dividend stocks trading near support levels with strong fundamentals"
				rows="4"
				disabled={$creating}
			></textarea>
			<button
				on:click={createStrategy}
				disabled={$creating || !newPrompt.trim()}
				class="create-btn"
			>
				{$creating ? 'Creating Strategy...' : 'Create Strategy'}
			</button>
		</div>
	</div>

	<!-- Strategies List -->
	<div class="strategies-list">
		<h3>Your Strategies ({$strategies.length})</h3>

		{#if $loading}
			<div class="loading">Loading strategies...</div>
		{:else if $strategies.length === 0}
			<div class="empty-state">
				<p>No strategies created yet.</p>
				<p>Create your first strategy above to get started!</p>
			</div>
		{:else}
			{#each $strategies as strategy (strategy.strategyId)}
				<div class="strategy-card">
					<div class="strategy-header">
						<h4 class="strategy-name">{strategy.name}</h4>
						<div class="strategy-actions">
							<button
								class="alert-btn {strategy.isAlertActive ? 'alert-active' : 'alert-inactive'}"
								on:click={() => toggleAlert(strategy.strategyId, strategy.isAlertActive || false)}
								title={strategy.isAlertActive ? 'Disable alerts' : 'Enable alerts'}
							>
								{strategy.isAlertActive ? '🔔 Alert ON' : '🔕 Alert OFF'}
							</button>
							<button
								class="delete-btn"
								on:click={() => deleteStrategy(strategy.strategyId, strategy.name)}
								title="Delete strategy"
							>
								🗑️
							</button>
						</div>
					</div>

					<div class="strategy-description">
						{strategy.description}
					</div>

					<div class="strategy-meta">
						<div class="meta-item">
							<span class="meta-label">Created:</span>
							<span class="meta-value">{formatDate(strategy.createdAt)}</span>
						</div>
						{#if strategy.score !== undefined}
							<div class="meta-item">
								<span class="meta-label">Score:</span>
								<span class="meta-value">{strategy.score}</span>
							</div>
						{/if}
						<div class="meta-item">
							<span class="meta-label">Version:</span>
							<span class="meta-value">{strategy.version || '1.0'}</span>
						</div>
					</div>

					<!-- Show original prompt if available -->
					{#if strategy.prompt && strategy.prompt !== strategy.description}
						<details class="strategy-details">
							<summary>View Original Request</summary>
							<div class="original-prompt">
								"{strategy.prompt}"
							</div>
						</details>
					{/if}
				</div>
			{/each}
		{/if}
	</div>
</div>

<style>
	.strategies-container {
		padding: 1rem;
		max-width: 1200px;
		margin: 0 auto;
		font-family:
			-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
	}

	.header {
		text-align: center;
		margin-bottom: 2rem;
	}

	.header h2 {
		margin: 0 0 0.5rem 0;
		color: var(--text-primary, #333);
		font-size: 2rem;
		font-weight: 600;
	}

	.subtitle {
		margin: 0;
		color: var(--text-secondary, #666);
		font-size: 1.1rem;
	}

	.create-section {
		background: var(--ui-bg-element, #fff);
		border: 1px solid var(--ui-border, #e0e0e0);
		border-radius: 8px;
		padding: 1.5rem;
		margin-bottom: 2rem;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.create-section h3 {
		margin: 0 0 1rem 0;
		color: var(--text-primary, #333);
		font-size: 1.3rem;
		font-weight: 600;
	}

	.create-form {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	textarea {
		width: 100%;
		min-height: 120px;
		padding: 1rem;
		border: 1px solid var(--ui-border, #ddd);
		border-radius: 6px;
		font-size: 0.95rem;
		line-height: 1.5;
		resize: vertical;
		font-family: inherit;
		background: var(--ui-bg-element, #fff);
		color: var(--text-primary, #333);
	}

	textarea:focus {
		outline: none;
		border-color: var(--accent-blue, #0066cc);
		box-shadow: 0 0 0 2px rgba(0, 102, 204, 0.2);
	}

	textarea:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.create-btn {
		align-self: flex-start;
		background: var(--accent-blue, #0066cc);
		color: white;
		border: none;
		padding: 0.75rem 1.5rem;
		border-radius: 6px;
		font-size: 1rem;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.create-btn:hover:not(:disabled) {
		background: var(--accent-blue-dark, #0052a3);
	}

	.create-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.strategies-list h3 {
		margin: 0 0 1rem 0;
		color: var(--text-primary, #333);
		font-size: 1.3rem;
		font-weight: 600;
	}

	.loading {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary, #666);
		font-size: 1.1rem;
	}

	.empty-state {
		text-align: center;
		padding: 3rem;
		color: var(--text-secondary, #666);
		background: var(--ui-bg-element, #fff);
		border: 1px solid var(--ui-border, #e0e0e0);
		border-radius: 8px;
	}

	.empty-state p {
		margin: 0.5rem 0;
		font-size: 1.1rem;
	}

	.strategy-card {
		background: var(--ui-bg-element, #fff);
		border: 1px solid var(--ui-border, #e0e0e0);
		border-radius: 8px;
		padding: 1.5rem;
		margin-bottom: 1rem;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
		transition: box-shadow 0.2s;
	}

	.strategy-card:hover {
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.15);
	}

	.strategy-header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		margin-bottom: 1rem;
		gap: 1rem;
	}

	.strategy-name {
		margin: 0;
		color: var(--text-primary, #333);
		font-size: 1.2rem;
		font-weight: 600;
		flex-grow: 1;
	}

	.strategy-actions {
		display: flex;
		gap: 0.5rem;
		flex-shrink: 0;
	}

	.alert-btn {
		padding: 0.5rem 1rem;
		border: none;
		border-radius: 6px;
		font-size: 0.9rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
	}

	.alert-btn.alert-active {
		background: var(--success-color, #22c55e);
		color: white;
	}

	.alert-btn.alert-active:hover {
		background: var(--success-color-dark, #16a34a);
	}

	.alert-btn.alert-inactive {
		background: var(--ui-bg-secondary, #f5f5f5);
		color: var(--text-secondary, #666);
		border: 1px solid var(--ui-border, #ddd);
	}

	.alert-btn.alert-inactive:hover {
		background: var(--ui-bg-hover, #e5e5e5);
	}

	.delete-btn {
		padding: 0.5rem;
		background: transparent;
		border: 1px solid var(--ui-border, #ddd);
		border-radius: 6px;
		cursor: pointer;
		transition: all 0.2s;
		font-size: 1rem;
	}

	.delete-btn:hover {
		background: var(--error-color-light, #fee2e2);
		border-color: var(--error-color, #ef4444);
	}

	.strategy-description {
		color: var(--text-primary, #333);
		font-size: 1rem;
		line-height: 1.6;
		margin-bottom: 1rem;
		background: var(--ui-bg-secondary, #f8f9fa);
		padding: 1rem;
		border-radius: 6px;
		border-left: 4px solid var(--accent-blue, #0066cc);
	}

	.strategy-meta {
		display: flex;
		gap: 2rem;
		flex-wrap: wrap;
	}

	.meta-item {
		display: flex;
		gap: 0.5rem;
		font-size: 0.9rem;
	}

	.meta-label {
		color: var(--text-secondary, #666);
		font-weight: 500;
	}

	.meta-value {
		color: var(--text-primary, #333);
	}

	.strategy-details {
		margin-top: 1rem;
		border-top: 1px solid var(--ui-border, #e0e0e0);
		padding-top: 1rem;
	}

	.strategy-details summary {
		cursor: pointer;
		color: var(--text-secondary, #666);
		font-size: 0.9rem;
		margin-bottom: 0.5rem;
	}

	.strategy-details summary:hover {
		color: var(--text-primary, #333);
	}

	.original-prompt {
		background: var(--ui-bg-secondary, #f8f9fa);
		padding: 0.75rem;
		border-radius: 4px;
		font-style: italic;
		color: var(--text-secondary, #666);
		border-left: 3px solid var(--ui-border, #ddd);
	}

	/* Responsive design */
	@media (max-width: 768px) {
		.strategies-container {
			padding: 0.5rem;
		}

		.strategy-header {
			flex-direction: column;
			align-items: stretch;
		}

		.strategy-actions {
			justify-content: flex-end;
			margin-top: 0.5rem;
		}

		.strategy-meta {
			flex-direction: column;
			gap: 0.5rem;
		}
	}
</style>
