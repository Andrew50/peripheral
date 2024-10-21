<script lang="ts">
    import List from "$lib/utils/modules/list.svelte"
    import {writable} from "svelte/store"
    import {onMount} from "svelte"
    import type {Alert,AlertLog} from './interface'
    import {privateRequest} from '$lib/core/backend'
    let alerts = writable<Alert[]>(undefined)
    let alertLogs = writable<AlertLog[]>(undefined)
    onMount(()=>{
        if ($alerts === undefined){
            privateRequest<Alert[]>("getAlerts",{},true)
            .then((v:Alert[])=>{alerts.set(v)})
        }
        if ($alertLogs === undefined){
            privateRequest<AlertLog[]>("getAlertLogs",{},true)
            .then((v:AlertLog[])=>{alertLogs.set(v)})
        }
    })

</script>


<List on:contextmenu={(event) => { event.preventDefault(); }} list={alerts} columns={["alertType","ticker","price"]} />
<List on:contextmenu={(event) => { event.preventDefault(); }} list={alertLogs} columns={["ticker","timestamp","alertType"]} />
