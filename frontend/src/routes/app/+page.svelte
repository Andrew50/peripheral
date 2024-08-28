<!-- app/+page.svelte -->
<script lang="ts">
    import Entry from './entry.svelte'
    import { onMount } from 'svelte';
    import { privateRequest } from '../../store'
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    import { writable ,  get} from 'svelte/store';
    import type { Writable } from 'svelte/store';
    let ticker: string;
    let timestamp: number;
    let errorMessage = writable<string>('')
    let selectedInstanceId = writable(0);
    let instances = writable<Instance[]>([]);
    let currentAnnotation = "";
    /*errorMessage.subscribe((value) => {
        if (value !== "") {
            setTimeout(() => {
                errorMessage.set('');
            }, 5000);
        }
    });*/
    interface Security {
        ticker: string;
        cik: string;
    }
    interface Annotation {
        annotationId: number;
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
    selectedInstanceId.subscribe((v) => {console.log(v)});
    onMount(() => {
        privateRequest<string>("verifyAuth", {}).catch((error) => {
            goto('/login')
        });

        privateRequest<Instance[]>("getInstances", {}).then((result: Instance[]) => {
            //idk why the fuck this needs to be here but somehow for each throws??
            try {
                result.forEach((v) => {v.annotations = []})
            }catch{}
            instances.set(result);
        })
    });
    /*$: if ($authToken == "" && browser) {
        goto('/login');
    }*/
    function newAnnotation (instance : Instance, timeframe: string): void {
        console.log(instance)
        if (!instance.annotations.some((a) => a.timeframe === timeframe)){
            var annotationId: number;
            privateRequest<number>("newAnnotation", {timeframe:timeframe, instanceId: instance.instanceId}, errorMessage).then((annotationId: number) => {
                const newAnnotation: Annotation = {annotationId: annotationId,  timeframe: timeframe, entry:""}
                instance.annotations.push(newAnnotation)
            })
        } else {
            errorMessage.set("tf already exists")
        }
    }

    function getAnnotations(instance: Instance): void {
        privateRequest<Annotation[]>("getAnnotations",{instanceId:instance.instanceId}).then((result: Annotation[]) => {
            currentAnnotation = result;
    })
    }

    function newInstance (): void {
        if (ticker && timestamp) {
            let security: Security;
            privateRequest<CikResult>("getCik", {ticker:ticker}).then((result : CikResult) => {
                security = {ticker: ticker, cik: result.cik};
                privateRequest<NewInstanceResult>("newInstance", {cik:security.cik, timestamp:timestamp}, errorMessage)
                .then((result : NewInstanceResult) => {
                    console.log(result)
                    const instance: Instance = {
                        instanceId: result.instanceId,
                        security: security,
                        timestamp: timestamp,
                        annotations: []
                    }
                    instances.update((v) => {
                        if(v){
                            return [instance,...v]
                        } else {
                            return [instance]
                        }
                    })
                })
            });
        } else {
            errorMessage.set("unfilled form")
        }
    }
</script>
<Entry/>
<!--<h1> new instance </h1>
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
{#if Array.isArray($instances) && $instances.length > 0}
    {#each $instances as instance}
        <tr>
            <td> {instance.instanceId}</td>
            <td> {instance.security.ticker}</td>
            <td> {instance.timestamp}</td>
            <td>
                <button on:click={()=> {selectedInstanceId.set(instance.instanceId); getAnnotations(instance); console.log(instance)}}> Annotations </button>
            </td>
            <td>
                <button on:click={()=> {selectedInstanceId.set(instance.instanceId); newAnnotation(instance,"1d");}}> New </button>
            </td>
        </tr>
        {#if Array.isArray(instance.annotations) && instance.annotations.length > 0 && $selectedInstanceId == instance.instanceId}
            {#each instance.annotations as annotation}
                <tr> 
                <td colspan="4">
                    <div>{annotation.timeframe}</div>
                    <div><textarea bind:value={currentAnnotation}/></div>
                    <div>
                        <button on:click={() => {privateRequest("setAnnotation",{annotationId:annotation.annotationId, entry:currentAnnotation},errorMessage);}}> Save </button>
                    </div>
                </td>
                </tr>
            {/each}
        {/if}
    {/each}
{/if}
</table>
<button on:click={newInstance}> enter </button> -->
<style>
</style>
