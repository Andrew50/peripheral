<script lang="ts">
	import { writable } from 'svelte/store';
	import { startReplay, addStream } from '$lib/utils/stream/interface';
	import '$lib/core/global.css';

	interface StoreType {
		price: number;
	}

	const store = writable<StoreType>({ price: 0 });

	function test() {
		//startReplay(1725894480000)
		//store = geeam(400,"fast")
	}
</script>

<div class="test-container">
	<button class="test-button" on:click={test}>test</button>
	<div class="test-value">{$store.price}</div>
</div>

<style>
	.test-container {
		display: flex;
		align-items: center;
		gap: clamp(0.5rem, 1vw, 0.75rem);
		padding: clamp(0.75rem, 2vw, 1rem);
	}

	.test-button {
		padding: clamp(0.375rem, 1vw, 0.5rem) clamp(0.75rem, 1.5vw, 1rem);
		border-radius: clamp(4px, 0.5vw, 6px);
		font-size: clamp(0.875rem, 1vw, 1rem);
		background: var(--ui-bg-primary, rgba(30, 41, 59, 0.5));
		color: var(--text-primary, white);
		border: 1px solid var(--ui-border, rgba(59, 130, 246, 0.2));
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.test-button:hover {
		background: var(--ui-bg-hover, rgba(40, 50, 70, 0.7));
	}

	.test-value {
		font-size: clamp(0.875rem, 1vw, 1rem);
		color: var(--text-primary, white);
	}
</style>
