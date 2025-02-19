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
		width: 300px;
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
		padding: 12px;
	}

	.alert-content {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.value {
		color: var(--text-primary);
		font-size: 14px;
		font-weight: 500;
	}

	.label {
		color: var(--text-secondary);
		font-size: 12px;
	}
</style>
