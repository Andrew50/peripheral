<script lang="ts">
	import { settings } from '$lib/core/stores';
	import { get } from 'svelte/store';
	import type { Settings } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import '$lib/core/global.css';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';

	let errorMessage: string = '';
	let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with
	let activeTab: 'chart' | 'format' | 'account' | 'screensaver' = 'chart';
	let watchlists = [];
	let customTickers = ''; // For managing comma-separated list of tickers

	// Add profile picture state
	let profilePic = browser ? sessionStorage.getItem('profilePic') || '' : '';
	let username = browser ? sessionStorage.getItem('username') || '' : '';
	let newProfilePic = '';
	let uploadedImage: File | null = null;
	let previewUrl = '';
	let uploadStatus = '';

	// Initialize timeframes as a comma-separated string for editing
	let timeframesString = tempSettings.screensaverTimeframes?.join(',') || '1w,1d,1h,1';

	onMount(() => {
		// Load watchlists for the screensaver settings
		privateRequest('getWatchlists', {}).then((response) => {
			watchlists = response || [];
		});

		// Initialize custom tickers string if available
		if (tempSettings.screensaverTickers && tempSettings.screensaverTickers.length > 0) {
			customTickers = tempSettings.screensaverTickers.join(',');
		}
	});

	function updateLayout() {
		// Update timeframes array from the comma-separated string
		if (timeframesString) {
			tempSettings.screensaverTimeframes = timeframesString
				.split(',')
				.map((tf) => tf.trim())
				.filter((tf) => tf.length > 0);
		}

		// Update custom tickers if user-defined is selected
		if (tempSettings.screensaverDataSource === 'user-defined' && customTickers) {
			tempSettings.screensaverTickers = customTickers
				.split(',')
				.map((ticker) => ticker.trim().toUpperCase())
				.filter((ticker) => ticker.length > 0);
		}

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
		<button
			class="tab-button {activeTab === 'screensaver' ? 'active' : ''}"
			on:click={() => (activeTab = 'screensaver')}
		>
			Screensaver
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
		{:else if activeTab === 'screensaver'}
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
						The screensaver is currently disabled. Enable it to display trending charts during idle
						periods.
					{/if}
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
		margin-bottom: 10px;
	}

	.setting-item.wide {
		grid-column: 1 / -1;
	}

	.setting-item.wide input {
		width: 60%;
		max-width: 400px;
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

	input[type='text'] {
		padding: 6px 8px;
		background-color: var(--c1);
		border: 1px solid var(--c4);
		border-radius: 4px;
		color: var(--f1);
		font-size: 14px;
		min-width: 200px;
	}

	input[type='text']:focus {
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

	.screensaver-info {
		margin-top: 16px;
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

	/* Profile section styles */
	.profile-section {
		display: flex;
		flex-direction: column;
		gap: 20px;
		margin-bottom: 20px;
	}

	.profile-picture-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
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
	}

	.profile-image {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.profile-placeholder {
		font-size: 48px;
		color: var(--f2);
		font-weight: bold;
	}

	.username-display {
		font-size: 16px;
		font-weight: 500;
		color: var(--f1);
	}

	.profile-upload-section {
		background-color: var(--c2);
		border-radius: 4px;
		padding: 16px;
		border: 1px solid var(--c3);
	}

	.profile-upload-section h4 {
		margin: 0 0 12px 0;
		font-size: 14px;
		color: var(--f2);
	}

	.file-upload {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.file-upload input[type='file'] {
		display: none;
	}

	.file-upload-label {
		background-color: var(--c3);
		color: var(--f1);
		padding: 8px 12px;
		border-radius: 4px;
		cursor: pointer;
		text-align: center;
		font-size: 14px;
		transition: background-color 0.2s;
	}

	.file-upload-label:hover {
		background-color: var(--c4);
	}

	.preview-container {
		width: 100%;
		height: 120px;
		display: flex;
		justify-content: center;
		align-items: center;
		overflow: hidden;
		border-radius: 4px;
		background-color: var(--c1);
		border: 1px solid var(--c3);
	}

	.preview-image {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain;
	}

	.upload-actions {
		display: flex;
		gap: 10px;
	}

	.upload-button,
	.reset-button {
		padding: 8px 12px;
		border-radius: 4px;
		font-size: 14px;
		cursor: pointer;
		flex: 1;
		transition: background-color 0.2s;
	}

	.upload-button {
		background-color: #3b82f6;
		color: white;
		border: none;
	}

	.upload-button:hover:not(:disabled) {
		background-color: #2563eb;
	}

	.upload-button:disabled {
		background-color: #93c5fd;
		cursor: not-allowed;
	}

	.reset-button {
		background-color: var(--c3);
		color: var(--f1);
		border: 1px solid var(--c4);
	}

	.reset-button:hover {
		background-color: var(--c4);
	}

	.upload-status {
		padding: 8px;
		text-align: center;
		font-size: 14px;
		color: var(--f2);
		background-color: var(--c1);
		border-radius: 4px;
		border: 1px solid var(--c3);
	}
</style>
