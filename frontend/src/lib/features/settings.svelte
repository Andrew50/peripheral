<script lang="ts">
	import { settings } from '$lib/core/stores';
	import { get } from 'svelte/store';
	import type { Settings } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import '$lib/core/global.css';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';

	let errorMessage: string = '';
	let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with
	let activeTab: 'chart' | 'format' | 'account' = 'chart';

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

	function handleLogout() {
		if (browser) {
			sessionStorage.removeItem('authToken');
			sessionStorage.removeItem('profilePic');
			sessionStorage.removeItem('username');
		}
		goto('/');
	}
</script>

<div class="settings-container">
	<div class="settings-layout">
		<div class="sidebar">
			<div class="tabs">
				<button
					class="tab-button {activeTab === 'account' ? 'active' : ''}"
					on:click={() => (activeTab = 'account')}
				>
					Account
				</button>
				<button
					class="tab-button {activeTab === 'chart' ? 'active' : ''}"
					on:click={() => (activeTab = 'chart')}
				>
					Chart
				</button>
				<button
					class="tab-button {activeTab === 'format' ? 'active' : ''}"
					on:click={() => (activeTab = 'format')}
				>
					Format
				</button>
			</div>
			<button class="logout-button" on:click={handleLogout}>Logout</button>
		</div>

		<div class="settings-content">
			<h2>{activeTab.charAt(0).toUpperCase() + activeTab.slice(1)} Settings</h2>

			{#if activeTab === 'chart'}
				<div class="settings-group">
					<div class="setting-row">
						<label for="chartRows">Chart Rows</label>
						<input
							type="number"
							id="chartRows"
							bind:value={tempSettings.chartRows}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</div>

					<div class="setting-row">
						<label for="chartColumns">Chart Columns</label>
						<input
							type="number"
							id="chartColumns"
							bind:value={tempSettings.chartColumns}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</div>

					<div class="setting-row">
						<label for="adrPeriod">AR Period</label>
						<input
							type="number"
							id="adrPeriod"
							bind:value={tempSettings.adrPeriod}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</div>
				</div>
			{:else if activeTab === 'format'}
				<div class="settings-group">
					<div class="setting-row">
						<label for="dolvol">Dollar Volume</label>
						<select
							class="default-select"
							id="dolvol"
							bind:value={tempSettings.dolvol}
							on:keypress={handleKeyPress}
						>
							<option value={true}>Yes</option>
							<option value={false}>No</option>
						</select>
					</div>

					<div class="setting-row">
						<label for="filterTaS">Display Trades Less than 100 shares</label>
						<select
							class="default-select"
							id="filterTaS"
							bind:value={tempSettings.filterTaS}
							on:keypress={handleKeyPress}
						>
							<option value={true}>Yes</option>
							<option value={false}>No</option>
						</select>
					</div>

					<div class="setting-row">
						<label for="divideTaS">Divide Time and Sales by 100</label>
						<select
							class="default-select"
							id="divideTaS"
							bind:value={tempSettings.divideTaS}
							on:keypress={handleKeyPress}
						>
							<option value={true}>Yes</option>
							<option value={false}>No</option>
						</select>
					</div>
				</div>
			{:else if activeTab === 'account'}
				<div class="settings-group">
					<div class="info-message">Account settings coming soon</div>
				</div>
			{/if}

			{#if errorMessage}
				<p class="error">{errorMessage}</p>
			{/if}

			<button class="submit-button" on:click={updateLayout}>Apply Changes</button>
		</div>
	</div>
</div>

<style>
	.settings-container {
		padding: 2rem;
		color: var(--text-primary);
		min-height: 100vh;
		display: flex;
		justify-content: center;
		align-items: flex-start;
	}

	.settings-layout {
		display: flex;
		gap: 2rem;
		background: rgba(255, 255, 255, 0.05);
		padding: 2rem;
		border-radius: 12px;
		backdrop-filter: blur(10px);
		width: 100%;
		max-width: 900px;
	}

	.sidebar {
		width: 200px;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		border-right: 1px solid rgba(255, 255, 255, 0.1);
		padding-right: 2rem;
	}

	.tabs {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.tab-button {
		text-align: left;
		padding: 0.75rem 1rem;
		background: transparent;
		border: none;
		border-radius: 6px;
		color: var(--text-secondary);
		font-size: 0.9rem;
		transition: all 0.2s ease;
	}

	.tab-button:hover {
		background: rgba(255, 255, 255, 0.05);
		color: var(--text-primary);
	}

	.tab-button.active {
		background: rgba(59, 130, 246, 0.1);
		color: #3b82f6;
	}

	.settings-content {
		flex: 1;
		min-width: 0;
	}

	h2 {
		color: white;
		margin-bottom: 2rem;
		font-size: 1.5rem;
		font-weight: 500;
	}

	.settings-group {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.setting-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		background: rgba(255, 255, 255, 0.03);
		border-radius: 6px;
		transition: background-color 0.2s ease;
	}

	.setting-row:hover {
		background: rgba(255, 255, 255, 0.05);
	}

	label {
		color: var(--text-secondary);
		font-size: 0.9rem;
	}

	input,
	select {
		padding: 0.75rem;
		border-radius: 6px;
		border: 1px solid rgba(255, 255, 255, 0.1);
		background: rgba(255, 255, 255, 0.05);
		color: white;
		font-size: 0.9rem;
		width: 150px;
	}

	input:focus,
	select:focus {
		outline: none;
		border-color: #3b82f6;
	}

	.error {
		color: #ef4444;
		text-align: center;
		margin-top: 1rem;
		font-size: 0.9rem;
	}

	.submit-button {
		width: 100%;
		padding: 1rem;
		background: #3b82f6;
		color: white;
		border: none;
		border-radius: 6px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.3s ease;
		margin-top: 2rem;
	}

	.submit-button:hover {
		background: #2563eb;
	}

	.logout-button {
		padding: 0.75rem 1rem;
		background: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border: 1px solid rgba(239, 68, 68, 0.2);
		border-radius: 6px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.3s ease;
	}

	.logout-button:hover {
		background: rgba(239, 68, 68, 0.2);
	}

	.info-message {
		color: var(--text-secondary);
		text-align: center;
		padding: 2rem;
		background: rgba(255, 255, 255, 0.03);
		border-radius: 6px;
	}
</style>
