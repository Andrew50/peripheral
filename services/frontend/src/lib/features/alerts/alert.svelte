<script lang="ts">
	/* ───── Imports ─────────────────────────────────────────────────────────── */
	import List from '$lib/components/list.svelte';
	import { queryInstanceInput } from '$lib/components/input/input.svelte';
	import { writable, type Writable } from 'svelte/store';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { activeAlerts, inactiveAlerts, alertLogs } from '$lib/utils/stores/stores';
	import type { Alert, AlertLog, Instance } from '$lib/utils/types/types';
	import { newPriceAlert } from './interface';

	/* ───── Types ───────────────────────────────────────────────────────────── */
	interface ExtendedInstance extends Instance {
		[key: string]: any;
	}

	/* ───── Create price alert ──────────────────────────────────────────────── */
	async function createPriceAlert() {
		// Prompt user for ticker & price, then save via API helper
		const inst = await queryInstanceInput(['ticker', 'price'], ['ticker', 'price'], {
			ticker: ''
		});
		await newPriceAlert(inst);
	}

	/* ───── Delete helpers ──────────────────────────────────────────────────── */
	function deleteAlert(alert: Alert) {
		privateRequest('deleteAlert', { alertId: alert.alertId }, true);
	}

	function deleteAlertLog(alertLog: AlertLog) {
		alertLogs.update((curr) => (curr ? curr.filter((log) => log !== alertLog) : []));
	}

	/* ───── Cast stores for <List> component ────────────────────────────────── */
	$: extendedActiveAlerts = activeAlerts as unknown as Writable<ExtendedInstance[]>;
	$: extendedInactiveAlerts = inactiveAlerts as unknown as Writable<ExtendedInstance[]>;
	$: extendedAlertLogs = alertLogs as unknown as Writable<ExtendedInstance[]>;

	const handleDeleteAlert = (item: ExtendedInstance) => deleteAlert(item as Alert);
	const handleDeleteAlertLog = (item: ExtendedInstance) => deleteAlertLog(item as AlertLog);
</script>

<!-- ───── UI ─────────────────────────────────────────────────────────────────── -->
<div class="alerts-container">
	<div class="alert-creator">
		<h3>Create New Price Alert</h3>
		<p class="description">Set alerts based on price levels.</p>
		<button class="create-btn" on:click={createPriceAlert}>Create Alert</button>
	</div>

	<!-- Active Alerts -->
	<h3>Active Alerts</h3>
	<List
		on:contextmenu={(event) => {
			event.preventDefault();
		}}
		list={extendedActiveAlerts}
		columns={['Ticker', 'Alert Price']}
		parentDelete={handleDeleteAlert}
	/>

	<!-- Inactive Alerts -->
	<h3>Inactive Alerts</h3>
	<List
		on:contextmenu={(event) => {
			event.preventDefault();
		}}
		list={extendedInactiveAlerts}
		columns={['Ticker', 'Alert Price']}
		parentDelete={handleDeleteAlert}
	/>

	<!-- Alert Logs -->
	<h3>Alert History</h3>
	<List
		on:contextmenu={(event) => {
			event.preventDefault();
		}}
		list={extendedAlertLogs}
		columns={['Ticker', 'Timestamp']}
		parentDelete={handleDeleteAlertLog}
	/>
</div>

<style>
	.alerts-container {
		padding: clamp(0.5rem, 1vw, 1rem);
		height: 100%;
		overflow-y: auto;
	}

	.alert-creator {
		background: var(--ui-bg-secondary);
		border-radius: 8px;
		padding: 20px;
		margin-bottom: 24px;
	}

	.description {
		color: var(--text-secondary);
		font-size: 14px;
		margin: 0 0 12px 0;
	}

	.create-btn {
		background: var(--accent-primary);
		color: #fff;
		padding: 12px 24px;
		border-radius: 6px;
		border: none;
		font-weight: 600;
		cursor: pointer;
		transition: background 0.2s ease;
	}

	.create-btn:hover {
		background: var(--accent-primary-dark);
	}
</style>
