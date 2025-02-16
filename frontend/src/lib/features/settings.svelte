<script lang="ts">
	import { settings } from '$lib/core/stores';
	import { get } from 'svelte/store';
	import type { Settings } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import '$lib/core/global.css';

	let errorMessage: string = '';
	let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with

	function updateLayout() {
		if (tempSettings.chartRows > 0 && tempSettings.chartColumns > 0) {
			privateRequest<void>('setSettings', { settings: tempSettings }).then(() => {
				console.log(tempSettings);
				settings.set(tempSettings); // Update the store with new settings
				errorMessage = '';
			});
		} else {
			errorMessage = 'invalid settings';
		}
	}

	function handleKeyPress(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			updateLayout();
		}
	}
</script>

<div class="settings-container">
	<h2>Settings</h2>
	<div class="content">
		<div class="settings-group">
			<div class="setting-row">
				<label for="chartRows">Chart Rows:</label>
				<input
					type="number"
					id="chartRows"
					bind:value={tempSettings.chartRows}
					min="1"
					on:keypress={handleKeyPress}
				/>
			</div>

			<div class="setting-row">
				<label for="chartColumns">Chart Columns:</label>
				<input
					type="number"
					id="chartColumns"
					bind:value={tempSettings.chartColumns}
					min="1"
					on:keypress={handleKeyPress}
				/>
			</div>

			<div class="setting-row">
				<label for="dolvol">Dollar Volume:</label>
				<select id="dolvol" bind:value={tempSettings.dolvol} on:keypress={handleKeyPress}>
					<option value={true}>Yes</option>
					<option value={false}>No</option>
				</select>
			</div>

			<div class="setting-row">
				<label for="adrPeriod">AR Period:</label>
				<input
					type="number"
					id="adrPeriod"
					bind:value={tempSettings.adrPeriod}
					min="1"
					on:keypress={handleKeyPress}
				/>
			</div>

			<div class="setting-row">
				<label for="filterTaS">Display Trades Less than 100 shares:</label>
				<select id="filterTaS" bind:value={tempSettings.filterTaS} on:keypress={handleKeyPress}>
					<option value={true}>Yes</option>
					<option value={false}>No</option>
				</select>
			</div>

			<div class="setting-row">
				<label for="divideTaS">Divide Time and Sales by 100:</label>
				<select id="divideTaS" bind:value={tempSettings.divideTaS} on:keypress={handleKeyPress}>
					<option value={true}>Yes</option>
					<option value={false}>No</option>
				</select>
			</div>
		</div>

		{#if errorMessage}
			<p class="error-message">{errorMessage}</p>
		{/if}

		<button class="apply-button" on:click={updateLayout}>Apply</button>
	</div>
</div>

<style>
	.settings-container {
		padding: 20px;
		color: var(--text-primary);
	}

	h2 {
		margin-bottom: 20px;
		font-size: 18px;
		font-weight: 500;
	}

	.content {
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		padding: 20px;
	}

	.settings-group {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.setting-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 8px;
		background: var(--ui-bg-element);
		border-radius: 4px;
	}

	.setting-row:hover {
		background: var(--ui-bg-hover);
	}

	label {
		color: var(--text-secondary);
		font-size: 14px;
		font-weight: 500;
	}

	input,
	select {
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		padding: 6px 12px;
		font-size: 14px;
		width: 150px;
	}

	input:focus,
	select:focus {
		outline: none;
		border-color: var(--ui-border-focus);
	}

	.error-message {
		color: var(--text-error);
		margin-top: 16px;
		font-size: 14px;
	}

	.apply-button {
		margin-top: 20px;
		padding: 8px 16px;
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		cursor: pointer;
		font-size: 14px;
		transition: background 0.2s ease;
	}

	.apply-button:hover {
		background: var(--ui-bg-hover);
	}
</style>
