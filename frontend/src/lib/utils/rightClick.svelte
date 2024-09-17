<!-- rightClick.svlete -->
<script lang="ts" context="module">
    //import {changeChart} from '$lib/features/chart/chart.svelte'
    import type { Writable } from 'svelte/store';
    import type {Instance } from '$lib/core/types';
    import {UTCTimestampToESTString} from '$lib/core/timestamp'
//    let similarInstance: Writable<SimilarInstance> = writable({});
    import { privateRequest} from '$lib/core/backend';
    import {embedInstance} from "$lib/features/entry.svelte";
    import {newStudy} from '$lib/features/study.svelte';
    import {newJournal} from '$lib/features/journal.svelte';
    import {newSample} from '$lib/features/sample.svelte'
    import { get, writable } from 'svelte/store';
    import {querySimilarInstances} from '$lib/utils/similar.svelte'
    import {startReplay} from '$lib/utils/stream'
    interface RightClickQuery {
        x?: number;
        y?: number;
        source?: Source
        instance: Instance
        status: "inactive" | "active" | "initializing" | "cancelled" | "complete"
        result: RightClickResult
    }
    export type RightClickResult = "edit" | "embed" | "alert" | "embedSimilar" | "none" | "flag"
    type Source = "chart" | "embedded" | "similar" | "list" | "header"
    const inactiveRightClickQuery: RightClickQuery = {
        status:"inactive",
        result: "none",
        instance: {}
    }

    let rightClickQuery: Writable<RightClickQuery> = writable(inactiveRightClickQuery)

    export async function queryInstanceRightClick(event:MouseEvent,instance:Instance,source:Source):Promise<RightClickResult>{
        event.preventDefault()
        console.log(instance,source)
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
        //if (rightClickMenu && !rightClickMenu.contains(event.target as Node)) {
            closeRightClickMenu()
        //}
    }
    function handleKeyDown(event:KeyboardEvent):void{
        if (event.key == "Escape"){
            closeRightClickMenu()
        }
    }
    function closeRightClickMenu():void{
        rightClickQuery.update((v:RightClickQuery)=>{
            v.status = "complete"
            return v
        })
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
    function completeRequest(result:RightClickResult,func:Function|null=null){
        rightClickQuery.update((v:RightClickQuery)=>{
            v.status = "complete"
            v.result = result
            return v
        })
        if (func !== null)
        {
            func(rightClickQuery.instance)
        }
    }
    function embedSimilar():void{
        rightClickQuery.update((v:RightClickQuery)=>{
            v.result = "embedSimilar"
            return v
        })
    }


</script>
{#if $rightClickQuery.status === "active"}
    <div bind:this={rightClickMenu} class="context-menu" style="top: {$rightClickQuery.y}px; left: {$rightClickQuery.x}px;">
        <div class="menu-item">{$rightClickQuery.instance.ticker} {UTCTimestampToESTString($rightClickQuery.instance.timestamp)} </div>
        <div class="menu-item"><button on:click={()=>newStudy(get(rightClickQuery).instance)}> Add to Study </button></div>
        <!--<div><button on:click={()=>newSample(get(rightClickQuery).instance)}> Add to Sample </button></div>
        <div><button on:click={()=>newJournal(get(rightClickQuery).instance)}> Add to Journal </button></div>-->
        <div class="menu-item"><button on:click={(event)=>querySimilarInstances(event,get(rightClickQuery).instance)}> Similar Instances </button></div>
        <!--<div><button on:click={getStats}> Instance Stats </button></div>
        <div><button on:click={replay}> Replay </button></div>-->
        <div class="menu-item"><button on:click={()=>startReplay($rightClickQuery.instance)}>Begin Replay</button></div>
        {#if $rightClickQuery.source === "chart"}
            <!--<div><button on:click={()=>completeRequest("alert")}>Add Alert </button></div>-->
            <div class="menu-item"><button on:click={()=>embedInstance(get(rightClickQuery).instance)}> Embed </button></div>
        {:else if $rightClickQuery.source === "embedded"}
            <div class="menu-item"><button on:click={()=>completeRequest("edit")}> Edit </button></div>
            <!--<div><button on:click={()=>completeRequest("embdedSimilar")}> Embed Similar </button></div>-->
        {:else if $rightClickQuery.source === "list"}
            <div class ="menu-item"><button on:click={()=>completeRequest("flag")}>{$rightClickQuery.instance.flagged ? "Unflag" : "Flag"}</button></div>
        {/if}
    </div>
{/if}

<style>
    @import "$lib/core/colors.css";

    /* Style the context menu similarly to the popup */
    .context-menu {
        display: flex;
        flex-direction: column;
        position: absolute;
        background-color: var(--c2);
        border: 1px solid var(--c4);
        z-index: 1000;
        padding: 10px;
        width: 250px;
        box-shadow: 0px 0px 20px rgba(0, 0, 0, 0.7);
        border-radius: 8px;
        color: var(--f1);
    }

    /* Add styling for the buttons inside the menu */
    .menu-item {
        padding: 10px;
        border-bottom: 1px solid var(--c4);
        display: flex;
        justify-content: space-between;
        color: var(--f1);
        cursor: pointer;
    }

    .menu-item:last-child {
        border-bottom: none;
    }

    button {
        background-color: var(--c3);
        color: var(--f1);
        border: none;
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
        width: 100%;
    }

    button:hover {
        background-color: var(--c3-hover);
    }

    .menu-item:hover {
        background-color: var(--c1);
    }

    .hidden {
        display: none;
    }
</style>
