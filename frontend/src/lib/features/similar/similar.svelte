<script lang="ts" context="module">
	import type { Instance } from '$lib/core/types';
	import { writable } from 'svelte/store';
	import type { Writable } from 'svelte/store';
	import { privateRequest } from '$lib/core/backend';
	import { queryChart } from '$lib/features/chart/interface';
	import { queryInstanceRightClick } from '$lib/utils/popups/rightClick.svelte';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { instance } from './interface';
	import List from '$lib/utils/modules/list.svelte';

	const similarList: Writable<Instance[]> = writable([]);

	const inactiveSimilarQuery = { status: 'inactive' as const, similarInstances: [] };

	// Watch for changes to similarQuery and fetch similar instances when activated
	instance.subscribe((query) => {
		console.log(query);
		if (query.ticker) {
			console.log('ticker');
			// Initialize similarInstances as empty array before fetching
			similarList.set([]);

			privateRequest<Instance[]>(
				'getSimilarInstances',
				{
					ticker: query.ticker,
					securityId: query.securityId,
					timeframe: query.timeframe,
					timestamp: query.timestamp
				},
				true
			).then((instances) => {
				if (instances) {
					console.log(instances);
					similarList.set(instances);
				}
			});
		}
	});
</script>

<script lang="ts">
	import '$lib/core/global.css';

	const columns = ['Ticker', 'Price', 'Chg%', 'Ext'];

	function handleAddTicker() {
		queryInstanceInput(['ticker'], ['ticker', 'timestamp'], $instance).then((ins: Instance) => {
			instance.set({
				...ins
			});
		});
	}

	function handleDelete(instance: Instance) {
		// Optional: Implement delete functionality if needed
		console.log('Delete:', instance);
	}
</script>

<div class="similar-container">
	<div class="header">
		<h3>Similar</h3>
	</div>
	<div class="content">
		<div class="base-instance">
			<button class="add-btn" on:click={handleAddTicker}>+ Add Ticker</button>
		</div>
		{#if $instance.ticker}
			<div class="base-instance">
				<span class="value">{$instance.ticker}</span>
			</div>
		{/if}

		{#if $similarList.length > 0}
			<List list={similarList} {columns} parentDelete={handleDelete} />
		{:else}
			<div class="no-results">No Similar Instances Found</div>
		{/if}
	</div>
</div>

<style>
	.similar-container {
		height: 100%;
		display: flex;
		flex-direction: column;
		background: var(--ui-bg-primary);
		color: var(--text-primary);
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 12px 16px;
		border-bottom: 1px solid var(--ui-border);
		background: var(--ui-bg-secondary);
	}

	.header h3 {
		margin: 0;
		font-size: 16px;
		font-weight: 600;
	}

	.content {
		flex: 1;
		overflow-y: auto;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.base-instance {
		padding: 12px;
		background: var(--ui-bg-secondary);
		border-radius: 6px;
		border: 1px solid var(--ui-border);
	}

	.label {
		color: var(--text-secondary);
		font-size: 12px;
		text-transform: uppercase;
		display: block;
		margin-bottom: 4px;
	}

	.value {
		font-size: 16px;
		font-weight: 500;
	}

	.similar-list {
		background: var(--ui-bg-secondary);
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		overflow: hidden;
	}

	.add-btn {
		width: 100%;
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		color: var(--text-primary);
		padding: 10px 16px;
		border-radius: 4px;
		cursor: pointer;
		font-size: 14px;
		font-weight: 500;
		transition: all 0.2s ease;
	}

	.add-btn:hover {
		background: var(--ui-bg-hover);
		transform: translateY(-1px);
	}
</style>
