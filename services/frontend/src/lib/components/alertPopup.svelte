<script lang="ts">
	import { alertPopup } from '$lib/utils/stores/stores';
	import { fade } from 'svelte/transition';

	function dismissAlert(alertId: number = 0) {
		alertPopup.set(null);
	}
</script>

<div class="alert-container">
	{#if $alertPopup}
		<div
			class="alert-popup responsive-shadow responsive-border content-padding"
			transition:fade
			on:click={() => dismissAlert($alertPopup.alertId)}
		>
			<div class="alert-content">
				<span class="value fluid-text">{$alertPopup.message}</span>
				<span class="label fluid-text">{new Date($alertPopup.timestamp).toLocaleTimeString()}</span>
			</div>
		</div>
	{/if}
</div>

<style>
	.alert-container {
		position: fixed;
		top: clamp(10px, 2vh, 20px);
		right: clamp(10px, 2vw, 20px);
		z-index: 1000;
		display: flex;
		flex-direction: column;
		gap: clamp(5px, 1vh, 10px);
	}

	.alert-popup {
		width: clamp(250px, 30vw, 300px);
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: clamp(6px, 0.8vw, 8px);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
	}

	.alert-content {
		display: flex;
		flex-direction: column;
		gap: clamp(4px, 0.8vh, 8px);
	}

	.value {
		color: var(--text-primary);
		font-weight: 500;
	}

	.label {
		color: var(--text-secondary);
	}
</style>
