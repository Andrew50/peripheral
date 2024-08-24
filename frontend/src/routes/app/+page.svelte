<!-- app/+page.svelte -->
<script lang="ts">
    import {auth_data, privateRequest} from '../../store'
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    import { writable } from 'svelte/store';
    
    let instanceId = 1;
    let ticker: string;
    let timestamp: number;
    let errorMessage = writable<string>('')
    let currentAnnotationId: number;
    interface Security {
        ticker: string;
        cik: string;
    }
    interface Annotation {
        timeframe: string;
        entry: string;
    }
    interface Instance {
        instance_id: number;
        security: Security;
        timestamp: number;
        annotations: Annotation[];
    }
    let instances = writable<Instance[]>([]);
    $: if ($auth_data == null && browser) {
        goto('/login');
    }
    function newInstance (): void {
        if (ticker && timestamp) {
            let cik: string;
            let security: Security;
            
            request(null, true, "GetCik", ticker).then((result) => {
                if(Array.isArray(result)) {
                const [cik, errorMessage] = result;
                if(errorMessage) {
                    errorMessage.set(errorMessage);
                }
                
                security = {ticker: ticker, cik: cik};
                request(null, true, "NewInstance", security.cik, timestamp).then((result) => {

                    if(Array.isArray(result)) {
                        const [instanceId, errorMessage] = result;
                        errorMessage.set(errorMessage)
                    }
                })
                const instance: Instance = {
                    instance_id: instanceId,
                    security: security,
                    timestamp: timestamp,
                    annotations: []
                }
                instances.update((v) => [instance,...v]);
                }
            });
           // request(null, true, "NewInstance", ticker, timestamp).then((result)=> errorMessage.set(result))

            //if (!errorMessage){
        } else {
            errorMessage.set("unfilled form")
        }
    }
</script>
<h1> new instance </h1>
<div class="form" >
    <div>
        <input bind:value={ticker}/>
    </div>
    <div>
        <input type="date" bind:value={timestamp}/>
    </div>
    <div>
        <button on:click={newInstance}> enter </button>
    </div>
    <div>
        {#if $errorMessage}
            {$errorMessage}
        {/if}
    </div>
</div>

<h1> instances </h1>
<table>
    <tr>
        <th> ID </th>
        <th> Ticker </th>
        <th> Datetime </th>
    </tr>
    {#each $instances as instance}
        <tr>
            <td> {instance.id}</td>
            <td> {instance.security.ticker}</td>
            <td> {instance.timestamp}</td>
            <td>
                <button on:click={()=> (currentAnnotationId = instance.id)}> Annotations </button>
            </td>
        </tr>
        {#if currentAnnotationId == instance.id}
            {#each instance.annotations as annotation}
                <tr> 
                    {annotation.timeframe}
                </tr>
                <tr> 
                    {annotation.entry}
                </tr>
            {/each}
        {/if}
    {/each}
</table>
<style>
</style>
