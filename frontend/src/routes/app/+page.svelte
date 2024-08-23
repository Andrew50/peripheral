<!-- app/+page.svelte -->
<script lang="ts">
    import {auth_data, request} from '../../store.js'
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    import { writable } from 'svelte/store';
    
    let ticker: string;
    let timestamp: number;
    let errorMessage = writable<string>('')
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
    let instances: Instance[] = [];
    $: if ($auth_data == null && browser) {
        goto('/login');
    }
    function newInstance (): void {
        if (ticker && timestamp) {
            request(null, true, "NewInstance", ticker, timestamp).then((result)=> errorMessage.set(result))
            if (!errMessage){
                const security: Security = {ticker: ticker, cik: res["cik"]}
                const instance: Instance = {
                    instance_id: result["instance_id"],
                    security: security,
                    timestamp: timestamp,
                    annotations: []}
                instances.push(instance)
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
    <th god />
    <th god />
    <th god />
    {#each $instances as instance}
        <tr> instance






<style>



</style>




