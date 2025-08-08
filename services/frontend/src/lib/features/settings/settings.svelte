<script lang="ts">
	import { settings } from '$lib/utils/stores/stores';
	import { get } from 'svelte/store';
	import type { Settings } from '$lib/utils/types/types';
	import '$lib/styles/global.css';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { colorSchemes, applyColorScheme } from '$lib/styles/colorSchemes';
	import { logout } from '$lib/auth';
	import { subscriptionStatus, fetchCombinedSubscriptionAndUsage } from '$lib/utils/stores/stores';
	import { privateRequest } from '$lib/utils/helpers/backend';

	// Export initialTab prop to handle external tab selection
	export let initialTab: 'interface' | 'account' | 'appearance' | 'usage' | 'chart' | 'format' =
		'interface';

	let errorMessage: string = '';
	let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with
	let originalSettings: Settings = { ...get(settings) }; // Store original settings for comparison
	let hasChanges = false; // Track if settings have been modified

	// Map old tab names to new ones for backward compatibility
	function mapTabName(tab: string): 'interface' | 'account' | 'appearance' | 'usage' {
		switch (tab) {
			case 'chart':
			case 'format':
				return 'interface';
			case 'account':
				return 'account';
			case 'appearance':
				return 'appearance';
			case 'usage':
				return 'usage';
			default:
				return 'interface';
		}
	}

	let activeTab: 'interface' | 'account' | 'appearance' | 'usage' = mapTabName(initialTab);

	// Delete account variables
	let showDeleteConfirmation = false;
	let deleteConfirmationText = '';
	let deletingAccount = false;

	// Cancel subscription variables
	let cancelingSubscription = false;
	let showCancelConfirmation = false;
	let cancelConfirmationText = '';

	// Handle manage subscription
	function handleManageSubscription() {
		goto('/pricing');
	}

	// Initialize component
	async function initializeComponent() {
		await fetchCombinedSubscriptionAndUsage();
	}

	// Run initialization on mount
	onMount(() => {
		initializeComponent();
	});

	// Function to check if settings have changed
	function checkForChanges() {
		hasChanges = JSON.stringify(tempSettings) !== JSON.stringify(originalSettings);
	}

	function saveSettings() {
		privateRequest<void>('setSettings', { settings: tempSettings }).then(() => {
			settings.set(tempSettings);
			originalSettings = { ...tempSettings }; // Update original settings after save
			hasChanges = false; // Reset change flag
			errorMessage = '';
		});
	}

	function resetSettings() {
		tempSettings = { ...originalSettings }; // Reset to original settings
		hasChanges = false; // Reset change flag
		saveSettings();
	}

	function handleKeyPress(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			saveSettings();
		}
	}

	// Function to handle subscription cancellation
	async function handleCancelSubscription() {
		if (cancelConfirmationText !== 'CANCEL') {
			return;
		}

		cancelingSubscription = true;
		errorMessage = '';

		try {
			await privateRequest('cancelSubscription', {});

			// Refresh subscription status to reflect the cancellation
			await fetchCombinedSubscriptionAndUsage();

			// Reset confirmation state
			showCancelConfirmation = false;
			cancelConfirmationText = '';
		} catch (error) {
			console.error('Error canceling subscription:', error);
			errorMessage = 'Failed to cancel subscription. Please try again.';
		} finally {
			cancelingSubscription = false;
		}
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
			class="tab {activeTab === 'interface' ? 'active' : ''}"
			on:click={() => (activeTab = 'interface')}
		>
			Interface
		</button>
		<button
			class="tab {activeTab === 'usage' ? 'active' : ''}"
			on:click={() => (activeTab = 'usage')}
		>
			Usage
		</button>
		<button
			class="tab {activeTab === 'account' ? 'active' : ''}"
			on:click={() => (activeTab = 'account')}
		>
			Account
		</button>
		<!-- <button
			class="tab {activeTab === 'appearance' ? 'active' : ''}"
			on:click={() => (activeTab = 'appearance')}
		>
			Appearance
		</button> -->
	</div>

	<div class="settings-content">
		<!-- Interface Settings Tab -->
		{#if activeTab === 'interface'}
			<div class="interface-settings">
				<h3>Interface Settings</h3>

				<div class="settings-section">
					<h4>Chart</h4>
					<label class="setting-item">
						<span>Show Dollar Volume:</span>
						<input type="checkbox" bind:checked={tempSettings.dolvol} on:change={checkForChanges} />
					</label>

					<label class="setting-item">
						<span>Show SEC Filings:</span>
						<input
							type="checkbox"
							bind:checked={tempSettings.showFilings}
							on:change={checkForChanges}
						/>
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
							on:change={checkForChanges}
						/>
					</label>
				</div>

				<!-- <div class="settings-section">
					<h4>Time & Sales</h4>
					<label class="setting-item">
						<span>Show trades less than 100 shares:</span>
						<input
							type="checkbox"
							bind:checked={tempSettings.filterTaS}
							on:change={checkForChanges}
						/>
					</label>

					<label class="setting-item">
						<span>Divide Time and Sales by 100:</span>
						<input
							type="checkbox"
							bind:checked={tempSettings.divideTaS}
							on:change={checkForChanges}
						/>
					</label>
				</div> -->
			</div>
		{/if}

		<!-- Account Settings Tab -->
		{#if activeTab === 'account'}
			<div class="account-settings">
				<h3>Account Settings</h3>

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
		<!-- {#if activeTab === 'appearance'}
			<div class="appearance-settings">
				<h3>Appearance Settings</h3>

				<div class="settings-section">
					<h4>Color Scheme</h4>
					<div class="color-scheme-grid">
						{#each Object.entries(colorSchemes) as [key, scheme]}
							<button
								class="color-scheme-option {tempSettings.colorScheme === key ? 'selected' : ''}"
								on:click={() => {
									tempSettings = { ...tempSettings, colorScheme: key };
									applyColorScheme(scheme);
								}}
							>
								<div class="color-preview">
									<div class="color-bar" style="background-color: {scheme.c1}"></div>
									<div class="color-bar" style="background-color: {scheme.c2}"></div>
									<div class="color-bar" style="background-color: {scheme.c3}"></div>
								</div>
								<span class="scheme-name">{scheme.name}</span>
							</button>
						{/each}
					</div>
				</div>
			</div>
		{/if} -->

		<!-- Usage Tab -->
		{#if activeTab === 'usage'}
			<div class="usage-settings">
				<h3>Usage</h3>
				<div class="usage-layout">
					<!-- Usage Information -->
					<div class="settings-section">
						<h4>Usage & Limits</h4>
						{#if $subscriptionStatus.loading}
							<p>Loading usage information...</p>
						{:else if $subscriptionStatus.error}
							<p class="error-text">{$subscriptionStatus.error}</p>
						{:else}
							<div class="usage-info">
								<!-- Queries Section -->
								<div class="usage-section">
									<h5>Queries</h5>
									<div class="usage-item">
										<span class="usage-label">Total Queries:</span>
										<span class="usage-value">{$subscriptionStatus.totalCreditsRemaining || 0}</span
										>
									</div>
									<div class="usage-item">
										<span class="usage-label">Subscription Queries:</span>
										<span class="usage-value"
											>{$subscriptionStatus.subscriptionCreditsRemaining || 0}</span
										>
									</div>
									<div class="usage-item">
										<span class="usage-label">Purchased Queries:</span>
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

								<!-- Purchase Queries Button -->
								{#if !$subscriptionStatus.isActive}
									<p class="upgrade-note">Upgrade to a paid plan to purchase additional queries</p>
								{/if}
							</div>
						{/if}
					</div>

					<!-- Subscription Management -->
					<div class="settings-section">
						<h4>Subscription</h4>
						{#if $subscriptionStatus.loading}
							<p>Loading subscription information...</p>
						{:else if $subscriptionStatus.error}
							<p class="error-text">{$subscriptionStatus.error}</p>
						{:else if $subscriptionStatus.isActive}
							<div class="subscription-info">
								{#if $subscriptionStatus.isCanceling}
									<p class="subscription-status">
										Status: <span class="canceling">Canceling</span>
									</p>
									{#if $subscriptionStatus.currentPlan}
										<p>Plan: {$subscriptionStatus.currentPlan}</p>
									{/if}
									{#if $subscriptionStatus.currentPeriodEnd}
										<p>
											Access until: {new Date(
												$subscriptionStatus.currentPeriodEnd * 1000
											).toLocaleDateString()}
										</p>
									{/if}
									<p class="canceling-note">
										Your subscription will remain active until the end of your current billing
										period.
									</p>
								{:else}
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
								{/if}
								<div class="subscription-buttons">
									<button class="manage-subscription-button" on:click={handleManageSubscription}>
										Manage Subscription
									</button>
									{#if !$subscriptionStatus.isCanceling}
										{#if !showCancelConfirmation}
											<button
												class="cancel-subscription-button"
												on:click={() => (showCancelConfirmation = true)}
											>
												Cancel Subscription
											</button>
										{:else}
											<div class="cancel-confirmation">
												<p class="warning-text">
													⚠️ This will cancel your subscription at the end of your current billing
													period.
												</p>
												<p>Type <strong>CANCEL</strong> to confirm:</p>
												<input
													type="text"
													bind:value={cancelConfirmationText}
													placeholder="Type CANCEL here"
													class="cancel-input"
												/>
												<div class="cancel-buttons">
													<button
														class="cancel-button"
														on:click={() => {
															showCancelConfirmation = false;
															cancelConfirmationText = '';
														}}
													>
														Keep Subscription
													</button>
													<button
														class="confirm-cancel-button"
														disabled={cancelConfirmationText !== 'CANCEL' || cancelingSubscription}
														on:click={handleCancelSubscription}
													>
														{cancelingSubscription ? 'Canceling...' : 'Cancel Subscription'}
													</button>
												</div>
											</div>
										{/if}
									{/if}
									<button class="upgrade-button" on:click={() => goto('/pricing')}>
										Purchase More Queries
									</button>
								</div>
							</div>
						{:else}
							<div class="subscription-info">
								<p class="subscription-status">Status: <span class="inactive">Free Plan</span></p>
								<p>Upgrade to access premium features</p>
								<button class="upgrade-button" on:click={() => goto('/pricing')}>
									View Plans
								</button>
							</div>
						{/if}
					</div>
				</div>
			</div>
		{/if}

		<!-- Settings Actions - Only show in Interface tab -->
		{#if activeTab === 'interface'}
			<div class="settings-actions">
				<button class="save-button {hasChanges ? 'has-changes' : ''}" on:click={saveSettings}
					>Save Settings</button
				>
				<button class="reset-button {hasChanges ? 'has-changes' : ''}" on:click={resetSettings}
					>Reset</button
				>
			</div>
		{/if}
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
		background-color: rgb(255 255 255 / 5%);
		color: var(--f1);
	}

	.tab.active {
		color: var(--f1);
		border-bottom-color: var(--f1);
		background-color: rgb(255 255 255 / 3%);
	}

	.settings-content {
		flex: 1;
		padding: 2rem;
		overflow-y: auto;
	}

	.interface-settings,
	.account-settings,
	.usage-settings {
		max-width: 600px;
	}

	.settings-section {
		margin-bottom: 2rem;
		padding: 1.5rem;
		background-color: rgb(255 255 255 / 3%);
		border-radius: 8px;
		border: 1px solid rgb(255 255 255 / 8%);
	}

	.settings-section h4 {
		margin: 0 0 1rem;
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

	.setting-item [type='number'] {
		padding: 0.5rem;
		background-color: var(--c2);
		border: 1px solid var(--c3);
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.875rem;
		min-width: 120px;
	}

	.setting-item [type='checkbox'] {
		width: 18px;
		height: 18px;
		accent-color: var(--f1);
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

	.subscription-status .canceling {
		color: var(--warning-color, #f59e0b);
	}

	.canceling-note {
		color: var(--warning-color, #f59e0b);
		font-size: 0.875rem;
		font-style: italic;
		margin-top: 0.5rem;
	}

	.subscription-buttons {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		align-items: flex-start;
	}

	.manage-subscription-button,
	.upgrade-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--secondary-button-bg, #6b7280);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		box-shadow: 0 2px 5px rgb(0 0 0 / 20%);
	}

	.manage-subscription-button:hover,
	.upgrade-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.cancel-subscription-button {
		padding: 0.75rem 1.5rem;
		background-color: #dc2626;
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		box-shadow: 0 2px 5px rgb(0 0 0 / 20%);
	}

	.cancel-subscription-button:hover:not(:disabled) {
		background-color: #b91c1c;
	}

	.cancel-subscription-button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.cancel-confirmation {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
		width: 100%;
		max-width: 400px;
		padding: 1rem;
		background-color: rgb(239 68 68 / 5%);
		border-radius: 6px;
		border: 1px solid rgb(239 68 68 / 10%);
	}

	.cancel-input {
		padding: 0.75rem;
		background-color: var(--c2);
		border: 1px solid #dc2626;
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.9375rem;
		width: 100%;
		text-align: center;
	}

	.cancel-buttons {
		display: flex;
		gap: 1rem;
		justify-content: center;
	}

	.confirm-cancel-button {
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

	.confirm-cancel-button:hover:not(:disabled) {
		background-color: #b91c1c;
	}

	.confirm-cancel-button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
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
		box-shadow: 0 2px 5px rgb(0 0 0 / 20%);
	}

	.logout-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.danger-zone {
		margin-top: 2rem;
		padding: 1.5rem;
		background-color: rgb(239 68 68 / 5%);
		border-radius: 6px;
		text-align: center;
		border: 1px solid rgb(239 68 68 / 10%);
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
		box-shadow: 0 2px 5px rgb(0 0 0 / 20%);
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

	.settings-actions {
		display: flex;
		gap: 1rem;
		margin-top: 2rem;
		padding-top: 2rem;
		border-top: 1px solid rgb(255 255 255 / 8%);
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
		box-shadow: 0 2px 5px rgb(0 0 0 / 20%);
	}

	.save-button {
		background-color: var(--secondary-button-bg, #6b7280);
		color: white;
		transition: background-color 0.2s;
	}

	.save-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.save-button.has-changes {
		background-color: #3b82f6;
	}

	.save-button.has-changes:hover {
		background-color: #2563eb;
	}

	.reset-button {
		background-color: var(--secondary-button-bg, #6b7280);
		color: white;
		transition: background-color 0.2s;
	}

	.reset-button:hover {
		background-color: var(--secondary-button-hover, #4b5563);
	}

	.reset-button.has-changes {
		background-color: #3b82f6;
	}

	.reset-button.has-changes:hover {
		background-color: #2563eb;
	}

	.error-message {
		margin-top: 1rem;
		padding: 1rem;
		background-color: rgb(239 68 68 / 10%);
		border: 1px solid rgb(239 68 68 / 30%);
		border-radius: 4px;
		color: #fca5a5;
		font-size: 0.875rem;
		text-align: center;
	}

	.error-text {
		color: #fca5a5;
	}

	h3 {
		margin: 0 0 1.5rem;
		color: var(--f1);
		font-size: 1.5rem;
		font-weight: 600;
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
		border-bottom: 1px solid rgb(255 255 255 / 10%);
		padding-bottom: 0.5rem;
	}

	.usage-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.5rem 0;
		border-bottom: 1px solid rgb(255 255 255 / 5%);
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

	.usage-settings {
		max-width: 1200px;
	}

	.usage-layout {
		display: flex;
		flex-wrap: wrap;
		gap: 2rem;
	}

	.usage-layout > .settings-section {
		flex: 1;
		min-width: 300px;
	}
</style>
