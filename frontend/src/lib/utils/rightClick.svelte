<!-- instance.svlete -->
<script lang="ts" context="module">
    import {changeChart} from '$lib/features/chart.svelte'
    import type { Writable } from 'svelte/store';
    import type {Instance } from '$lib/api/backend';
    let similarInstance: Writable<SimilarInstance> = writable({});
    import { privateRequest} from '$lib/api/backend';
    import {newStudy} from '$lib/features/study.svelte';
    import {newJournal} from '$lib/features/journal.svelte';
    import {newSample} from '$lib/features/sample.svelte'
    import { get, writable } from 'svelte/store';
    interface SimilarInstance {
        x: number
        y: number
        similarInstances: Instance[]
        status: "active" | "inactive"
    }
    interface RightClickQuery {
        x?: number;
        y?: number;
        source?: Source
        instance: Instance
        status: "inactive" | "active" | "initializing" | "cancelled" | "complete"
        result: RightClickResult
    }
    type RightClickResult = "edit" | "embed" | "alert" | "embdedSimilar" | "none" 
    type Source = "chart" | "embedded" | "similar"
    const inactiveRightClickQuery: RightClickQuery = {
        status:"inactive",
        result: "none",
        instance: {}
    }

    let rightClickQuery: Writable<RightClickQuery> = writable(inactiveRightClickQuery)

    export async function queryInstanceRightClick(event:MouseEvent,instance:Instance,source:Source):Promise<RightClickResult>{
        const rqQ: RightClickQuery = {
            x: event.clientX,
            y: event.clientY,
            source: source,
            status: "initializing",
            instance: instance,
            result: "none",
        }
        rightClickQuery.set(rqQ)
        return new Promise<RightClickResult>((resolve, reject) => {
            const unsubscribe = rightClickQuery.subscribe((r: RightClickQuery)=>{
                if (r.status === "cancelled"){
                    deactivate()
                    reject()
                }else if(r.status === "complete"){
                    const res = r.result
                    deactivate()
                    resolve(res)
                }
            })
            function deactivate(){
                rightClickQuery.set(inactiveRightClickQuery)
                unsubscribe()
            }
        })
    }
    function getSimilarInstances(event:MouseEvent):void{
        const baseIns = get(rightClickQuery)
        privateRequest<Instance[]>("getSimilarInstances",{ticker:baseIns.ticker,securityId:baseIns.securityId,timeframe:baseIns.timeframe,datetime:baseIns.datetime})
        .then((v:Instance[])=>{
            console.log(v)
            const simInst: SimilarInstance = {
                x: event.clientX,
                y: event.clientY,
                instances: v
            }
            console.log(simInst)
            similarInstance.set(simInst)
        })
    }
        
</script>


<script lang="ts">
    import {browser} from '$app/environment'
    import {onMount} from 'svelte'
    let rightClickMenu: HTMLElement;
    onMount(()=>{
        rightClickQuery.subscribe((v:RightClickQuery) => {
            if (browser){
                if (v.status === "initializing"){
                    document.addEventListener('click',handleClick)
                    document.addEventListener('keydown', handleKeyDown)
                    v.status = "active"
                    return v
                }else if(v.status == "inactive"){
                    document.removeEventListener('click',handleClick)
                    document.removeEventListener('keydown', handleKeyDown)
                }
            }
        })
    })
    function handleClick(event:MouseEvent):void{
        if (rightClickMenu && !rightClickMenu.contains(event.target as Node)) {
            closeRightClickMenu()
        }
    }
    function handleKeyDown(event:KeyboardEvent):void{
        if (event.key == "Escape"){
            closeRightClickMenu()
        }
    }

    function getStats():void{}
    function replay():void{}
    function addAlert():void{}
    function embed():void{}
    function edit():void{
        rightClickQuery.update((v:RightClickQuery)=>{
            v.result = "edit"
            return v
        })
    }
    function cancelRequest(){
        rightClickQuery.update((v:RightClickQuery)=>{
            v.status = "cancelled"
            return v
        })
    }
    function completeRequest(result:RightClickResult){
        rightClickQuery.update((v:RightClickQuery)=>{
            v.status = "complete"
            v.result = result
            return v
        })
    }
    function embedSimilar():void{
        rightClickQuery.update((v:RightClickQuery)=>{
            v.result = "embedSimilar"
            return v
        })
    }


</script>
{#if $rightClickQuery !== null}
    <div bind:this={rightClickMenu} class="context-menu" style="top: {$rightClickQuery.y}px; left: {$rightClickQuery.x}px;">
        <div>{$rightClickQuery.ticker} {$rightClickQuery.datetime} </div>
        <div><button on:click={()=>newStudy(get(rightClickQuery).instance)}> Add to Study </button></div>
        <!--<div><button on:click={()=>newSample(get(rightClickQuery).instance)}> Add to Sample </button></div>
        <div><button on:click={()=>newJournal(get(rightClickQuery).instance)}> Add to Journal </button></div>-->
        <div><button on:click={()=>getSimilarInstances(get(rightClickQuery).instance)}> Similar Instances </button></div>
        <!--<div><button on:click={getStats}> Instance Stats </button></div>
        <div><button on:click={replay}> Replay </button></div>-->
        {#if $rightClickQuery.source === "chart"}
            <!--<div><button on:click={()=>completeRequest("alert")}>Add Alert </button></div>-->
            <div><button on:click={()=>completeRequest("embed")}> Embed </button></div>
        {:else if $rightClickQuery.source === "embedded"}
            <div><button on:click={()=>completeRequest("edit")}> Edit </button></div>
            <!--<div><button on:click={()=>completeRequest("embdedSimilar")}> Embed Similar </button></div>-->
        {/if}
    </div>
{/if}
{#if $similarInstance.status === "active"}
    <div class="context-menu" style="top: {$similarInstance.y}px; left: {$similarInstance.x}px;">
        <table>
        {#each $similarInstance.instances as instance} 
            <tr>
                <td on:click={()=>changeChart(instance)} 
                on:contextmenu={(e)=>{e.preventDefault();
                queryInstanceRightClick(e,instance,"similar")}}>{instance.ticker}</td>
            </tr>
        {/each}
        </table>
    </div>
{/if}


<style>
    .red {
        color: red;
    }
    .normal {
        color: black;
    }
    .context-menu {
        position: absolute;
        background-color: white;
        border: 1px solid #ccc;
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        padding: 10px;
        border-radius: 4px;
    }

    .context-menu-item {
        background-color: transparent;
        border: none;
        padding: 5px 10px;
        text-align: left;
        cursor: pointer;
        width: 100%;
    }

    .context-menu-item:hover {
        background-color: #f0f0f0;
    }
    .hidden {
        display: none;
    }
</style>

