<!-- screen.svelte-->
<script lang="ts">
	import { writable, get } from 'svelte/store';
	import List from '$lib/utils/modules/list.svelte';
	import '$lib/core/global.css';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { queueRequest } from '$lib/core/backend';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';
	import { setups } from '$lib/core/stores';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';

	// Assume this function is already coded and imported

	let screens: Writable<Screen[]> = writable([]);
	let selectedDate: Writable<number> = writable(0); // Store for the date, default to current date

	interface Screen extends Instance {
		setupType: string;
		score: number;
		flagged: boolean;
	}

	function runScreen() {
		const setupIds = get(setups)
			.filter((v) => v.activeScreen)
			.map((v) => v.setupId);
		const dateToScreen = get(selectedDate); // Get the currently selected date
		queueRequest<Screen[]>('screen', { setupIds: setupIds, timestamp: dateToScreen / 1000 }).then(
			(response) => {
				console.log(response);
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
	{#if Array.isArray($setups) && $setups.length > 0}
		{#each $setups as setup (setup.setupId)}
			<button
				class="toggle-button {setup.activeScreen ? 'active' : ''}"
				on:click={() => {
					setup.activeScreen = !setup.activeScreen;
				}}
			>
				{setup.name}
			</button>
		{/each}
	{/if}
</div>

<button on:click={changeDate}> Change Date </button>
<button on:click={runScreen}> Screen {UTCTimestampToESTString($selectedDate)} </button>

<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={screens}
	columns={['Ticker', 'Chg', 'Setup', 'Score']}
/>
