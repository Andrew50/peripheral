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
				settings.set(tempSettings); // Update the store with new settings
				errorMessage = '';
			});
		} else {
			errorMessage = 'Chart rows and columns must be greater than 0';
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

<div class="settings-panel">
	<div class="settings-tabs">
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

	<div class="settings-content">
		{#if activeTab === 'chart'}
			<div class="settings-section">
				<h3>Chart Layout</h3>
				<div class="settings-grid">
					<div class="setting-item">
						<label for="chartRows">Chart Rows</label>
						<input
							type="number"
							id="chartRows"
							bind:value={tempSettings.chartRows}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</div>

					<div class="setting-item">
						<label for="chartColumns">Chart Columns</label>
						<input
							type="number"
							id="chartColumns"
							bind:value={tempSettings.chartColumns}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</div>
				</div>
			</div>

			<div class="settings-section">
				<h3>Technical Indicators</h3>
				<div class="settings-grid">
					<div class="setting-item">
						<label for="adrPeriod">Average Range Period</label>
						<input
							type="number"
							id="adrPeriod"
							bind:value={tempSettings.adrPeriod}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</div>
				</div>
			</div>
		{:else if activeTab === 'format'}
			<div class="settings-section">
				<h3>Display Options</h3>
				<div class="settings-grid">
					<div class="setting-item">
						<label for="dolvol">Show Dollar Volume</label>
						<div class="toggle-container">
							<select id="dolvol" bind:value={tempSettings.dolvol} on:keypress={handleKeyPress}>
								<option value={true}>Yes</option>
								<option value={false}>No</option>
							</select>
						</div>
					</div>

					<div class="setting-item">
						<label for="showFilings">Show SEC Filings</label>
						<div class="toggle-container">
							<select
								id="showFilings"
								bind:value={tempSettings.showFilings}
								on:keypress={handleKeyPress}
							>
								<option value={true}>Yes</option>
								<option value={false}>No</option>
							</select>
						</div>
					</div>
				</div>
			</div>

			<div class="settings-section">
				<h3>Time & Sales</h3>
				<div class="settings-grid">
					<div class="setting-item">
						<label for="filterTaS">Show trades less than 100 shares</label>
						<div class="toggle-container">
							<select
								id="filterTaS"
								bind:value={tempSettings.filterTaS}
								on:keypress={handleKeyPress}
							>
								<option value={true}>Yes</option>
								<option value={false}>No</option>
							</select>
						</div>
					</div>

					<div class="setting-item">
						<label for="divideTaS">Divide Time and Sales by 100</label>
						<div class="toggle-container">
							<select
								id="divideTaS"
								bind:value={tempSettings.divideTaS}
								on:keypress={handleKeyPress}
							>
								<option value={true}>Yes</option>
								<option value={false}>No</option>
							</select>
						</div>
					</div>
				</div>
			</div>
		{:else if activeTab === 'account'}
			<div class="settings-section">
				<h3>Account Information</h3>
				<p class="info-message">Account settings will be available soon</p>
				<div class="account-actions">
					<button class="logout-button" on:click={handleLogout}>Logout</button>
				</div>
			</div>
		{/if}

		{#if errorMessage}
			<div class="error-message">{errorMessage}</div>
		{/if}

		<div class="settings-actions">
			<button class="apply-button" on:click={updateLayout}>Apply Changes</button>
		</div>
	</div>
</div>

<style>
	.settings-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		color: var(--f1);
		background-color: var(--c1);
		border-radius: 4px;
		overflow: hidden;
	}

	.settings-tabs {
		display: flex;
		background-color: var(--c2);
		border-bottom: 1px solid var(--c3);
	}

	.tab-button {
		padding: 10px 16px;
		background: transparent;
		border: none;
		color: var(--f2);
		font-size: 14px;
		cursor: pointer;
		transition:
			background-color 0.2s,
			color 0.2s;
		text-align: center;
		flex: 1;
	}

	.tab-button:hover {
		background-color: rgba(255, 255, 255, 0.05);
		color: var(--f1);
	}

	.tab-button.active {
		background-color: var(--c3);
		color: var(--f1);
		font-weight: 500;
	}

	.settings-content {
		flex: 1;
		padding: 16px;
		overflow-y: auto;
	}

	.settings-section {
		margin-bottom: 20px;
		border-bottom: 1px solid var(--c3);
		padding-bottom: 16px;
	}

	.settings-section:last-child {
		border-bottom: none;
		margin-bottom: 0;
	}

	h3 {
		margin: 0 0 12px 0;
		font-size: 16px;
		font-weight: 500;
		color: var(--f1);
	}

	.settings-grid {
		display: grid;
		grid-template-columns: 1fr;
		gap: 10px;
	}

	@media (min-width: 600px) {
		.settings-grid {
			grid-template-columns: 1fr 1fr;
		}
	}

	.setting-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 8px 12px;
		background-color: var(--c2);
		border-radius: 4px;
		border: 1px solid var(--c3);
	}

	label {
		font-size: 14px;
		color: var(--f2);
	}

	input[type='number'] {
		width: 80px;
		padding: 6px 8px;
		background-color: var(--c1);
		border: 1px solid var(--c4);
		border-radius: 4px;
		color: var(--f1);
		font-size: 14px;
		text-align: center;
	}

	input[type='number']:focus {
		outline: none;
		border-color: #3b82f6;
	}

	.toggle-container select {
		padding: 6px 8px;
		background-color: var(--c1);
		border: 1px solid var(--c4);
		border-radius: 4px;
		color: var(--f1);
		font-size: 14px;
		min-width: 80px;
		cursor: pointer;
	}

	.toggle-container select:focus {
		outline: none;
		border-color: #3b82f6;
	}

	.info-message {
		padding: 12px;
		background-color: var(--c2);
		border-radius: 4px;
		color: var(--f2);
		font-size: 14px;
		text-align: center;
		border: 1px solid var(--c3);
	}

	.account-actions {
		margin-top: 20px;
		display: flex;
		justify-content: center;
	}

	.error-message {
		margin: 16px 0;
		padding: 10px;
		background-color: rgba(239, 68, 68, 0.2);
		color: #ef4444;
		border-radius: 4px;
		font-size: 14px;
		text-align: center;
	}

	.settings-actions {
		margin-top: 20px;
		display: flex;
		justify-content: flex-end;
	}

	.apply-button {
		padding: 8px 16px;
		background-color: #3b82f6;
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 14px;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.apply-button:hover {
		background-color: #2563eb;
	}

	.logout-button {
		padding: 8px 16px;
		background-color: rgba(239, 68, 68, 0.15);
		color: #ef4444;
		border: 1px solid rgba(239, 68, 68, 0.3);
		border-radius: 4px;
		font-size: 14px;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.logout-button:hover {
		background-color: rgba(239, 68, 68, 0.25);
	}
</style>
