<!-- replay.svelte -->
<script lang="ts">
	import {
		startReplay,
		stopReplay,
		pauseReplay,
		changeSpeed,
		resumeReplay,
		nextDay
	} from '$lib/utils/stream/interface';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import { streamInfo } from '$lib/utils/stores/stores';
	import type { StreamInfo } from '$lib/utils/stores/stores';
	import '$lib/styles/global.css';

	import type { Instance } from '$lib/utils/types/types';
	function strtReplay() {
		queryInstanceInput(['timestamp'], ['timestamp'], { timestamp: 0, extendedHours: false }).then(
			(v: Instance) => {
				/*            streamInfo.update((r:StreamInfo) => {
                r.startTimestamp = v.timestamp
                return r
            })*/
				startReplay(v);
			}
		);
	}
	function changeReplaySpeed(event: Event) {
		const input = event.target as HTMLInputElement;
		const newSpeed = parseFloat(input.value); // Parse the speed as a decimal number
		if (!isNaN(newSpeed) && newSpeed > 0) {
			changeSpeed(newSpeed);
		}
	}
</script>

<div class="replay-controls" tabindex="-1">
	{#if $streamInfo.replayActive}
		<button class="replay-button" on:click={stopReplay}>Stop</button>
		<button
			class="replay-button"
			on:click={() => {
				stopReplay;
				startReplay({
					timestamp: $streamInfo.startTimestamp,
					extendedHours: $streamInfo.extendedHours
				});
			}}
			>Reset
			<!-- to {UTCTimestampToESTString($replayInfo.startTimestamp)}-->
		</button>

		{#if $streamInfo.replayPaused}
			<button class="replay-button" on:click={resumeReplay}>Play </button>
		{:else}
			<button class="replay-button" on:click={pauseReplay}>Pause</button>
			<div class="speed-control">
				<label for="speed-input">Speed:</label>
				<input
					id="speed-input"
					type="number"
					step="0.1"
					min="0.1"
					value="1.0"
					on:input={changeReplaySpeed}
					class="speed-input"
				/>
			</div>
			<div>
				<button class="replay-button" on:click={nextDay}
					>Jump to next market open (9:30 AM EST)</button
				>
				<!--<button on:click={jumpToNextDay} >Jump to next day (4 AM EST)</button>    -->
			</div>
		{/if}
	{:else}
		<button class="replay-button" on:click={strtReplay}>Start</button>
	{/if}
</div>

<style>
	.replay-controls {
		display: flex;
		flex-wrap: wrap;
		gap: clamp(0.5rem, 1vw, 0.75rem);
		padding: clamp(0.75rem, 2vw, 1rem);
		background: var(--ui-bg-secondary, rgba(30, 41, 59, 0.5));
		border-radius: clamp(4px, 0.5vw, 6px);
		align-items: center;
	}

	.replay-button {
		padding: clamp(0.375rem, 1vw, 0.5rem) clamp(0.75rem, 1.5vw, 1rem);
		border-radius: clamp(4px, 0.5vw, 6px);
		font-size: clamp(0.875rem, 1vw, 1rem);
		background: var(--ui-bg-primary, rgba(20, 30, 45, 0.7));
		color: var(--text-primary, white);
		border: 1px solid var(--ui-border, rgba(59, 130, 246, 0.2));
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.replay-button:hover {
		background: var(--ui-bg-hover, rgba(40, 50, 70, 0.7));
		transform: translateY(-1px);
	}

	.speed-control {
		display: flex;
		align-items: center;
		gap: clamp(0.375rem, 0.75vw, 0.5rem);
	}

	.speed-input {
		width: clamp(3rem, 6vw, 4rem);
		padding: clamp(0.25rem, 0.5vw, 0.375rem);
		border-radius: clamp(3px, 0.4vw, 4px);
		border: 1px solid var(--ui-border, rgba(59, 130, 246, 0.2));
		background: var(--ui-bg-primary, rgba(20, 30, 45, 0.7));
		color: var(--text-primary, white);
	}

	@media (max-width: 768px) {
		.replay-controls {
			flex-direction: column;
			align-items: stretch;
		}
	}
</style>
