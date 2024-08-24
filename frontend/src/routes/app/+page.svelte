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
    errorMessage.subscribe((value) => {
        if (value !== "") {
            setTimeout(() => {
                errorMessage.set('');
            }, 5000);
        }
    });

    interface Security {
        ticker: string;
        cik: string;
    }
    interface Annotation {
        timeframe: string;
        entry: string;
    }
    interface Instance {
        instanceId: number;
        security: Security;
        timestamp: number;
        annotations: Annotation[];
    }

    interface CikResult {
        cik: string;
    }
    interface NewInstanceResult {
        instanceId : number;
    }
    let instances = writable<Instance[]>([]);
    $: if ($auth_data == "" && browser) {
        goto('/login');
    }

    function newAnnotation (instance : Instance, timeframe: string): void {
        if (!instance.annotations.map((a) => {a.timeframe}).contains(timeframe)){
            const newAnnotation: Annotation = {timeframe: timeframe, entry:""}
            instance.annotation.push(newAnnotation)
            console.log(instance)
        } else {
            errorMessage = "tf already exists"
        }
    }

    function newInstance (): void {
        if (ticker && timestamp) {
            let security: Security;
            privateRequest<CikResult>("GetCik", {ticker:ticker}, errorMessage).then((result : CikResult) => {
                security = {ticker: ticker, cik: result.cik};
                privateRequest<NewInstanceResult>("NewInstance", {cik:security.cik, timestamp:timestamp}, errorMessage).then((result : NewInstanceResult) => {
                    console.log(result)
                    const instance: Instance = {
                        instanceId: result.instanceId,
                        security: security,
                        timestamp: timestamp,
                        annotations: []
                    }
                    instances.update((v) => [instance,...v]);
                })
            });
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
            <td> {instance.instanceId}</td>
            <td> {instance.security.ticker}</td>
            <td> {instance.timestamp}</td>
            <td>
                <button on:click={()=> (currentAnnotationId = instance.instanceId)}> Annotations </button>
            </td>
            <td>
                <button on:click={()=> {currentAnnotationId = instance.instanceId; newAnnotation(instance,"1d");}}> Annotations </button>
            </td>
        </tr>
        {#if currentAnnotationId == instance.instanceId}
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
