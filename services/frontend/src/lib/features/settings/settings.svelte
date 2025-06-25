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

	let errorMessage: string = '';
	let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with
	let activeTab: 'chart' | 'format' | 'account' | 'appearance' = 'chart';
	let watchlists: Array<{ watchlistId: string; watchlistName: string }> = [];
	let customTickers = ''; // For managing comma-separated list of tickers

	// Add profile picture state
	let profilePic = browser ? sessionStorage.getItem('profilePic') || '' : '';
	let username = browser ? sessionStorage.getItem('username') || '' : '';
	let newProfilePic = '';
	let uploadedImage: File | null = null;
	let previewUrl = '';
	let uploadStatus = '';

	// Delete account variables
	let showDeleteConfirmation = false;
	let deleteConfirmationText = '';
	let deletingAccount = false;

	// Function to determine if the current user is a guest
	const isGuestAccount = (): boolean => {
		return username === 'Guest';
	};

	// DEPRECATED: Screensaver settings initialization
	// Initialize timeframes as a comma-separated string for editing
	// let timeframesString = tempSettings.screensaverTimeframes?.join(',') || '1w,1d,1h,1';

	onMount(() => {
		// DEPRECATED: Load watchlists for the screensaver settings
		// privateRequest<Array<{ watchlistId: string; watchlistName: string }>>('getWatchlists', {}).then(
		// 	(response) => {
		// 		watchlists = response || [];
		// 	}
		// );
		// DEPRECATED: Initialize custom tickers string if available
		// if (tempSettings.screensaverTickers && tempSettings.screensaverTickers.length > 0) {
		// 	customTickers = tempSettings.screensaverTickers.join(',');
		// }
		// Apply the current color scheme on mount using the store value
		// if ($settings.colorScheme && browser) {
		// 	const scheme = colorSchemes[$settings.colorScheme];
		// 	if (scheme) {
		// 		applyColorScheme(scheme);
		// 	}
		// }
	});

	function updateLayout() {
		// DEPRECATED: Screensaver timeframes and tickers update
		// Update timeframes array from the comma-separated string
		// if (timeframesString) {
		// 	tempSettings.screensaverTimeframes = timeframesString
		// 		.split(',')
		// 		.map((tf) => tf.trim())
		// 		.filter((tf) => tf.length > 0);
		// }

		// Update custom tickers if user-defined is selected
		// if (tempSettings.screensaverDataSource === 'user-defined' && customTickers) {
		// 	tempSettings.screensaverTickers = customTickers
		// 		.split(',')
		// 		.map((ticker) => ticker.trim().toUpperCase())
		// 		.filter((ticker) => ticker.length > 0);
		// }

		if (tempSettings.chartRows > 0 && tempSettings.chartColumns > 0) {
			privateRequest<void>('setSettings', { settings: tempSettings }).then(() => {
				settings.set(tempSettings); // Update the store with new settings

				// Apply color scheme if changed - REMOVED FROM HERE, handled by reactive statement
				// if (browser && tempSettings.colorScheme) {
				// 	const scheme = colorSchemes[tempSettings.colorScheme];
				// 	if (scheme) {
				// 		applyColorScheme(scheme);
				// 	}
				// }

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

	// Function to navigate to the signup page
	function goToSignup() {
		goto('/signup');
	}

	// Generate initial avatar SVG from username
	function generateInitialAvatar(username: string) {
		const initial = username.charAt(0).toUpperCase();
		return `data:image/svg+xml,${encodeURIComponent(`<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="50" fill="#1a1c21"/><text x="50" y="65" font-family="Arial" font-size="40" fill="#e0e0e0" text-anchor="middle" font-weight="bold">${initial}</text></svg>`)}`;
	}

	// Handle file selection for profile picture upload
	function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		if (input.files && input.files.length > 0) {
			uploadedImage = input.files[0];

			// Validate file type
			if (!uploadedImage.type.match('image.*')) {
				uploadStatus = 'Please select an image file';
				previewUrl = '';
				return;
			}

			// Create a preview URL
			previewUrl = URL.createObjectURL(uploadedImage);
			uploadStatus = '';
		}
	}

	// Update profile picture
	async function updateProfilePicture() {
		if (!uploadedImage) {
			uploadStatus = 'Please select an image to upload';
			return;
		}

		try {
			uploadStatus = 'Uploading...';

			// Convert image to base64
			const reader = new FileReader();
			reader.readAsDataURL(uploadedImage);
			reader.onload = async () => {
				const base64String = reader.result as string;

				try {
					// Send to backend
					await privateRequest('updateProfilePicture', {
						profilePicture: base64String
					});

					// Update in session storage
					if (browser) {
						sessionStorage.setItem('profilePic', base64String);
					}

					profilePic = base64String;
					uploadStatus = 'Profile picture updated successfully!';

					// Clear file input
					uploadedImage = null;
				} catch (error) {
					console.error('Error uploading profile picture:', error);
					uploadStatus = 'Failed to update profile picture. Please try again.';
				}
			};
		} catch (error) {
			console.error('Error processing image:', error);
			uploadStatus = 'Error processing image. Please try again.';
		}
	}

	// Reset to default initial avatar
	function resetToDefaultAvatar() {
		if (username) {
			const defaultAvatar = generateInitialAvatar(username);
			profilePic = defaultAvatar;

			if (browser) {
				sessionStorage.setItem('profilePic', defaultAvatar);
			}

			// Clear uploaded state
			uploadedImage = null;
			previewUrl = '';
			uploadStatus = 'Reset to default avatar';
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
			if (browser) {
				sessionStorage.removeItem('authToken');
				sessionStorage.removeItem('profilePic');
				sessionStorage.removeItem('username');
				sessionStorage.removeItem('isGuestSession');
			}

			// Redirect to login page
			goto('/login');
		} catch (error) {
			console.error('Error deleting account:', error);
			errorMessage = 'Failed to delete account. Please try again.';
			showDeleteConfirmation = false;
			deletingAccount = false;
		}
	}
</script>

<div class="settings-panel">
	{#if isGuestAccount()}
		<!-- Simplified view for guest users -->
		<div class="settings-header">
			<h2>Settings</h2>
		</div>
		<div class="settings-content guest-only-content">
			<div class="profile-section guest-profile">
				<div class="profile-picture-container">
					<div class="profile-picture">
						<img src={generateInitialAvatar(username)} alt="Profile" class="profile-image" />
					</div>
					<div class="username-display">
						{username}
					</div>
				</div>

				<div class="guest-account-notice">
					<p>
						You're currently using a guest account. Create your own account to save your preferences
						and data.
					</p>
					<button class="create-account-button" on:click={goToSignup}> Create Your Account </button>
				</div>
			</div>

			<div class="account-actions">
				<button class="logout-button" on:click={handleLogout}>Logout</button>
			</div>
		</div>
	{:else}
		<!-- Full settings panel for registered users -->
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
			<!-- DEPRECATED: Screensaver tab -->
			<!-- <button
				class="tab-button {activeTab === 'screensaver' ? 'active' : ''}"
				on:click={() => (activeTab = 'screensaver')}
			>
				Screensaver
			</button> -->
			<button
				class="tab-button {activeTab === 'appearance' ? 'active' : ''}"
				on:click={() => (activeTab = 'appearance')}
			>
				Appearance
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

						<!-- DEPRECATED: Screensaver enable/disable setting -->
						<!-- <div class="setting-item">
							<label for="enableScreensaver">Enable Screensaver</label>
							<div class="toggle-container">
								<select
									id="enableScreensaver"
									bind:value={tempSettings.enableScreensaver}
									on:keypress={handleKeyPress}
								>
									<option value={true}>Yes</option>
									<option value={false}>No</option>
								</select>
							</div>
						</div> -->
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
				<!-- DEPRECATED: Screensaver settings section -->
				<!-- {:else if activeTab === 'screensaver'}
				<div class="settings-section">
					<h3>Screensaver Settings</h3>

					<div class="setting-item">
						<label for="enableScreensaver">Enable Screensaver</label>
						<div class="toggle-container">
							<select
								id="enableScreensaver"
								bind:value={tempSettings.enableScreensaver}
								on:keypress={handleKeyPress}
							>
								<option value={true}>Yes</option>
								<option value={false}>No</option>
							</select>
						</div>
					</div>

					<div class="settings-grid">
						<div class="setting-item">
							<label for="screensaverTimeout">Inactivity Timeout (seconds)</label>
							<input
								type="number"
								id="screensaverTimeout"
								bind:value={tempSettings.screensaverTimeout}
								min="30"
								on:keypress={handleKeyPress}
							/>
						</div>

						<div class="setting-item">
							<label for="screensaverSpeed">Cycle Speed (seconds)</label>
							<input
								type="number"
								id="screensaverSpeed"
								bind:value={tempSettings.screensaverSpeed}
								min="1"
								on:keypress={handleKeyPress}
							/>
						</div>
					</div>

					<div class="setting-item wide">
						<label for="screensaverTimeframes">Timeframes (comma-separated)</label>
						<input
							type="text"
							id="screensaverTimeframes"
							bind:value={timeframesString}
							placeholder="1w,1d,1h,1"
							on:keypress={handleKeyPress}
						/>
					</div>

					<h3>Data Source</h3>
					<div class="setting-item">
						<label for="screensaverDataSource">Chart Data Source</label>
						<div class="toggle-container">
							<select
								id="screensaverDataSource"
								bind:value={tempSettings.screensaverDataSource}
								on:keypress={handleKeyPress}
							>
								<option value="gainers-losers">Gainers & Losers</option>
								<option value="watchlist">Watchlist</option>
								<option value="user-defined">Custom Tickers</option>
							</select>
						</div>
					</div>

					{#if tempSettings.screensaverDataSource === 'watchlist'}
						<div class="setting-item">
							<label for="screensaverWatchlistId">Select Watchlist</label>
							<div class="toggle-container">
								<select
									id="screensaverWatchlistId"
									bind:value={tempSettings.screensaverWatchlistId}
									on:keypress={handleKeyPress}
								>
									{#each watchlists as watchlist}
										<option value={watchlist.watchlistId}>{watchlist.watchlistName}</option>
									{/each}
								</select>
							</div>
						</div>
					{:else if tempSettings.screensaverDataSource === 'user-defined'}
						<div class="setting-item wide">
							<label for="customTickers">Custom Tickers (comma-separated)</label>
							<input
								type="text"
								id="customTickers"
								bind:value={customTickers}
								placeholder="AAPL,MSFT,GOOGL,AMZN"
								on:keypress={handleKeyPress}
							/>
						</div>
					{/if}

					<div class="info-message screensaver-info">
						{#if tempSettings.enableScreensaver}
							The screensaver will activate after {tempSettings.screensaverTimeout} seconds of inactivity,
							cycling through charts every {tempSettings.screensaverSpeed} seconds.
						{:else}
							The screensaver is currently disabled. Enable it to display trending charts during
							idle periods.
						{/if}
					</div>
				</div> -->
			{:else if activeTab === 'appearance'}
				<div class="settings-section">
					<h3>Color Scheme</h3>
					<div class="setting-item wide">
						<label for="colorScheme">Choose a color scheme</label>
						<div class="toggle-container">
							<select
								id="colorScheme"
								bind:value={tempSettings.colorScheme}
								on:keypress={handleKeyPress}
							>
								<option value="default">Default</option>
								<option value="dark-blue">Dark Blue</option>
								<option value="midnight">Midnight</option>
								<option value="forest">Forest</option>
								<option value="sunset">Sunset</option>
								<option value="grayscale">Grayscale</option>
							</select>
						</div>
					</div>

					<!-- Color scheme preview -->
					<div class="color-scheme-preview">
						<h4>Preview</h4>
						<div
							class="color-preview-container"
							style="background-color: {colorSchemes[tempSettings.colorScheme].c2};"
						>
							<div
								class="preview-header"
								style="background-color: {colorSchemes[tempSettings.colorScheme]
									.c1}; border-bottom: 1px solid {colorSchemes[tempSettings.colorScheme].c3};"
							>
								<div
									class="preview-title"
									style="color: {colorSchemes[tempSettings.colorScheme].f1};"
								>
									Chart Window
								</div>
							</div>
							<div class="preview-content">
								<div
									class="preview-section"
									style="background-color: {colorSchemes[tempSettings.colorScheme]
										.c2}; border: 1px solid {colorSchemes[tempSettings.colorScheme].c4};"
								>
									<div
										class="preview-text"
										style="color: {colorSchemes[tempSettings.colorScheme].f1};"
									>
										Primary Text
									</div>
									<div
										class="preview-text"
										style="color: {colorSchemes[tempSettings.colorScheme].f2};"
									>
										Secondary Text
									</div>
								</div>
								<div class="preview-buttons">
									<button
										class="preview-button"
										style="background-color: {colorSchemes[tempSettings.colorScheme]
											.c3}; color: white;"
									>
										Action Button
									</button>
									<div class="preview-indicators">
										<span style="color: {colorSchemes[tempSettings.colorScheme].colorUp};"
											>▲ +2.45%</span
										>
										<span style="color: {colorSchemes[tempSettings.colorScheme].colorDown};"
											>▼ -1.23%</span
										>
									</div>
								</div>
							</div>
						</div>
					</div>

					<div class="info-message">
						Color scheme changes will be applied immediately but must be saved using the "Apply
						Changes" button to persist across sessions.
					</div>
				</div>
			{:else if activeTab === 'account'}
				<div class="settings-section">
					<h3>Account Information</h3>

					<div class="profile-section">
						<div class="profile-picture-container">
							<div class="profile-picture">
								{#if profilePic}
									<img
										src={profilePic}
										alt="Profile"
										class="profile-image"
										on:error={() => {
											profilePic = generateInitialAvatar(username);
										}}
									/>
								{:else if username}
									<img src={generateInitialAvatar(username)} alt="Profile" class="profile-image" />
								{:else}
									<div class="profile-placeholder">?</div>
								{/if}
							</div>
							<div class="username-display">
								{username || 'User'}
							</div>
						</div>

						<div class="profile-upload-section">
							<h4>Update Profile Picture</h4>

							<div class="file-upload">
								<label for="profile-pic-upload" class="file-upload-label"> Choose Image </label>
								<input
									type="file"
									id="profile-pic-upload"
									accept="image/*"
									on:change={handleFileSelect}
								/>

								{#if previewUrl}
									<div class="preview-container">
										<img src={previewUrl} alt="Preview" class="preview-image" />
									</div>
								{/if}

								<div class="upload-actions">
									<button
										class="upload-button"
										on:click={updateProfilePicture}
										disabled={!uploadedImage}
									>
										Upload
									</button>

									<button class="reset-button" on:click={resetToDefaultAvatar}>
										Reset to Default
									</button>
								</div>

								{#if uploadStatus}
									<div class="upload-status">
										{uploadStatus}
									</div>
								{/if}
							</div>
						</div>
					</div>

					<div class="account-actions">
						<button class="logout-button" on:click={handleLogout}>Logout</button>
					</div>

					<div class="danger-zone">
						<h3>Danger Zone</h3>
						<div class="delete-account-section">
							<div class="warning-message">
								<p>
									Permanently delete your account and all associated data. This action cannot be
									undone.
								</p>
							</div>

							<button
								class="delete-account-button"
								on:click={() => (showDeleteConfirmation = true)}
							>
								Delete Account
							</button>
						</div>
					</div>
				</div>
			{/if}

			{#if errorMessage}
				<div class="error-message">{errorMessage}</div>
			{/if}

			<div class="settings-actions">
				<button class="apply-button" on:click={updateLayout}>Apply Changes</button>
			</div>

			{#if showDeleteConfirmation}
				<div class="confirmation-modal-overlay">
					<div class="confirmation-modal">
						<h3>Delete Account</h3>
						<p>
							This will permanently delete your account and all associated data. This action cannot
							be undone.
						</p>
						<p class="confirmation-instruction">
							Type <strong>DELETE</strong> to confirm.
						</p>

						<input
							type="text"
							bind:value={deleteConfirmationText}
							placeholder="Type DELETE to confirm"
							class="confirmation-input"
						/>

						<div class="confirmation-actions">
							<button class="cancel-button" on:click={() => (showDeleteConfirmation = false)}>
								Cancel
							</button>
							<button
								class="confirm-delete-button"
								disabled={deleteConfirmationText !== 'DELETE'}
								on:click={handleDeleteAccount}
							>
								{#if deletingAccount}
									<span class="loader"></span>
								{:else}
									Delete My Account
								{/if}
							</button>
						</div>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.settings-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		width: 100%;
		max-width: 95vw;
		max-height: 92vh;
		color: var(--f1);
		background-color: var(--c1);
		border-radius: 8px;
		overflow: hidden;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.6);
		margin: 0 auto;
		position: relative;
		min-width: 800px;
		min-height: 600px;
	}

	.settings-tabs {
		display: flex;
		background-color: var(--c2);
		border-bottom: 1px solid var(--c3);
		height: 56px;
		min-height: 56px;
	}

	.tab-button {
		padding: 0 20px;
		background: transparent;
		border: none;
		color: var(--f2);
		font-size: 0.9375rem;
		cursor: pointer;
		transition: all 0.2s ease;
		position: relative;
		text-align: center;
		height: 100%;
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 500;
		letter-spacing: 0.2px;
	}

	.tab-button:hover {
		background-color: rgba(255, 255, 255, 0.05);
		color: var(--f1);
	}

	.tab-button.active {
		color: var(--f1);
		font-weight: 600;
	}

	.tab-button.active::after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 100%;
		height: 3px;
		background-color: var(--c3);
	}

	.settings-content {
		flex: 1;
		padding: 1.75rem 2rem;
		overflow-y: auto;
		overflow-x: hidden;
	}

	.settings-section {
		margin-bottom: 2rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.1);
		padding-bottom: 1.75rem;
	}

	.settings-section:last-child {
		border-bottom: none;
		margin-bottom: 0;
		padding-bottom: 0;
	}

	h3 {
		margin: 0 0 1.25rem 0;
		font-size: 1.125rem;
		font-weight: 600;
		color: var(--f1);
		position: relative;
		padding-bottom: 0.5rem;
	}

	h3::after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 40px;
		height: 2px;
		background-color: var(--c3);
	}

	.settings-grid {
		display: grid;
		grid-template-columns: 1fr;
		gap: 0.875rem;
	}

	@media (min-width: 600px) {
		.settings-grid {
			grid-template-columns: 1fr 1fr;
			gap: 1.25rem;
		}
	}

	@media (min-width: 1200px) {
		.settings-grid {
			grid-template-columns: 1fr 1fr 1fr;
			gap: 1.5rem;
		}
	}

	.setting-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem 1.25rem;
		background-color: rgba(255, 255, 255, 0.03);
		border-radius: 6px;
		border: 1px solid rgba(255, 255, 255, 0.08);
		transition: border-color 0.2s ease;
	}

	.setting-item:hover {
		border-color: rgba(255, 255, 255, 0.12);
	}

	.setting-item.wide {
		grid-column: 1 / -1;
	}

	label {
		font-size: 0.9375rem;
		color: var(--f2);
		margin-right: 1rem;
		font-weight: 500;
	}

	input[type='number'] {
		width: 90px;
		padding: 0.625rem;
		background-color: rgba(0, 0, 0, 0.2);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.9375rem;
		text-align: center;
		transition: all 0.2s ease;
	}

	input[type='number']:focus {
		outline: none;
		border-color: var(--c3);
		box-shadow: 0 0 0 1px var(--c3);
	}

	input[type='text'] {
		padding: 0.625rem 0.875rem;
		background-color: rgba(0, 0, 0, 0.2);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.9375rem;
		min-width: 250px;
		transition: all 0.2s ease;
	}

	input[type='text']:focus {
		outline: none;
		border-color: var(--c3);
		box-shadow: 0 0 0 1px var(--c3);
	}

	.toggle-container select {
		padding: 0.625rem 0.875rem;
		background-color: rgba(0, 0, 0, 0.2);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.9375rem;
		min-width: 120px;
		cursor: pointer;
		transition: all 0.2s ease;
		appearance: none;
		background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='6' fill='none'%3E%3Cpath fill='%23999' d='M0 0h12L6 6 0 0z'/%3E%3C/svg%3E");
		background-repeat: no-repeat;
		background-position: right 12px center;
		padding-right: 36px;
	}

	.toggle-container select:focus {
		outline: none;
		border-color: var(--c3);
		box-shadow: 0 0 0 1px var(--c3);
	}

	.info-message {
		padding: 1.25rem;
		background-color: rgba(59, 130, 246, 0.05);
		border-radius: 6px;
		color: var(--f2);
		font-size: 0.9375rem;
		text-align: center;
		border: 1px solid rgba(59, 130, 246, 0.1);
		margin-top: 1.5rem;
	}

	.account-actions {
		margin-top: 2rem;
		display: flex;
		justify-content: center;
	}

	.error-message {
		margin: 1.25rem 0;
		padding: 1rem 1.25rem;
		background-color: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border-radius: 6px;
		font-size: 0.9375rem;
		text-align: center;
		border: 1px solid rgba(239, 68, 68, 0.2);
	}

	.settings-actions {
		margin-top: 2rem;
		display: flex;
		justify-content: flex-end;
		position: sticky;
		bottom: 0;
		background-color: var(--c1);
		padding: 1.25rem 0 0 0;
		border-top: 1px solid rgba(255, 255, 255, 0.05);
	}

	.apply-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--c3);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
	}

	.apply-button:hover {
		background-color: var(--c3-hover);
	}

	.logout-button {
		padding: 0.75rem 1.5rem;
		background-color: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border: 1px solid rgba(239, 68, 68, 0.3);
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
	}

	.logout-button:hover {
		background-color: rgba(239, 68, 68, 0.2);
	}

	/* Profile section styles */
	.profile-section {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
		margin-bottom: 1.5rem;
	}

	@media (min-width: 768px) {
		.profile-section {
			flex-direction: row;
			align-items: flex-start;
		}

		.profile-picture-container {
			flex: 0 0 auto;
		}

		.profile-upload-section {
			flex: 1;
		}
	}

	.profile-picture-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.875rem;
	}

	.profile-picture {
		width: 100px;
		height: 100px;
		border-radius: 50%;
		overflow: hidden;
		background-color: var(--c2);
		display: flex;
		align-items: center;
		justify-content: center;
		border: 2px solid var(--c3);
		box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
	}

	.profile-image {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.profile-placeholder {
		font-size: 2.5rem;
		color: var(--f2);
		font-weight: bold;
	}

	.username-display {
		font-size: 1.125rem;
		font-weight: 600;
		color: var(--f1);
	}

	.profile-upload-section {
		background-color: rgba(0, 0, 0, 0.1);
		border-radius: 6px;
		padding: 1.5rem;
		border: 1px solid rgba(255, 255, 255, 0.08);
	}

	.profile-upload-section h4 {
		margin: 0 0 1.25rem 0;
		font-size: 1rem;
		font-weight: 600;
		color: var(--f1);
	}

	.file-upload {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

	.file-upload input[type='file'] {
		display: none;
	}

	.file-upload-label {
		background-color: var(--c3);
		color: white;
		padding: 0.75rem 1.25rem;
		border-radius: 4px;
		cursor: pointer;
		text-align: center;
		font-size: 0.9375rem;
		font-weight: 500;
		transition: background-color 0.2s;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
	}

	.file-upload-label:hover {
		background-color: var(--c3-hover);
	}

	.preview-container {
		width: 100%;
		height: 140px;
		display: flex;
		justify-content: center;
		align-items: center;
		overflow: hidden;
		border-radius: 6px;
		background-color: rgba(0, 0, 0, 0.2);
		border: 1px solid rgba(255, 255, 255, 0.1);
	}

	.preview-image {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain;
	}

	.upload-actions {
		display: flex;
		gap: 1rem;
	}

	.upload-button,
	.reset-button {
		padding: 0.75rem 1.25rem;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		flex: 1;
		transition: all 0.2s;
	}

	.upload-button {
		background-color: var(--c3);
		color: white;
		border: none;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
	}

	.upload-button:hover:not(:disabled) {
		background-color: var(--c3-hover);
	}

	.upload-button:disabled {
		background-color: rgba(59, 130, 246, 0.3);
		cursor: not-allowed;
		box-shadow: none;
	}

	.reset-button {
		background-color: rgba(0, 0, 0, 0.2);
		color: var(--f1);
		border: 1px solid rgba(255, 255, 255, 0.1);
	}

	.reset-button:hover {
		background-color: rgba(0, 0, 0, 0.3);
	}

	.upload-status {
		padding: 0.875rem;
		text-align: center;
		font-size: 0.9375rem;
		color: var(--f2);
		background-color: rgba(0, 0, 0, 0.1);
		border-radius: 4px;
		border: 1px solid rgba(255, 255, 255, 0.08);
	}

	/* Color scheme preview styles */
	.color-scheme-preview {
		margin-top: 1.75rem;
		margin-bottom: 1.75rem;
	}

	.color-scheme-preview h4 {
		margin: 0 0 1rem 0;
		font-size: 1rem;
		font-weight: 600;
		color: var(--f1);
	}

	.color-preview-container {
		border-radius: 8px;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
		border: 1px solid rgba(255, 255, 255, 0.1);
	}

	.preview-header {
		padding: 0.875rem 1.25rem;
		display: flex;
		align-items: center;
	}

	.preview-title {
		font-size: 1rem;
		font-weight: 600;
	}

	.preview-content {
		padding: 1.5rem;
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}

	.preview-section {
		padding: 1.25rem;
		border-radius: 8px;
	}

	.preview-text {
		margin-bottom: 1rem;
		font-size: 0.9375rem;
	}

	.preview-buttons {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: center;
		gap: 1rem;
	}

	.preview-button {
		padding: 0.625rem 1.25rem;
		border: none;
		border-radius: 4px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
	}

	.preview-indicators {
		display: flex;
		gap: 1.25rem;
		font-size: 0.875rem;
		font-weight: 600;
	}

	.guest-account-notice {
		background-color: rgba(59, 130, 246, 0.05);
		border-radius: 6px;
		padding: 1.25rem;
		margin-bottom: 1.5rem;
		text-align: center;
		border: 1px solid rgba(59, 130, 246, 0.1);
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}

	.guest-account-notice p {
		color: var(--f2);
		margin: 0;
		font-size: 0.9375rem;
	}

	.create-account-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--c3);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
		box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
		width: fit-content;
	}

	.create-account-button:hover {
		background-color: var(--c3-hover);
	}

	/* Guest mode styles */
	.settings-header {
		padding: 1.5rem 2rem;
		border-bottom: 1px solid var(--c3);
		background-color: var(--c2);
	}

	.settings-header h2 {
		margin: 0;
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--f1);
	}

	.guest-only-content {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 2rem;
		height: 100%;
	}

	.guest-profile {
		width: 100%;
		max-width: 500px;
		margin: 0 auto;
		padding: 2rem;
		background-color: rgba(255, 255, 255, 0.03);
		border-radius: 8px;
		border: 1px solid rgba(255, 255, 255, 0.08);
		text-align: center;
	}

	.guest-account-notice {
		width: 100%;
		margin-top: 2rem;
		padding: 1.5rem;
		background-color: rgba(59, 130, 246, 0.05);
		border-radius: 6px;
		text-align: center;
		border: 1px solid rgba(59, 130, 246, 0.1);
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1.5rem;
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

	.warning-message {
		color: var(--f2);
		font-size: 0.9375rem;
	}

	.delete-account-button {
		padding: 0.75rem 1.5rem;
		background-color: rgba(239, 68, 68, 0.1);
		color: #ef4444;
		border: 1px solid rgba(239, 68, 68, 0.3);
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s;
	}

	.delete-account-button:hover {
		background-color: rgba(239, 68, 68, 0.2);
	}

	.confirmation-modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
		background-color: rgba(0, 0, 0, 0.5);
		display: flex;
		justify-content: center;
		align-items: center;
	}

	.confirmation-modal {
		background-color: var(--c1);
		padding: 2rem;
		border-radius: 8px;
		width: 100%;
		max-width: 400px;
	}

	.confirmation-modal h3 {
		margin-top: 0;
		margin-bottom: 1rem;
		font-size: 1.5rem;
		font-weight: 600;
		color: var(--f1);
	}

	.confirmation-instruction {
		margin-bottom: 1.5rem;
		font-size: 0.9375rem;
		color: var(--f2);
	}

	.confirmation-input {
		width: 100%;
		padding: 0.75rem;
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 4px;
		color: var(--f1);
		font-size: 0.9375rem;
	}

	.confirmation-actions {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}

	.cancel-button {
		padding: 0.75rem 1.5rem;
		background-color: rgba(255, 255, 255, 0.1);
		color: var(--f1);
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.cancel-button:hover {
		background-color: rgba(255, 255, 255, 0.2);
	}

	.confirm-delete-button {
		padding: 0.75rem 1.5rem;
		background-color: var(--c3);
		color: white;
		border: none;
		border-radius: 4px;
		font-size: 0.9375rem;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.confirm-delete-button:hover {
		background-color: var(--c3-hover);
	}

	.confirm-delete-button:disabled {
		background-color: rgba(59, 130, 246, 0.3);
		cursor: not-allowed;
	}

	.loader {
		border: 4px solid rgba(255, 255, 255, 0.3);
		border-top: 4px solid var(--c3);
		border-radius: 50%;
		width: 24px;
		height: 24px;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}
</style>
