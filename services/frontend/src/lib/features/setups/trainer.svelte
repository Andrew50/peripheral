<script lang="ts">
	import type { Setup } from '$lib/core/types';
	import type { Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import { onMount } from 'svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import { ESTSecondstoUTCSeconds } from '$lib/core/timestamp';

	export let setup: Setup | null = null;
	export let handleExit: (
		event: MouseEvent & { currentTarget: EventTarget & HTMLButtonElement }
	) => void;

	interface TrainingInstance extends Instance {
		id: number;
		sampleId: number;
		timestamp: number;
		securityId: number;
		price: number;
	}

	let trainingQueue: TrainingInstance[] = [];

	function showInstance(instance: TrainingInstance) {
		instance.timestamp = ESTSecondstoUTCSeconds(instance.timestamp) * 1000;
		queryChart(instance);
	}

	function refillQueue() {
		if (setup?.setupId) {
			privateRequest<TrainingInstance[]>('getTrainingQueue', { setupId: setup.setupId }, true).then(
				(v: TrainingInstance[]) => {
					trainingQueue = v;
				}
			);
		}
	}

	onMount(() => {
		refillQueue();
	});

	function label(c: string) {
		if (c === 'yes' || c === 'no') {
			const boolLabel = c === 'yes' ? true : false;
			privateRequest<TrainingInstance[]>('labelTrainingQueueInstance', {
				sampleId: trainingQueue[0].sampleId,
				label: boolLabel
			});
		}
		trainingQueue.shift();
		if (!Array.isArray(trainingQueue) || trainingQueue.length == 0) {
			refillQueue();
		} else {
			showInstance(trainingQueue[0]);
		}
	}
</script>

<div class="feature-container">
	<button class="trainer-button" on:click={handleExit}>Exit</button>
	<h1 class="trainer-title">Setup: {setup?.name}</h1>
	<h2 class="trainer-score">Score: {setup?.score}</h2>

	<div class="trainer-question">
		<h2>Is this a {setup?.name}?</h2>
	</div>
	<div class="trainer-actions">
		<button class="trainer-button yes-button" on:click={() => label('yes')}>Yes</button>
		<button class="trainer-button no-button" on:click={() => label('no')}>No</button>
		<button class="trainer-button skip-button" on:click={() => label('skip')}>Skip</button>
	</div>
</div>

<style>
	.feature-container {
		padding: clamp(1rem, 3vw, 2rem);
		display: flex;
		flex-direction: column;
		gap: clamp(1rem, 3vh, 1.5rem);
	}

	.trainer-title,
	.trainer-score {
		margin: 0;
		color: var(--text-primary, white);
		font-size: clamp(1.25rem, 2.5vw, 1.5rem);
	}

	.trainer-question {
		margin-top: clamp(1rem, 3vh, 2rem);
	}

	.trainer-question h2 {
		font-size: clamp(1.125rem, 2vw, 1.25rem);
		color: var(--text-primary, white);
		margin: 0;
	}

	.trainer-actions {
		display: flex;
		gap: clamp(0.75rem, 2vw, 1rem);
		flex-wrap: wrap;
	}

	.trainer-button {
		padding: clamp(0.5rem, 1.5vw, 0.75rem) clamp(1rem, 3vw, 1.5rem);
		border-radius: clamp(4px, 0.5vw, 6px);
		font-size: clamp(0.875rem, 1.25vw, 1rem);
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
		background: var(--ui-bg-primary, rgba(30, 41, 59, 0.5));
		color: var(--text-primary, white);
		border: 1px solid var(--ui-border, rgba(59, 130, 246, 0.2));
	}

	.trainer-button:hover {
		transform: translateY(-2px);
		background: var(--ui-bg-hover, rgba(40, 50, 70, 0.7));
	}

	.yes-button {
		background: rgba(16, 185, 129, 0.2);
		border-color: rgba(16, 185, 129, 0.4);
	}

	.yes-button:hover {
		background: rgba(16, 185, 129, 0.3);
	}

	.no-button {
		background: rgba(239, 68, 68, 0.2);
		border-color: rgba(239, 68, 68, 0.4);
	}

	.no-button:hover {
		background: rgba(239, 68, 68, 0.3);
	}

	@media (max-width: 640px) {
		.trainer-actions {
			flex-direction: column;
		}
	}
</style>
