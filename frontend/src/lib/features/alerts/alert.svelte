<script lang="ts">
	import List from '$lib/utils/modules/list.svelte';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { writable } from 'svelte/store';
	import { onMount } from 'svelte';
	import { querySetup } from '$lib/utils/popups/setup.svelte';
	import { queryAlgo } from '$lib/utils/popups/algo.svelte';
	import { privateRequest } from '$lib/core/backend';
	import { activeAlerts, inactiveAlerts, alertLogs } from '$lib/core/stores';
	import { type Alert, type AlertLog, newAlert, newPriceAlert } from './interface';
	let selectedAlertType = writable<string>('price');

	async function createAlert(event: MouseEvent) {
		const alertType = $selectedAlertType; // Get selected alert type from the dropdown

		if (alertType === 'price') {
			const inst = await queryInstanceInput(['ticker', 'price'],['ticker','price'], { ticker: '' });
			//price?
			console.log('inst', inst);
			newPriceAlert(inst);
		} else if (alertType === 'setup') {
			const setupId = await querySetup(event);
			newAlert({
				setupId: setupId,
				alertType: 'setup',
				price: null, // No price for setup alerts
				securityId: null // No securityId for setup alerts
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
		alertLogs.update((currentLogs) => currentLogs.filter((log) => log !== alertLog));
	}

	// Add derived stores for active and inactive alerts
</script>

<div class="controls-container">
	<label for="alertType">Select Alert Type:</label>
	<select id="alertType" bind:value={$selectedAlertType}>
		<option value="price">Price</option>
		<option value="setup">Setup</option>
		<option value="algo">Algo</option>
	</select>
	<button
		on:click={(event) => {
			createAlert(event);
		}}
	>
		New
	</button>
</div>

<!-- Active Alerts -->
<h3>Active Alerts</h3>
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={activeAlerts}
	columns={['alertType', 'ticker', 'alertPrice']}
	parentDelete={deleteAlert}
/>

<!-- Inactive Alerts -->
<h3>Inactive Alerts</h3>
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={inactiveAlerts}
	columns={['alertType', 'ticker', 'alertPrice']}
	parentDelete={deleteAlert}
/>

<!-- Alert Logs -->
<h3>Alert History</h3>
<List
	on:contextmenu={(event) => {
		event.preventDefault();
	}}
	list={alertLogs}
	columns={['ticker', 'timestamp', 'alertType']}
	parentDelete={deleteAlertLog}
/>
