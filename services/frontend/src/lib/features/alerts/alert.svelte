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

	const handleDeleteAlert = (item: ExtendedInstance) => deleteAlert(item as unknown as Alert);
	const handleDeleteAlertLog = (item: ExtendedInstance) =>
		deleteAlertLog(item as unknown as AlertLog);
</script>

<!-- ───── UI ─────────────────────────────────────────────────────────────────── -->
<div class="alerts-container">
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
</style>
