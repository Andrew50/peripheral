<!-- app/+page.svelte -->
<script lang="ts">
    import {auth_data, request} from '../../store.js'
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
        cik: number;
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
            let cik: number;
            [cik, errorMessage] = request(null, true, "GetCik", ticker)
           // request(null, true, "NewInstance", ticker, timestamp).then((result)=> errorMessage.set(result))
           if (!errorMessage) {

            //if (!errorMessage){
                const security: Security = {ticker: ticker, cik: cik}
                [instanceId, errorMessage] = request(null, true, "NewInstance", security.sik, timestamp);
                const instance: Instance = {
                    instance_id: result["instance_id"],
                    security: security,
                    timestamp: timestamp,
                    annotations: []
                }
                instances.update((v) => [instance,...v]);
            }
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
