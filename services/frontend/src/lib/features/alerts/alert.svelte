<script lang="ts">
	import List from '$lib/utils/modules/list.svelte';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { writable, type Writable } from 'svelte/store';
	import { onMount } from 'svelte';
	import { querySetup } from '$lib/utils/popups/setup.svelte';
	import { queryAlgo } from '$lib/utils/popups/algo.svelte';
	import { privateRequest } from '$lib/core/backend';
	import { activeAlerts, inactiveAlerts, alertLogs } from '$lib/core/stores';
	import type { Alert, AlertLog, Instance } from '$lib/core/types';
	import { newAlert, newPriceAlert } from './interface';

	// Define ExtendedInstance type that matches the List component's expectations
	interface ExtendedInstance extends Instance {
		[key: string]: any; // Allow dynamic property access
	}

	let selectedAlertType = writable<string>('price');

	// Add state for showing alert type descriptions
	let alertTypeDescription = writable<string>('Set alerts based on price levels');

	// Update description when alert type changes
	$: {
		switch ($selectedAlertType) {
			case 'price':
				$alertTypeDescription = 'Set alerts based on price levels';
				break;
			case 'setup':
				$alertTypeDescription = 'Get notified when a setup triggers';
				break;
			case 'algo':
				$alertTypeDescription = 'Get notified when an algorithm signals';
				break;
		}
	}

	async function createAlert(event: MouseEvent) {
		const alertType = $selectedAlertType; // Get selected alert type from the dropdown

		if (alertType === 'price') {
			const inst = await queryInstanceInput(['ticker', 'price'], ['ticker', 'price'], {
				ticker: ''
			});
			// Call newPriceAlert with the instance
			await newPriceAlert(inst);
		} else if (alertType === 'setup') {
			const setupId = await querySetup(event);
			newAlert({
				setupId: setupId,
				alertType: 'setup',
				price: undefined, // Use undefined instead of null
				securityId: undefined // Use undefined instead of null
			});
		} else if (alertType === 'algo') {
			const algoId = await queryAlgo(event);
			newAlert({
				algoId: algoId,
				alertType: 'algo'
			});
		}
	}

	function deleteAlert(alert: Alert) {
		privateRequest('deleteAlert', { alertId: alert.alertId }, true);
	}

	function deleteAlertLog(alertLog: AlertLog) {
		alertLogs.update((currentLogs) =>
			currentLogs ? currentLogs.filter((log) => log !== alertLog) : []
		);
	}

	// Cast stores to ExtendedInstance[] type
	$: extendedActiveAlerts = activeAlerts as unknown as Writable<ExtendedInstance[]>;
	$: extendedInactiveAlerts = inactiveAlerts as unknown as Writable<ExtendedInstance[]>;
	$: extendedAlertLogs = alertLogs as unknown as Writable<ExtendedInstance[]>;

	// Cast delete handlers to match ExtendedInstance parameter
	const handleDeleteAlert = (item: ExtendedInstance) => deleteAlert(item as unknown as Alert);
	const handleDeleteAlertLog = (item: ExtendedInstance) =>
		deleteAlertLog(item as unknown as AlertLog);
</script>

<div class="alert-creator">
	<div class="alert-type-selector">
		<h3>Create New Alert</h3>
		<div class="alert-types">
			<button
				class="alert-type-btn {$selectedAlertType === 'price' ? 'active' : ''}"
				on:click={() => ($selectedAlertType = 'price')}
			>
				<i class="fas fa-dollar-sign"></i>
				Price Alert
			</button>
			<button
				class="alert-type-btn {$selectedAlertType === 'setup' ? 'active' : ''}"
				on:click={() => ($selectedAlertType = 'setup')}
			>
				<i class="fas fa-chart-line"></i>
				Setup Alert
			</button>
			<button
				class="alert-type-btn {$selectedAlertType === 'algo' ? 'active' : ''}"
				on:click={() => ($selectedAlertType = 'algo')}
			>
				<i class="fas fa-robot"></i>
				Algo Alert
			</button>
		</div>
		<p class="description">{$alertTypeDescription}</p>
		<button class="create-btn" on:click={(event) => createAlert(event)}> Create Alert </button>
	</div>
</div>

<!-- Active Alerts -->
<h3>Active Alerts</h3>
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={extendedActiveAlerts}
	columns={['Alert Type', 'Ticker', 'Alert Price']}
	parentDelete={handleDeleteAlert}
/>

<!-- Inactive Alerts -->
<h3>Inactive Alerts</h3>
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={extendedInactiveAlerts}
	columns={['Alert Type', 'Ticker', 'Alert Price']}
	parentDelete={handleDeleteAlert}
/>

<!-- Alert Logs -->
<h3>Alert History</h3>
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={extendedAlertLogs}
	columns={['Ticker', 'Timestamp', 'Alert Type']}
	parentDelete={handleDeleteAlertLog}
/>

<style>
	.alert-creator {
		background: var(--ui-bg-secondary);
		border-radius: 8px;
		padding: 20px;
		margin-bottom: 24px;
	}

	.alert-type-selector {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.alert-types {
		display: flex;
		gap: 12px;
	}

	.alert-type-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 12px 16px;
		border-radius: 6px;
		border: 1px solid var(--ui-border);
		background: var(--ui-bg-primary);
		color: var(--text-primary);
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.alert-type-btn:hover {
		background: var(--ui-bg-hover);
	}

	.alert-type-btn.active {
		background: var(--accent-primary);
		color: white;
		border-color: var(--accent-primary);
	}

	.description {
		color: var(--text-secondary);
		font-size: 14px;
		margin: 0;
	}

	.create-btn {
		background: var(--accent-primary);
		color: white;
		padding: 12px 24px;
		border-radius: 6px;
		border: none;
		font-weight: 600;
		cursor: pointer;
		transition: background 0.2s ease;
		align-self: flex-start;
	}

	.create-btn:hover {
		background: var(--accent-primary-dark);
	}
</style>
