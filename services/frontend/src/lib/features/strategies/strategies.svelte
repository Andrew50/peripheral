<script lang="ts">
	import { queueRequest, privateRequest } from '$lib/core/backend';
	import type { Strategy as CoreStrategy } from '$lib/core/types';
	import { strategies } from '$lib/core/stores';
	import '$lib/core/global.css';
	//import Trainer from './trainer.svelte';
	import { onMount } from 'svelte';
	import type { Subscriber, Updater } from 'svelte/store';
	import { eventDispatcher } from './interface';
	import type { StrategyEvent } from './interface';

	// Extend the core Strategy type to include the 'new' state
	type StrategyId = number | 'new' | null;
	interface EditableStrategy extends Omit<CoreStrategy, 'strategyId' | 'activeScreen'> {
		strategyId: StrategyId;
		activeScreen: boolean | string;
	}

	let selectedStrategyId: StrategyId = 'new';
	let trainingStrategy: CoreStrategy | null = null;
	let editedStrategy: EditableStrategy | null = {
		strategyId: 'new',
		name: '',
		criteria: {
			timeframe: '',
			bars: 0,
			threshold: 0,
			dolvol: 0,
			adr: 0,
			mcap: 0
		},
		score: 0,
		activeScreen: false
	};

	onMount(() => {
		eventDispatcher.subscribe((v) => {
			if (v === 'new') {
				editedStrategy = {
					strategyId: 'new',
					name: '',
					criteria: {
						timeframe: '',
						bars: 0,
						threshold: 0,
						dolvol: 0,
						adr: 0,
						mcap: 0
					},
					score: 0,
					activeScreen: false
				};
				selectedStrategyId = 'new';
			}
		});
	});

	function editStrategy(strategy: EditableStrategy) {
		selectedStrategyId = strategy.strategyId;
		editedStrategy = { ...strategy }; // Create a copy for editing
	}
	
	function cancelEdit() {
		selectedStrategyId = null;
		editedStrategy = null;
		eventDispatcher.set('cancel');
	}

	function train(strategy: EditableStrategy) {
		// Convert EditableStrategy to CoreStrategy for the Trainer component
		if (typeof strategy.strategyId === 'number') {
			trainingStrategy = {
				...strategy,
				strategyId: strategy.strategyId,
				activeScreen: typeof strategy.activeScreen === 'boolean' ? strategy.activeScreen : false
			};
		}
	}

	function createNewStrategy() {
		editedStrategy = {
			strategyId: 'new', //temp id
			name: '',
			criteria: {
				timeframe: '',
				bars: 0,
				threshold: 0,
				dolvol: 0,
				adr: 0,
				mcap: 0
			},
			score: 0,
			activeScreen: false
		};
		selectedStrategyId = 'new';
	}
	
	function manualTrain() {
		if (selectedStrategyId === null || selectedStrategyId === 'new') return;
		queueRequest('train', { strategyId: selectedStrategyId });
	}
	
	function deleteStrategy() {
		if (!selectedStrategyId || selectedStrategyId === 'new') return;

		privateRequest('deleteStrategy', { strategyId: selectedStrategyId }).then(() => {
			strategies.update((currentStrategies) => {
				return currentStrategies.filter((strategy) => strategy.strategyId !== selectedStrategyId);
			});
			selectedStrategyId = null;
		});
	}
	
	function saveStrategy() {
		if (!editedStrategy) return;

		if (selectedStrategyId === 'new') {
			privateRequest<CoreStrategy>('newStrategy', { ...editedStrategy }).then((s: CoreStrategy) => {
				strategies.update((o: CoreStrategy[]) => [...o, s]);
				selectedStrategyId = s.strategyId;
				editedStrategy = { ...s, activeScreen: false };
			});
		} else {
			privateRequest<void>('setStrategy', { ...editedStrategy }).then(() => {
				strategies.update((currentStrategies: CoreStrategy[]) =>
					currentStrategies.map((s) => {
						if (editedStrategy && s.strategyId === editedStrategy.strategyId) {
							// Convert EditableStrategy to CoreStrategy
							const convertedStrategy: CoreStrategy = {
								...editedStrategy,
								strategyId: typeof editedStrategy.strategyId === 'number' ? editedStrategy.strategyId : 0,
								activeScreen:
									typeof editedStrategy.activeScreen === 'boolean' ? editedStrategy.activeScreen : false
							};
							return convertedStrategy;
						}
						return s;
					})
				);
			});
		}
	}
</script>

{#if selectedStrategyId === null && trainingStrategy === null}
	<button on:click={createNewStrategy}>New Strategy</button>
	<div class="table-container">
		<table>
			<thead>
				<tr class="defalt-tr">
					<th class="defalt-th">Strategy</th>
					<th class="defalt-th">Score</th>
					<th class="defalt-th">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#if Array.isArray($strategies) && $strategies.length > 0}
					{#each $strategies as strategy}
						<tr class="defalt-tr">
							<td class="defalt-td">{strategy.name}</td>
							<td class="defalt-td">{strategy.score}</td>
							<td class="defalt-td">
								<button
									on:click={() => train({ ...strategy, strategyId: strategy.strategyId, activeScreen: false })}
									>Train</button
								>
								<button
									on:click={() =>
										editStrategy({ ...strategy, strategyId: strategy.strategyId, activeScreen: false })}
									>Edit</button
								>
							</td>
						</tr>
					{/each}
				{:else}
					<tr class="defalt-tr">
						<td colspan="3">No strategies available.</td>
					</tr>
				{/if}
			</tbody>
		</table>
	</div>
{:else if selectedStrategyId !== null && editedStrategy !== null}
	<div>
		<label>Name: <input type="text" bind:value={editedStrategy.name} /></label>
	</div>
	<div>
		<label>Timeframe: <input type="text" bind:value={editedStrategy.criteria.timeframe} /></label>
	</div>
	<div>
		<label>Bars: <input type="number" bind:value={editedStrategy.criteria.bars} /></label>
	</div>
	<div>
		<label>Threshold: <input type="number" bind:value={editedStrategy.criteria.threshold} /></label>
	</div>
	<div>
		<label>DolVol: <input type="number" bind:value={editedStrategy.criteria.dolvol} /></label>
	</div>
	<div>
		<label>ADR: <input type="number" bind:value={editedStrategy.criteria.adr} /></label>
	</div>
	<div>
		<label>MCap: <input type="number" bind:value={editedStrategy.criteria.mcap} /></label>
	</div>
	<button on:click={saveStrategy}>Save</button>
	<button on:click={cancelEdit}>Cancel</button>
	<!-- <button on:click={manualTrain}>(dev) Manual Train</button> -->
	{#if selectedStrategyId !== 'new'}
		<button on:click={deleteStrategy}>Delete</button>
	{/if}
{:else if trainingStrategy !== null}
	<Trainer
		handleExit={() => {
			trainingStrategy = null;
		}}
		strategy={trainingStrategy}
	/>
{/if}
