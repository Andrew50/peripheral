<script lang="ts">
	import { queueRequest, privateRequest } from '$lib/core/backend';
	import type { Setup } from '$lib/core/types';
	import { setups } from '$lib/core/stores';
	import '$lib/core/global.css';
	import Trainer from './trainer.svelte';
	import { onMount } from 'svelte';
	let selectedSetupId: number | null | 'new' = null;
	let trainingSetup: Setup | null = null;
	let editedSetup: Setup | null = null;
	import { eventDispatcher } from './interface';
	onMount(() => {
		eventDispatcher.subscribe((v: SetupEvent) => {
			if (v === 'new') {
				createNewSetup();
			}
		});
	});

	function editSetup(setup: Setup) {
		selectedSetupId = setup.setupId;
		editedSetup = { ...setup }; // Create a copy for editing
	}
	function cancelEdit() {
		selectedSetupId = null;
		editedSetup = null;
		eventDispatcher.set('cancel');
	}

	function train(setup: Setup) {
		trainingSetup = setup;
	}

	function createNewSetup() {
		editedSetup = {
			setupId: 'new', //temp id
			name: '',
			timeframe: '',
			bars: 0,
			threshold: 0,
			dolvol: 0,
			adr: 0,
			mcap: 0,
			score: 0
		};
		selectedSetupId = 'new';
	}
	function manualTrain() {
		queueRequest('train', { setupId: selectedSetupId });
	}
	function deleteSetup() {
		if (!selectedSetupId) return;

		privateRequest('deleteSetup', { setupId: selectedSetupId }).then(() => {
			setups.update((currentSetups) => {
				return currentSetups.filter((setup) => setup.setupId !== selectedSetupId);
			});
			selectedSetupId = null;
		});
	}
	function saveSetup() {
		if (!editedSetup?.name || !editedSetup?.bars || !editedSetup?.timeframe) return;
		if (selectedSetupId === 'new') {
			privateRequest<Setup>('newSetup', editedSetup).then((s: Setup) => {
				setups.update((o: Setup[]) => {
					return [...o, s];
				});
				eventDispatcher.set(s.setupId);
			});
		} else {
			privateRequest('setSetup', editedSetup).then(() => {
				setups.update((currentSetups) => {
					return currentSetups.map((setup) =>
						setup.setupId === editedSetup!.setupId ? editedSetup! : setup
					);
				});
				selectedSetupId = null;
				editedSetup = null;
			});
		}
		selectedSetupId = null;
	}
</script>

{#if selectedSetupId === null && trainingSetup === null}
	<button on:click={createNewSetup}>New Setup</button>
	<div class="table-container">
		<table>
			<thead>
				<tr class="defalt-tr">
					<th class="defalt-th">Setup</th>
					<th class="defalt-th">Score</th>
					<th class="defalt-th">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#if Array.isArray($setups) && $setups.length > 0}
					{#each $setups as setup}
						<tr class="defalt-tr">
							<td class="defalt-td">{setup.name}</td>
							<td class="defalt-td">{setup.score}</td>
							<td class="defalt-td">
								<button on:click={() => train(setup)}>Train</button>
								<button on:click={() => editSetup(setup)}>Edit</button>
							</td>
						</tr>
					{/each}
				{:else}
					<tr class="defalt-tr">
						<td colspan="2">No setups available.</td>
					</tr>
				{/if}
			</tbody>
		</table>
	</div>
{:else if selectedSetupId !== null}
	<div>
		<label>Name: <input type="text" bind:value={editedSetup.name} /></label>
	</div>
	<div>
		<label>Timeframe: <input type="text" bind:value={editedSetup.timeframe} /></label>
	</div>
	<div>
		<label>Bars: <input type="number" bind:value={editedSetup.bars} /></label>
	</div>
	<div>
		<label>Threshold: <input type="number" bind:value={editedSetup.threshold} /></label>
	</div>
	<div>
		<label>DolVol: <input type="number" bind:value={editedSetup.dolvol} /></label>
	</div>
	<div>
		<label>ADR: <input type="number" bind:value={editedSetup.adr} /></label>
	</div>
	<div>
		<label>MCap: <input type="number" bind:value={editedSetup.mcap} /></label>
	</div>
	<button on:click={saveSetup}>Save</button>
	<button on:click={cancelEdit}>Cancel</button>
	<button on:click={manualTrain}>(dev) Manual Train</button>
	{#if selectedSetupId !== 'new'}
		<button on:click={deleteSetup}>Delete</button>
	{/if}
{:else if trainingSetup !== null}
	<Trainer
		handleExit={() => {
			trainingSetup = null;
		}}
		setup={trainingSetup}
	/>
{/if}
