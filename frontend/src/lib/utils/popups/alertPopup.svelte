<script lang="ts">
	import { alertPopup } from '$lib/core/stores';
	import { fade } from 'svelte/transition';

	function dismissAlert(alertId: number) {
		alertPopup.set(null);
	}
</script>

<div class="alert-container">
	{#if $alertPopup}
		<div class="alert-popup" transition:fade on:click={() => dismissAlert()}>
			<div class="alert-content">
				<span class="value">{$alertPopup.message}</span>
				<span class="label">{new Date($alertPopup.timestamp).toLocaleTimeString()}</span>
			</div>
		</div>
	{/if}
</div>

<style>
	.alert-container {
		position: fixed;
		top: 20px;
		right: 20px;
		z-index: 1000;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.alert-popup {
		background-color: var(--c2);
		border: 1px solid var(--c4);
		padding: 15px;
		border-radius: 4px;
		min-width: 200px;
		cursor: pointer;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
	}

	.alert-content {
		display: flex;
		flex-direction: column;
		gap: 5px;
	}
</style>
