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
		console.log(alert.alertType);
		if (alert.alertType === 'strategy') {
			privateRequest(
				'setAlert',
				{
					strategyId: alert.strategyId,
					active: false,
					threshold: alert.alertThreshold,
					universe: alert.alertUniverse || []
				},
				true
			)
				.then(() => {
					// Update the strategies store to reflect the alert is now inactive
					strategies.update((currentStrategies) =>
						currentStrategies.map((strategy) =>
							strategy.strategyId === alert.strategyId
								? { ...strategy, isAlertActive: false }
								: strategy
						)
					);
				})
				.catch((error) => {
					console.error('Error deleting strategy alert:', error);
				});
		} else {
			privateRequest('deleteAlert', { alertId: alert.alertId }, true);
		}
	}

	function deleteAlertLog(alertLog: AlertLog) {
		alertLogs.update((curr) => (curr ? curr.filter((log) => log !== alertLog) : []));
	}

	/* ───── Strategy alert helpers ──────────────────────────────────────────── */
	function saveStrategyAlert() {
		if (!selectedStrategy || strategyThreshold === null) {
			return;
		}

		// Store references before resetForm() clears them
		const strategyToUpdate = selectedStrategy;
		const thresholdValue = strategyThreshold;

		// Parse the universe text into array
		const parsedUniverse = universeAllTickers
			? null
			: strategyUniverseText
					.split(',')
					.map((t: string) => t.trim())
					.filter((t) => t);

		privateRequest(
			'setAlert',
			{
				strategyId: strategyToUpdate.strategyId,
				active: true,
				threshold: thresholdValue,
				universe: parsedUniverse
			},
			true
		)
			.then(() => {
				// Update the strategies store to reflect the alert is now active
				strategies.update((currentStrategies) =>
					currentStrategies.map((strategy) =>
						strategy.strategyId === strategyToUpdate.strategyId
							? {
									...strategy,
									isAlertActive: true,
									alertThreshold: thresholdValue,
									alertUniverse: parsedUniverse || []
								}
							: strategy
					)
				);
			})
			.catch((error) => {
				console.error('Error setting strategy alert:', error);
			});

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
		)
			.then(() => {
				// Update the strategies store to reflect the alert status change
				strategies.update((currentStrategies) =>
					currentStrategies.map((s) =>
						s.strategyId === strategy.strategyId ? { ...s, isAlertActive: active } : s
					)
				);
			})
			.catch((error) => {
				console.error('Error toggling strategy alert:', error);
			});
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

	// Handler to submit form on Enter key press
	function handleSubmit() {
		if (alertTypeSelection === 'price') {
			saveAlert();
		} else if (alertTypeSelection === 'strategy') {
			saveStrategyAlert();
		}
	}

	/* ───── Export functions for parent components ──────────────────────────── */
	export function showPriceForm() {
		console.log('Showing price alert form');
		showAddAlertForm = true;
		alertTypeSelection = 'price'; // Skip type selection, go directly to price form
	}

	export function showStrategyForm() {
		console.log('Showing strategy alert form');
		showAddAlertForm = true;
		alertTypeSelection = 'strategy'; // Skip type selection, go directly to strategy form
	}

	/* ───── Props ──────────────────────────────────────────────────────────── */
	export let view: 'price' | 'strategy' | 'logs' = 'price';

	// Add reactive watcher to close the add alert form when the view changes
	let prevView: 'price' | 'strategy' | 'logs' = view;
	$: if (view !== prevView) {
		if (showAddAlertForm) {
			resetForm();
		}
		prevView = view;
	}

	/* ───── View-specific data preparation ──────────────────────────────────── */
	$: priceAlerts = [
		...($activeAlerts?.filter((alert) => alert.alertType === 'price') || []),
		...($inactiveAlerts?.filter((alert) => alert.alertType === 'price') || [])
	];

	$: strategyAlerts =
		$strategies
			?.filter((strategy) => strategy.isAlertActive === true)
			.map((strategy) => ({
				...strategy,
				alertType: 'strategy',
				alertId: strategy.strategyId
			})) || [];

	$: alertLogsWithCondition =
		$alertLogs?.map((log) => ({
			...log,
			Condition:
				log.alertType === 'price'
					? `Crossed $${log.alertPrice?.toFixed(2) || '0.00'}`
					: log.strategyName || 'Unknown Strategy'
		})) || [];

	/* ───── Cast stores for <List> component ────────────────────────────────── */
	$: extendedPriceAlerts = writable(priceAlerts) as unknown as Writable<ExtendedInstance[]>;
	$: extendedStrategyAlerts = writable(strategyAlerts) as unknown as Writable<ExtendedInstance[]>;
	$: extendedAlertLogs = writable(alertLogsWithCondition) as unknown as Writable<
		ExtendedInstance[]
	>;

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
		<form class="add-alert-form" on:submit|preventDefault={handleSubmit}>
			<h4>Create New Alert</h4>

			{#if alertTypeSelection === 'price'}
				<!-- Price Alert Form -->
				<div class="form-field">
					<label for="ticker-selector">Ticker:</label>
					<button id="ticker-selector" class="ticker-selector" on:click={openTickerSelection}>
						{newAlertTicker?.ticker || 'Select Ticker'}
					</button>
				</div>

				<div class="form-field">
					<label for="price-input">Alert Price:</label>
					<input
						id="price-input"
						type="number"
						step="0.01"
						min="0"
						bind:value={newAlertPrice}
						placeholder="Enter price"
						class="price-input"
					/>
				</div>

				<div class="form-buttons">
					<button type="button" class="cancel-button" on:click={cancel}>Cancel</button>
					<button type="submit" class="save-button" disabled={!isPriceFormValid}>Save Alert</button>
				</div>
			{:else if alertTypeSelection === 'strategy'}
				<!-- Strategy Alert Form -->
				<div class="form-field">
					<label for="strategy-selector">Strategy:</label>
					<select id="strategy-selector" bind:value={selectedStrategy} class="strategy-selector">
						<option value={null}>Select Strategy</option>
						{#each $strategies || [] as strategy}
							<option value={strategy}>{strategy.name}</option>
						{/each}
					</select>
				</div>

				<div class="form-field">
					<label for="threshold-input">Threshold:</label>
					<input
						id="threshold-input"
						type="number"
						step="0.01"
						min="0"
						bind:value={strategyThreshold}
						placeholder="Enter threshold"
						class="threshold-input"
					/>
				</div>

				<div class="form-buttons">
					<button type="button" class="cancel-button" on:click={cancel}>Cancel</button>
					<button type="submit" class="save-button" disabled={!isStrategyFormValid}
						>Save Alert</button
					>
				</div>
			{/if}
		</form>
	{/if}

	{#if view === 'price'}
		<!-- Price Alerts -->
		<h3>Price Alerts</h3>
		<List
			on:contextmenu={(event) => {
				event.preventDefault();
			}}
			list={extendedPriceAlerts}
			columns={['Ticker', 'alertPrice']}
			parentDelete={handleDeleteAlert}
		/>
	{:else if view === 'strategy'}
		<!-- Strategy Alerts -->
		<h3>Strategy Alerts</h3>
		<List
			on:contextmenu={(event) => {
				event.preventDefault();
			}}
			list={extendedStrategyAlerts}
			columns={['name', 'alertThreshold']}
			parentDelete={handleDeleteAlert}
		/>
	{:else if view === 'logs'}
		<!-- Alert Logs -->
		<h3>Alert Logs</h3>
		<List
			on:contextmenu={(event) => {
				event.preventDefault();
			}}
			list={extendedAlertLogs}
			columns={['Ticker', 'Timestamp', 'Condition']}
			parentDelete={handleDeleteAlertLog}
		/>
	{/if}
</div>

<style>
	.alerts-container {
		/* Increase top padding to match watchlist title position */
		padding: clamp(0.25rem, 0.5vw, 0.75rem) clamp(0.5rem, 1vw, 1rem) clamp(0.5rem, 1vw, 1rem);
		height: 100%;
		overflow-y: hidden;
	}

	.alerts-container h3 {
		margin-left: 8px; /* Match watchlist title left margin */
	}

	.add-alert-form {
		background: rgb(255 255 255 / 5%);
		border: 1px solid rgb(255 255 255 / 10%);
		border-radius: 8px;
		padding: 16px;
		margin-bottom: 16px;
	}

	.add-alert-form h4 {
		margin: 0 0 16px;
		color: #fff;
		font-size: 14px;
		font-weight: 600;
	}

	.form-field {
		margin-bottom: 12px;
	}

	.form-field label {
		display: block;
		margin-bottom: 4px;
		color: #fff;
		font-size: 12px;
		font-weight: 500;
	}

	.ticker-selector {
		width: 100%;
		padding: 8px 12px;
		background: rgb(255 255 255 / 10%);
		border: 1px solid rgb(255 255 255 / 20%);
		border-radius: 4px;
		color: #fff;
		font-size: 13px;
		cursor: pointer;
		transition: all 0.2s ease;
		text-align: left;
	}

	.ticker-selector:hover {
		background: rgb(255 255 255 / 15%);
		border-color: rgb(255 255 255 / 30%);
	}

	.price-input {
		width: 100%;
		padding: 8px 12px;
		background: rgb(255 255 255 / 10%);
		border: 1px solid rgb(255 255 255 / 20%);
		border-radius: 4px;
		color: #fff;
		font-size: 13px;
	}

	.price-input:focus {
		outline: none;
		border-color: rgb(255 255 255 / 40%);
		background: rgb(255 255 255 / 15%);
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
		background: rgb(255 255 255 / 10%);
		color: #fff;
	}

	.cancel-button:hover {
		background: rgb(255 255 255 / 20%);
	}

	.save-button {
		background: #089981;
		color: #fff;
	}

	.save-button:hover:not(:disabled) {
		background: #0a8a73;
	}

	.save-button:disabled {
		background: rgb(255 255 255 / 10%);
		color: rgb(255 255 255 / 50%);
		cursor: not-allowed;
	}

	/* ───── Strategy Alert Form ────────────────────────────────────────────── */
	.strategy-selector,
	.threshold-input {
		width: 100%;
		padding: 8px 12px;
		background: rgb(255 255 255 / 10%);
		border: 1px solid rgb(255 255 255 / 20%);
		border-radius: 4px;
		color: #fff;
		font-size: 13px;
	}

	.strategy-selector:focus,
	.threshold-input:focus {
		outline: none;
		border-color: rgb(255 255 255 / 40%);
		background: rgb(255 255 255 / 15%);
	}

	.strategy-selector option {
		background: #1a1a1a;
		color: #fff;
	}

	/* .universe-controls {
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
	} */
</style>
