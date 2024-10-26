<script lang="ts">
    import List from "$lib/utils/modules/list.svelte";
    import {queryInstanceInput} from '$lib/utils/popups/input.svelte';
    import { writable } from "svelte/store";
    import { onMount } from "svelte";
    import {querySetup} from '$lib/utils/popups/setup.svelte'; // Assuming you have a specific function for querying setup inputs
    import { privateRequest } from '$lib/core/backend';
    import {type Alert,type AlertLog, alerts,newAlert,newPriceAlert, alertLogs} from './interface';
    let selectedAlertType = writable<string>('price');
    onMount(() => {
        if ($alerts === undefined) {
            privateRequest<Alert[]>("getAlerts", {}, true).then((v: Alert[]) => { alerts.set(v) });
        }
        if ($alertLogs === undefined) {
            privateRequest<AlertLog[]>("getAlertLogs", {}, true).then((v: AlertLog[]) => { alertLogs.set(v) });
        }
    });
    async function createAlert(event:MouseEvent) {
        const alertType = $selectedAlertType; // Get selected alert type from the dropdown

        if (alertType === 'price') {
        const inst = await queryInstanceInput(["ticker","price"], { ticker: "" });
        //price?
        const price = 0;
        newPriceAlert(inst.securityId,price)
        } else if (alertType === 'setup') {
            const setupId = await querySetup(event)
            newAlert({
                setupId:setupId,
                alertType: 'setup',
                price: null, // No price for setup alerts
                securityId: null // No securityId for setup alerts
            }) 
        } else if (alertType === "algo") {
            const algoId = await queryAlgo(event)
            newAlert({
                algoId:algoId,
                alertType: 'algo',
            })
        }

    }
</script>

<div class="controls-container">
    <label for="alertType">Select Alert Type:</label>
    <select id="alertType" bind:value={$selectedAlertType}>
        <option value="price">Price</option>
        <option value="setup">Setup</option>
        <option value="algo">Setup</option>
    </select>
    <button on:click={(event)=>{createAlert(event)}}> New </button>
</div>
<List on:contextmenu={(event) => { event.preventDefault(); }} list={alerts} columns={["alertType", "ticker", "price"]} />
<List on:contextmenu={(event) => { event.preventDefault(); }} list={alertLogs} columns={["ticker", "timestamp", "alertType"]} />
