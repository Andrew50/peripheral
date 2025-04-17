<!-- screen.svelte-->
<script lang="ts">
	import { writable, get } from 'svelte/store';
	import List from '$lib/utils/modules/list.svelte';
	import '$lib/core/global.css';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { queueRequest } from '$lib/core/backend';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';
	import { strategies } from '$lib/core/stores';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';

	// Assume this function is already coded and imported

	let screens: Writable<Screen[]> = writable([]);
	let selectedDate: Writable<number> = writable(0); // Store for the date, default to current date

	interface Screen extends Instance {
		strategyType: string;
		score: number;
		flagged: boolean;
	}

	function runScreen() {
		const strategyIds = get(strategies)
			.filter((v) => v.activeScreen)
			.map((v) => v.strategyId);
		const dateToScreen = get(selectedDate); // Get the currently selected date
		queueRequest<Screen[]>('screen', { strategyIds: strategyIds, timestamp: dateToScreen / 1000 }).then(
			(response) => {
				response;
				screens.set(response);
			}
		);
	}

	// This function is called when the date button is clicked
	function changeDate() {
		queryInstanceInput(['timestamp'], ['timestamp'], { timestamp: $selectedDate }).then(
			(v: Instance) => {
				if (v.timestamp !== undefined) {
					selectedDate.set(v.timestamp); // Update the date store with the new selected date
				}
			}
		);
	}
</script>

<div class="controls-container">
	{#if Array.isArray($strategies) && $strategies.length > 0}
		{#each $strategies as strategy (strategy.strategyId)}
			<button
				class="toggle-button {strategy.activeScreen ? 'active' : ''}"
				on:click={() => {
					strategy.activeScreen = !strategy.activeScreen;
				}}
			>
				{strategy.name}
			</button>
		{/each}
	{/if}
</div>

<div class="button-row">
	<button on:click={changeDate}> Change Date </button>
	<button on:click={runScreen}> Screen {UTCTimestampToESTString($selectedDate)} </button>
</div>

<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={screens}
	columns={['Ticker', 'Chg%', 'Setup', 'Score', 'Ext.']}
/>

<style>
	.controls-container {
		display: flex;
		justify-content: flex-start;
		gap: clamp(0.375rem, 1vw, 0.5rem);
		margin-bottom: clamp(0.75rem, 2vh, 1rem);
		margin-top: clamp(0.5rem, 1vh, 0.75rem);
		flex-wrap: wrap;
		width: 100%;
	}

	.toggle-button {
		margin-right: 0;
		padding: clamp(0.25rem, 0.75vw, 0.375rem) clamp(0.5rem, 1vw, 0.625rem);
		min-width: clamp(4rem, 10vw, 5rem);
		height: clamp(2rem, 5vh, 2.5rem);
		font-weight: 500;
		transition: all 0.2s ease;
		border-radius: clamp(4px, 0.5vw, 6px);
		position: relative;
		overflow: hidden;
	}

	.toggle-button:after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 0;
		height: 2px;
		background: var(--ui-accent, #4a80f0);
		transition: width 0.2s ease;
	}

	.toggle-button:hover:after {
		width: 100%;
	}

	.toggle-button.active {
		background: var(--ui-bg-hover);
		border-bottom: 2px solid var(--ui-accent, #4a80f0);
	}

	.toggle-button.active:after {
		width: 100%;
	}

	.button-row {
		display: flex;
		gap: clamp(0.5rem, 1vw, 0.625rem);
		margin-bottom: clamp(0.75rem, 2vh, 1rem);
	}

	button {
		min-width: clamp(7.5rem, 15vw, 9rem);
		border-radius: clamp(4px, 0.5vw, 6px);
		font-weight: 500;
		letter-spacing: 0.3px;
	}
</style>
