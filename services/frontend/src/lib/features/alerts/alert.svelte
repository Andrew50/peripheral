<script lang="ts">
	/* ───── Imports ─────────────────────────────────────────────────────────── */
	import List from '$lib/components/list.svelte';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { writable, type Writable } from 'svelte/store';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { activeAlerts, inactiveAlerts, alertLogs, strategies } from '$lib/utils/stores/stores';
	import type { Alert, AlertLog, Instance, Strategy } from '$lib/utils/types/types';
	import { newPriceAlert } from './interface';

	/* ───── Types ───────────────────────────────────────────────────────────── */
	interface ExtendedInstance extends Instance {
		[key: string]: any;
	}

	/* ───── Form state variables ────────────────────────────────────────────── */
	let showAddAlertForm = false;
	let alertTypeSelection: 'price' | 'strategy' | null = null;

	// Price alert form variables
	let newAlertTicker: Instance | null = null;
	let newAlertPrice: number | null = null;

	// Strategy alert form variables
	let selectedStrategy: Strategy | null = null;
	let strategyThreshold: number | null = null;
	let strategyUniverse: string[] = [];
	let strategyUniverseText = '';
	let universeAllTickers = true;

	/* ───── Delete helpers ──────────────────────────────────────────────────── */
	function deleteAlert(alert: Alert) {
		privateRequest('deleteAlert', { alertId: alert.alertId }, true);
	}

	function deleteAlertLog(alertLog: AlertLog) {
		alertLogs.update((curr) => (curr ? curr.filter((log) => log !== alertLog) : []));
	}

	/* ───── Strategy alert helpers ──────────────────────────────────────────── */
	function saveStrategyAlert() {
		if (!selectedStrategy || strategyThreshold === null) {
			return;
		}

		// Parse the universe text into array
		const parsedUniverse = universeAllTickers
			? null
			: strategyUniverseText
					.split(',')
					.map((t) => t.trim())
					.filter((t) => t);

		privateRequest(
			'setAlert',
			{
				strategyId: selectedStrategy.strategyId,
				active: true,
				threshold: strategyThreshold,
				universe: parsedUniverse
			},
			true
		);

		resetForm();
	}

	function toggleStrategyAlert(strategy: Strategy, active: boolean) {
		privateRequest(
			'setAlert',
			{
				strategyId: strategy.strategyId,
				active: active,
				threshold: strategy.alertThreshold || 0,
				universe: strategy.alertUniverse || []
			},
			true
		);
	}

	/* ───── Form logic functions ────────────────────────────────────────────── */
	async function openTickerSelection() {
		try {
			const selectedInstance = await queryInstanceInput('any', ['ticker'], {
				ticker: ''
			});
			newAlertTicker = selectedInstance;
		} catch (error: any) {
			// Handle cancellation silently
			if (error.message !== 'User cancelled input') {
				console.error('Error selecting ticker:', error);
			}
		}
	}

	function saveAlert() {
		// Validation
		if (!newAlertTicker || !newAlertPrice) {
			return;
		}

		// Create the alert using the existing newPriceAlert function
		const alertInstance: Instance = {
			...newAlertTicker,
			price: newAlertPrice
		};

		newPriceAlert(alertInstance);

		// Reset form
		resetForm();
	}

	function cancel() {
		resetForm();
	}

	function resetForm() {
		showAddAlertForm = false;
		alertTypeSelection = null;
		// Reset price alert form
		newAlertTicker = null;
		newAlertPrice = null;
		// Reset strategy alert form
		selectedStrategy = null;
		strategyThreshold = null;
		strategyUniverse = [];
		strategyUniverseText = '';
		universeAllTickers = true;
	}

	/* ───── Export function for parent components ───────────────────────────── */
	export function showForm() {
		showAddAlertForm = true;
		alertTypeSelection = null; // Start with type selection
	}

	/* ───── Props ──────────────────────────────────────────────────────────── */
	export let view: 'active' | 'inactive' | 'history' = 'active';

	/* ───── Filtering state ─────────────────────────────────────────────────── */
	let alertFilter: 'all' | 'price' | 'strategy' = 'all';

	/* ───── Filtered stores ─────────────────────────────────────────────────── */
	$: filteredActiveAlerts =
		$activeAlerts?.filter((alert) => alertFilter === 'all' || alert.alertType === alertFilter) ||
		[];

	$: filteredInactiveAlerts =
		$inactiveAlerts?.filter((alert) => alertFilter === 'all' || alert.alertType === alertFilter) ||
		[];

	$: filteredAlertLogs =
		$alertLogs?.filter((log) => alertFilter === 'all' || log.alertType === alertFilter) || [];

	/* ───── Cast stores for <List> component ────────────────────────────────── */
	$: extendedActiveAlerts = writable(filteredActiveAlerts) as unknown as Writable<
		ExtendedInstance[]
	>;
	$: extendedInactiveAlerts = writable(filteredInactiveAlerts) as unknown as Writable<
		ExtendedInstance[]
	>;
	$: extendedAlertLogs = writable(filteredAlertLogs) as unknown as Writable<ExtendedInstance[]>;

	const handleDeleteAlert = (item: ExtendedInstance) => deleteAlert(item as unknown as Alert);
	const handleDeleteAlertLog = (item: ExtendedInstance) =>
		deleteAlertLog(item as unknown as AlertLog);

	/* ───── Form validation ─────────────────────────────────────────────────── */
	$: isPriceFormValid = newAlertTicker && newAlertPrice && newAlertPrice > 0;
	$: isStrategyFormValid = selectedStrategy && strategyThreshold !== null && strategyThreshold >= 0;
</script>

<!-- ───── UI ─────────────────────────────────────────────────────────────────── -->
<div class="alerts-container">
	<!-- Add Alert Form -->
	{#if showAddAlertForm}
		<div class="add-alert-form">
			<h4>Create New Alert</h4>

			<!-- Alert Type Selection -->
			{#if !alertTypeSelection}
				<div class="alert-type-selection">
					<p>Choose alert type:</p>
					<div class="type-buttons">
						<button class="type-button" on:click={() => (alertTypeSelection = 'price')}>
							Price Alert
						</button>
						<button class="type-button" on:click={() => (alertTypeSelection = 'strategy')}>
							Strategy Alert
						</button>
					</div>
				</div>
			{:else if alertTypeSelection === 'price'}
				<!-- Price Alert Form -->
				<div class="form-field">
					<label>Ticker:</label>
					<button class="ticker-selector" on:click={openTickerSelection}>
						{newAlertTicker?.ticker || 'Select Ticker'}
					</button>
				</div>

				<div class="form-field">
					<label>Alert Price:</label>
					<input
						type="number"
						step="0.01"
						min="0"
						bind:value={newAlertPrice}
						placeholder="Enter price"
						class="price-input"
					/>
				</div>

				<div class="form-buttons">
					<button class="cancel-button" on:click={cancel}>Cancel</button>
					<button class="back-button" on:click={() => (alertTypeSelection = null)}>Back</button>
					<button class="save-button" on:click={saveAlert} disabled={!isPriceFormValid}>
						Save Alert
					</button>
				</div>
			{:else if alertTypeSelection === 'strategy'}
				<!-- Strategy Alert Form -->
				<div class="form-field">
					<label>Strategy:</label>
					<select bind:value={selectedStrategy} class="strategy-selector">
						<option value={null}>Select Strategy</option>
						{#each $strategies || [] as strategy}
							<option value={strategy}>{strategy.name}</option>
						{/each}
					</select>
				</div>

				<div class="form-field">
					<label>Threshold:</label>
					<input
						type="number"
						step="0.01"
						min="0"
						bind:value={strategyThreshold}
						placeholder="Enter threshold"
						class="threshold-input"
					/>
				</div>

				<div class="form-field">
					<label>Universe:</label>
					<div class="universe-controls">
						<label class="checkbox-label">
							<input type="checkbox" bind:checked={universeAllTickers} />
							All Tickers
						</label>
						{#if !universeAllTickers}
							<textarea
								bind:value={strategyUniverseText}
								placeholder="Enter tickers separated by commas (e.g. AAPL, MSFT, GOOGL)"
								class="universe-input"
							></textarea>
						{/if}
					</div>
				</div>

				<div class="form-buttons">
					<button class="cancel-button" on:click={cancel}>Cancel</button>
					<button class="back-button" on:click={() => (alertTypeSelection = null)}>Back</button>
					<button class="save-button" on:click={saveStrategyAlert} disabled={!isStrategyFormValid}>
						Save Alert
					</button>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Alert Filter Controls -->
	<div class="filter-controls">
		<div class="filter-buttons">
			<button
				class="filter-button {alertFilter === 'all' ? 'active' : ''}"
				on:click={() => (alertFilter = 'all')}
			>
				All
			</button>
			<button
				class="filter-button {alertFilter === 'price' ? 'active' : ''}"
				on:click={() => (alertFilter = 'price')}
			>
				Price
			</button>
			<button
				class="filter-button {alertFilter === 'strategy' ? 'active' : ''}"
				on:click={() => (alertFilter = 'strategy')}
			>
				Strategy
			</button>
		</div>
	</div>

	{#if view === 'active'}
		<!-- Active Alerts -->
		<h3>Active Alerts</h3>
		<List
			on:contextmenu={(event) => {
				event.preventDefault();
			}}
			list={extendedActiveAlerts}
			columns={['Ticker', 'alertPrice']}
			parentDelete={handleDeleteAlert}
		/>
	{:else if view === 'inactive'}
		<!-- Inactive Alerts -->
		<h3>Inactive Alerts</h3>
		<List
			on:contextmenu={(event) => {
				event.preventDefault();
			}}
			list={extendedInactiveAlerts}
			columns={['Ticker', 'alertPrice']}
			parentDelete={handleDeleteAlert}
		/>
	{:else if view === 'history'}
		<!-- Alert History -->
		<h3>Alert History</h3>
		<List
			on:contextmenu={(event) => {
				event.preventDefault();
			}}
			list={extendedAlertLogs}
			columns={['Ticker', 'Timestamp']}
			parentDelete={handleDeleteAlertLog}
		/>
	{/if}
</div>

<style>
	.alerts-container {
		/* Increase top padding to match watchlist title position */
		padding: clamp(0.25rem, 0.5vw, 0.75rem) clamp(0.5rem, 1vw, 1rem) clamp(0.5rem, 1vw, 1rem);
		height: 100%;
		overflow-y: auto;
	}

	.alerts-container h3 {
		margin-left: 8px; /* Match watchlist title left margin */
	}

	.add-alert-form {
		background: rgba(255, 255, 255, 0.05);
		border: 1px solid rgba(255, 255, 255, 0.1);
		border-radius: 8px;
		padding: 16px;
		margin-bottom: 16px;
	}

	.add-alert-form h4 {
		margin: 0 0 16px 0;
		color: #ffffff;
		font-size: 14px;
		font-weight: 600;
	}

	.form-field {
		margin-bottom: 12px;
	}

	.form-field label {
		display: block;
		margin-bottom: 4px;
		color: #ffffff;
		font-size: 12px;
		font-weight: 500;
	}

	.ticker-selector {
		width: 100%;
		padding: 8px 12px;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		color: #ffffff;
		font-size: 13px;
		cursor: pointer;
		transition: all 0.2s ease;
		text-align: left;
	}

	.ticker-selector:hover {
		background: rgba(255, 255, 255, 0.15);
		border-color: rgba(255, 255, 255, 0.3);
	}

	.price-input {
		width: 100%;
		padding: 8px 12px;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		color: #ffffff;
		font-size: 13px;
	}

	.price-input:focus {
		outline: none;
		border-color: rgba(255, 255, 255, 0.4);
		background: rgba(255, 255, 255, 0.15);
	}

	.form-buttons {
		display: flex;
		gap: 8px;
		margin-top: 16px;
	}

	.cancel-button,
	.save-button {
		flex: 1;
		padding: 8px 16px;
		border: none;
		border-radius: 4px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.cancel-button {
		background: rgba(255, 255, 255, 0.1);
		color: #ffffff;
	}

	.cancel-button:hover {
		background: rgba(255, 255, 255, 0.2);
	}

	.save-button {
		background: #089981;
		color: #ffffff;
	}

	.save-button:hover:not(:disabled) {
		background: #0a8a73;
	}

	.save-button:disabled {
		background: rgba(255, 255, 255, 0.1);
		color: rgba(255, 255, 255, 0.5);
		cursor: not-allowed;
	}

	/* ───── Alert Type Selection ──────────────────────────────────────────── */
	.alert-type-selection {
		text-align: center;
		margin-bottom: 16px;
	}

	.alert-type-selection p {
		margin: 0 0 12px 0;
		color: #ffffff;
		font-size: 13px;
	}

	.type-buttons {
		display: flex;
		gap: 12px;
		justify-content: center;
	}

	.type-button {
		flex: 1;
		padding: 12px 16px;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 6px;
		color: #ffffff;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.type-button:hover {
		background: rgba(255, 255, 255, 0.2);
		border-color: rgba(255, 255, 255, 0.3);
	}

	.back-button {
		flex: 1;
		padding: 8px 16px;
		border: none;
		border-radius: 4px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.2s ease;
		background: rgba(255, 255, 255, 0.15);
		color: #ffffff;
	}

	.back-button:hover {
		background: rgba(255, 255, 255, 0.25);
	}

	/* ───── Strategy Alert Form ────────────────────────────────────────────── */
	.strategy-selector,
	.threshold-input {
		width: 100%;
		padding: 8px 12px;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		color: #ffffff;
		font-size: 13px;
	}

	.strategy-selector:focus,
	.threshold-input:focus {
		outline: none;
		border-color: rgba(255, 255, 255, 0.4);
		background: rgba(255, 255, 255, 0.15);
	}

	.strategy-selector option {
		background: #1a1a1a;
		color: #ffffff;
	}

	.universe-controls {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.checkbox-label {
		display: flex;
		align-items: center;
		gap: 6px;
		color: #ffffff;
		font-size: 12px;
		cursor: pointer;
	}

	.checkbox-label input[type='checkbox'] {
		margin: 0;
	}

	.universe-input {
		width: 100%;
		min-height: 60px;
		padding: 8px 12px;
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 4px;
		color: #ffffff;
		font-size: 13px;
		resize: vertical;
		font-family: inherit;
	}

	.universe-input:focus {
		outline: none;
		border-color: rgba(255, 255, 255, 0.4);
		background: rgba(255, 255, 255, 0.15);
	}

	/* ───── Filter Controls ────────────────────────────────────────────────── */
	.filter-controls {
		margin-bottom: 16px;
	}

	.filter-buttons {
		display: flex;
		gap: 4px;
		background: rgba(255, 255, 255, 0.05);
		border-radius: 6px;
		padding: 4px;
	}

	.filter-button {
		flex: 1;
		padding: 6px 12px;
		background: transparent;
		border: none;
		border-radius: 4px;
		color: rgba(255, 255, 255, 0.7);
		font-size: 12px;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.filter-button:hover {
		color: #ffffff;
		background: rgba(255, 255, 255, 0.1);
	}

	.filter-button.active {
		background: rgba(255, 255, 255, 0.15);
		color: #ffffff;
		font-weight: 600;
	}
</style>
