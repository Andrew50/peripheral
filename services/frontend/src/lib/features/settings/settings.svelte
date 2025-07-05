<script lang="ts">
	import { settings } from '$lib/utils/stores/stores';
	import { get } from 'svelte/store';
	import type { Settings } from '$lib/utils/types/types';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import '$lib/styles/global.css';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { colorSchemes, applyColorScheme } from '$lib/styles/colorSchemes';
	import { logout } from '$lib/auth';
	import { redirectToCustomerPortal } from '$lib/utils/helpers/stripe';
	import { subscriptionStatus, fetchCombinedSubscriptionAndUsage } from '$lib/utils/stores/stores';

	let errorMessage: string = '';
	let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with
	let activeTab: 'chart' | 'format' | 'account' | 'appearance' = 'account';

	// Add profile picture state
	let profilePic = browser ? sessionStorage.getItem('profilePic') || '' : '';
	let username = browser ? sessionStorage.getItem('username') || '' : '';

	// Delete account variables
	let showDeleteConfirmation = false;
	let deleteConfirmationText = '';
	let deletingAccount = false;

	// Handle manage subscription
	async function handleManageSubscription() {
		subscriptionStatus.update((s) => ({ ...s, loading: true, error: '' }));

		try {
			const response = await privateRequest<{ url: string }>('createCustomerPortal', {});
			redirectToCustomerPortal(response.url);
		} catch (error) {
			console.error('Error opening customer portal:', error);
			subscriptionStatus.update((s) => ({
				...s,
				loading: false,
				error: 'Failed to open subscription management. Please try again.'
			}));
		}
	}

	// Initialize component
	async function initializeComponent() {
		await fetchCombinedSubscriptionAndUsage();
	}

	// Run initialization on mount
	onMount(() => {
		initializeComponent();
	});

	function saveSettings() {
		if (tempSettings.chartRows > 0 && tempSettings.chartColumns > 0) {
			privateRequest<void>('setSettings', { settings: tempSettings }).then(() => {
				settings.set(tempSettings);
				errorMessage = '';
			});
		} else {
			errorMessage = 'Chart rows and columns must be greater than 0';
		}
	}

	function resetSettings() {
		tempSettings = { ...get(settings) };
		saveSettings();
	}

	function handleKeyPress(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			saveSettings();
		}
	}

	// Generate initial avatar SVG from username
	function generateInitialAvatar(username: string) {
		const initial = username.charAt(0).toUpperCase();
		return `data:image/svg+xml,${encodeURIComponent(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="50" fill="#1a1c21"/><text x="50" y="65" font-family="Arial" font-size="40" fill="#e0e0e0" text-anchor="middle" font-weight="bold">${initial}</text></svg>`)}`;
	}

	// Function to handle account deletion
	async function handleDeleteAccount() {
		if (deleteConfirmationText !== 'DELETE') {
			return;
		}

		deletingAccount = true;

		try {
			// Call the deleteAccount API
			await privateRequest('deleteAccount', {
				confirmation: 'DELETE'
			});

			// If successful, logout and return to login page
			logout('/login');
		} catch (error) {
			console.error('Error deleting account:', error);
			errorMessage = 'Failed to delete account. Please try again.';
			showDeleteConfirmation = false;
			deletingAccount = false;
		}
	}
</script>

<div class="settings-panel">
	<!-- Full settings view for all authenticated users -->
	<div class="tabs">
		<button
			class="tab {activeTab === 'chart' ? 'active' : ''}"
			on:click={() => (activeTab = 'chart')}
		>
			Chart
		</button>
		<button
			class="tab {activeTab === 'format' ? 'active' : ''}"
			on:click={() => (activeTab = 'format')}
		>
			Format
		</button>
		<button
			class="tab {activeTab === 'account' ? 'active' : ''}"
			on:click={() => (activeTab = 'account')}
		>
			Account
		</button>
		<button
			class="tab {activeTab === 'appearance' ? 'active' : ''}"
			on:click={() => (activeTab = 'appearance')}
		>
			Appearance
		</button>
	</div>

	<div class="settings-content">
		<!-- Chart Settings Tab -->
		{#if activeTab === 'chart'}
			<div class="chart-settings">
				<h3>Chart Settings</h3>

				<div class="settings-section">
					<h4>Chart Layout</h4>
					<label class="setting-item">
						<span>Chart Rows:</span>
						<input
							type="number"
							bind:value={tempSettings.chartRows}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</label>

					<label class="setting-item">
						<span>Chart Columns:</span>
						<input
							type="number"
							bind:value={tempSettings.chartColumns}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</label>

					<label class="setting-item">
						<span>Show Dollar Volume:</span>
						<input type="checkbox" bind:checked={tempSettings.dolvol} />
					</label>

					<label class="setting-item">
						<span>Show SEC Filings:</span>
						<input type="checkbox" bind:checked={tempSettings.showFilings} />
					</label>
				</div>

				<div class="settings-section">
					<h4>Technical Indicators</h4>
					<label class="setting-item">
						<span>Average Range Period:</span>
						<input
							type="number"
							bind:value={tempSettings.adrPeriod}
							min="1"
							on:keypress={handleKeyPress}
						/>
					</label>
				</div>
			</div>
		{/if}

		<!-- Format Settings Tab -->
		{#if activeTab === 'format'}
			<div class="format-settings">
				<h3>Format Settings</h3>

				<div class="settings-section">
					<h4>Time & Sales</h4>
					<label class="setting-item">
						<span>Show trades less than 100 shares:</span>
						<input type="checkbox" bind:checked={tempSettings.filterTaS} />
					</label>

					<label class="setting-item">
						<span>Divide Time and Sales by 100:</span>
						<input type="checkbox" bind:checked={tempSettings.divideTaS} />
					</label>
				</div>
			</div>
		{/if}

		<!-- Account Settings Tab -->
		{#if activeTab === 'account'}
			<div class="account-settings">
				<h3>Account Settings</h3>

				<div class="profile-section">
					<div class="profile-picture-container">
						<div class="profile-picture">
							<img src={generateInitialAvatar(username)} alt="Profile" class="profile-image" />
						</div>
						<div class="username-display">
							{username}
						</div>
					</div>
				</div>

				<!-- Subscription Management -->
				<div class="subscription-section">
					<h4>Subscription</h4>
					{#if $subscriptionStatus.loading}
						<p>Loading subscription information...</p>
					{:else if $subscriptionStatus.error}
						<p class="error-text">{$subscriptionStatus.error}</p>
					{:else if $subscriptionStatus.isActive}
						<div class="subscription-info">
							<p class="subscription-status">Status: <span class="active">Active</span></p>
							{#if $subscriptionStatus.currentPlan}
								<p>Plan: {$subscriptionStatus.currentPlan}</p>
							{/if}
							{#if $subscriptionStatus.currentPeriodEnd}
								<p>
									Next billing: {new Date(
										$subscriptionStatus.currentPeriodEnd * 1000
									).toLocaleDateString()}
								</p>
							{/if}
							<button class="manage-subscription-button" on:click={handleManageSubscription}>
								Manage Subscription
							</button>
						</div>
					{:else}
						<div class="subscription-info">
							<p class="subscription-status">Status: <span class="inactive">Free Plan</span></p>
							<p>Upgrade to access premium features</p>
							<button class="upgrade-button" on:click={() => goto('/pricing')}> View Plans </button>
						</div>
					{/if}
				</div>

				<!-- Usage Information -->
				<div class="subscription-section">
					<h4>Usage & Limits</h4>
					{#if $subscriptionStatus.loading}
						<p>Loading usage information...</p>
					{:else if $subscriptionStatus.error}
						<p class="error-text">{$subscriptionStatus.error}</p>
					{:else}
						<div class="usage-info">
							<!-- Credits Section -->
							<div class="usage-section">
								<h5>Credits</h5>
								<div class="usage-item">
									<span class="usage-label">Total Credits:</span>
									<span class="usage-value">{$subscriptionStatus.totalCreditsRemaining || 0}</span>
								</div>
								<div class="usage-item">
									<span class="usage-label">Subscription Credits:</span>
									<span class="usage-value"
										>{$subscriptionStatus.subscriptionCreditsRemaining || 0}</span
									>
								</div>
								<div class="usage-item">
									<span class="usage-label">Purchased Credits:</span>
									<span class="usage-value"
										>{$subscriptionStatus.purchasedCreditsRemaining || 0}</span
									>
								</div>
								{#if $subscriptionStatus.isActive}
									<div class="usage-item">
										<span class="usage-label">Monthly Allocation:</span>
										<span class="usage-value"
											>{$subscriptionStatus.subscriptionCreditsAllocated || 0}</span
										>
									</div>
								{/if}
							</div>

							<!-- Alerts Section -->
							<div class="usage-section">
								<h5>Alerts</h5>
								<div class="usage-item">
									<span class="usage-label">Active Alerts:</span>
									<span class="usage-value">
										{$subscriptionStatus.activeAlerts || 0}
										{#if $subscriptionStatus.alertsLimit !== undefined}
											/ {$subscriptionStatus.alertsLimit}
										{/if}
									</span>
								</div>
								<div class="usage-item">
									<span class="usage-label">Strategy Alerts:</span>
									<span class="usage-value">
										{$subscriptionStatus.activeStrategyAlerts || 0}
										{#if $subscriptionStatus.strategyAlertsLimit !== undefined}
											/ {$subscriptionStatus.strategyAlertsLimit}
										{/if}
									</span>
								</div>
							</div>

							<!-- Purchase Credits Button -->
							{#if $subscriptionStatus.isActive}
								<button
									class="upgrade-button"
									on:click={() => goto('/pricing')}
									style="margin-top: 1rem;"
								>
									Purchase More Credits
								</button>
							{:else}
								<p class="upgrade-note">Upgrade to a paid plan to purchase additional credits</p>
							{/if}
						</div>
					{/if}
				</div>

				<!-- Account Actions -->
				<div class="account-actions">
					<button class="logout-button" on:click={() => logout('/')}>Logout</button>

					<!-- Delete Account Section -->
					<div class="danger-zone">
						<h4>Danger Zone</h4>
						<div class="delete-account-section">
							<p>Permanently delete your account and all associated data.</p>

							{#if !showDeleteConfirmation}
								<button class="delete-button" on:click={() => (showDeleteConfirmation = true)}>
									Delete Account
								</button>
							{:else}
								<div class="delete-confirmation">
									<p class="warning-text">
										⚠️ This action cannot be undone. All your data will be permanently deleted.
									</p>
									<p>Type <strong>DELETE</strong> to confirm:</p>
									<input
										type="text"
										bind:value={deleteConfirmationText}
										placeholder="Type DELETE here"
										class="delete-input"
									/>
									<div class="delete-buttons">
										<button
											class="cancel-button"
											on:click={() => {
												showDeleteConfirmation = false;
												deleteConfirmationText = '';
											}}
										>
											Cancel
										</button>
										<button
											class="confirm-delete-button"
											disabled={deleteConfirmationText !== 'DELETE' || deletingAccount}
											on:click={handleDeleteAccount}
										>
											{deletingAccount ? 'Deleting...' : 'Delete Account'}
										</button>
									</div>
								</div>
							{/if}
						</div>
					</div>
				</div>
			</div>
		{/if}

		<!-- Appearance Settings Tab -->
		{#if activeTab === 'appearance'}
			<div class="appearance-settings">
				<h3>Appearance Settings</h3>

				<div class="settings-section">
					<h4>Color Scheme</h4>
					<div class="color-scheme-grid">
						{#each Object.entries(colorSchemes) as [key, scheme]}
							<button
								class="color-scheme-option {tempSettings.colorScheme === key ? 'selected' : ''}"
								on:click={() => {
									tempSettings.colorScheme = key;
									applyColorScheme(key);
								}}
							>
								<div class="color-preview">
									<div class="color-bar" style="background-color: {scheme.primary}"></div>
									<div class="color-bar" style="background-color: {scheme.secondary}"></div>
									<div class="color-bar" style="background-color: {scheme.accent}"></div>
								</div>
								<span class="scheme-name">{scheme.name}</span>
							</button>
						{/each}
					</div>
				</div>
			</div>
		{/if}

		<!-- Settings Actions -->
		<div class="settings-actions">
			<button class="save-button" on:click={saveSettings}>Save Settings</button>
			<button class="reset-button" on:click={resetSettings}>Reset to Default</button>
		</div>
	</div>

	{#if errorMessage}
		<div class="error-message">{errorMessage}</div>
	{/if}
</div>

<style>
	.settings-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		background-color: var(--c1);
		color: var(--f1);
		font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
	}

	.tabs {
		display: flex;
		border-bottom: 1px solid var(--c3);
		background-color: var(--c2);
	}

	.tab {
		padding: 1rem 1.5rem;
		background: none;
		border: none;
		color: var(--f2);
		cursor: pointer;
		font-size: 0.9375rem;
		font-weight: 500;
		transition: all 0.2s ease;
		border-bottom: 3px solid transparent;
	}

	.tab:hover {
		background-color: rgba(255, 255, 255, 0.05);
		color: var(--f1);
	}

	.tab.active {
		color: var(--c3);
		border-bottom-color: var(--c3);
		background-color: rgba(255, 255, 255, 0.03);
	}

	.settings-content {
		flex: 1;
		padding: 2rem;
		overflow-y: auto;
	}

	.chart-settings,
	.format-settings,
	.account-settings,
	.appearance-settings {
		max-width: 600px;
	}

	.settings-section {
		margin-bottom: 2rem;
		padding: 1.5rem;
		background-color: rgba(255, 255, 255, 0.03);
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.08);
	}

	.settings-section h4 {
		margin: 0 0 1rem 0;
		color: var(--f1);
		font-size: 1rem;
		font-weight: 600;
	}

	.setting-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
		font-size: 0.9375rem;
	}

	.setting-item span {
		color: var(--f2);
		flex-grow: 1;
	}

	.setting-item select,
	.setting-item input[type='number'] {
		padding: 0.5rem;
		background-color: var(--c2);
		border: 1px solid var(--c3);
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.875rem;
		min-width: 120px;
	}

	.setting-item input[type='checkbox'] {
		width: 18px;
		height: 18px;
		accent-color: var(--c3);
	}

	.profile-section {
		margin-bottom: 2rem;
		padding: 1.5rem;
		background-color: rgba(255, 255, 255, 0.03);
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.08);
		text-align: center;
	}

	.profile-picture-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}

	.profile-picture {
		width: 80px;
		height: 80px;
		border-radius: 50%;
		overflow: hidden;
		border: 3px solid var(--c3);
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.profile-image {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.username-display {
		font-size: 1.25rem;
		font-weight: 600;
		color: var(--f1);
	}

	.subscription-section {
		margin-bottom: 2rem;
		padding: 1.5rem;
		background-color: rgba(255, 255, 255, 0.03);
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.08);
	}

	.subscription-section h4 {
		margin: 0 0 1rem 0;
		color: var(--f1);
		font-size: 1rem;
		font-weight: 600;
	}

	.subscription-info {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.subscription-status {
		font-weight: 600;
	}

	.subscription-status .active {
		color: var(--success-color, #10b981);
	}

	.subscription-status .inactive {
		color: var(--warning-color, #f59e0b);
	}

	.manage-subscription-button,
	.upgrade-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--c3);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		align-self: flex-start;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
	}

	.manage-subscription-button:hover,
	.upgrade-button:hover {
		background-color: var(--c3-hover);
	}

	.account-actions {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.logout-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--secondary-button-bg, #6b7280);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		align-self: flex-start;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
	}

	.logout-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.danger-zone {
		margin-top: 2rem;
		padding: 1.5rem;
		background-color: rgba(239, 68, 68, 0.05);
		border-radius: 6px;
		text-align: center;
		border: 1px solid rgba(239, 68, 68, 0.1);
	}

	.delete-account-section {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}

	.delete-button {
		padding: 0.75rem 1.5rem;
		background-color: #dc2626;
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
	}

	.delete-button:hover {
		background-color: #b91c1c;
	}

	.delete-confirmation {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
		width: 100%;
		max-width: 400px;
	}

	.warning-text {
		color: #fbbf24;
		font-weight: 600;
		text-align: center;
	}

	.delete-input {
		padding: 0.75rem;
		background-color: var(--c2);
		border: 1px solid #dc2626;
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.9375rem;
		width: 100%;
		text-align: center;
	}

	.delete-buttons {
		display: flex;
		gap: 1rem;
		justify-content: center;
	}

	.cancel-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--secondary-button-bg, #6b7280);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.cancel-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.confirm-delete-button {
		padding: 0.75rem 1.5rem;
		background-color: #dc2626;
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.confirm-delete-button:hover:not(:disabled) {
		background-color: #b91c1c;
	}

	.confirm-delete-button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.color-scheme-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
		gap: 1rem;
	}

	.color-scheme-option {
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: 1rem;
		background-color: var(--c2);
		border: 2px solid transparent;
		border-radius: 8px;
		cursor: pointer;
		transition: all 0.2s ease;
		gap: 0.75rem;
	}

	.color-scheme-option:hover {
		background-color: rgba(255, 255, 255, 0.05);
	}

	.color-scheme-option.selected {
		border-color: var(--c3);
		background-color: rgba(255, 255, 255, 0.03);
	}

	.color-preview {
		display: flex;
		width: 60px;
		height: 20px;
		border-radius: 4px;
		overflow: hidden;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
	}

	.color-bar {
		flex: 1;
	}

	.scheme-name {
		font-size: 0.875rem;
		color: var(--f2);
		font-weight: 500;
	}

	.settings-actions {
		display: flex;
		gap: 1rem;
		margin-top: 2rem;
		padding-top: 2rem;
		border-top: 1px solid var(--c3);
	}

	.save-button,
	.reset-button {
		padding: 0.75rem 1.5rem;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
	}

	.save-button {
		background-color: var(--c3);
		color: white;
	}

	.save-button:hover {
		background-color: var(--c3-hover);
	}

	.reset-button {
		background-color: var(--secondary-button-bg, #6b7280);
		color: white;
	}

	.reset-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.error-message {
		margin-top: 1rem;
		padding: 1rem;
		background-color: rgba(239, 68, 68, 0.1);
		border: 1px solid rgba(239, 68, 68, 0.3);
		border-radius: 4px;
		color: #fca5a5;
		font-size: 0.875rem;
		text-align: center;
	}

	.error-text {
		color: #fca5a5;
	}

	h3 {
		margin: 0 0 1.5rem 0;
		color: var(--f1);
		font-size: 1.5rem;
		font-weight: 600;
	}

	/* Credits Information Styles */
	.credits-info {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.credit-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.5rem 0;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
	}

	.credit-item:last-child {
		border-bottom: none;
	}

	.credit-label {
		color: var(--f2);
		font-size: 0.9375rem;
	}

	.credit-value {
		color: var(--f1);
		font-weight: 600;
		font-size: 1rem;
	}

	.upgrade-note {
		color: var(--f2);
		font-size: 0.875rem;
		font-style: italic;
		margin-top: 0.5rem;
	}

	/* Usage Information Styles */
	.usage-info {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.usage-section {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.usage-section h5 {
		margin: 0;
		color: var(--f1);
		font-size: 0.9375rem;
		font-weight: 600;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		padding-bottom: 0.5rem;
	}

	.usage-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.5rem 0;
		border-bottom: 1px solid rgba(255, 255, 255, 0.05);
	}

	.usage-item:last-child {
		border-bottom: none;
	}

	.usage-label {
		color: var(--f2);
		font-size: 0.9375rem;
	}

	.usage-value {
		color: var(--f1);
		font-weight: 600;
		font-size: 1rem;
	}
</style>
