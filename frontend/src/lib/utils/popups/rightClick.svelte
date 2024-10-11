 <!-- rightClick.svlete -->
<script lang="ts" context="module">
    import '$lib/core/global.css'
    import type { Writable } from 'svelte/store';
    import type {Instance,Setup} from '$lib/core/types';
    import {UTCTimestampToESTString} from '$lib/core/timestamp'
    import {flagSecurity} from '$lib/utils/flag'
    import {embedInstance} from "$lib/utils/modules/entry.svelte";
    import {newStudy} from '$lib/features/study.svelte';
    import {get, writable } from 'svelte/store';
    import {setSample} from '$lib/features/setups/interface'
    import {querySimilarInstances} from '$lib/utils/popups/similar.svelte'
    import {querySetup} from '$lib/utils/popups/setup.svelte'
    import {startReplay} from '$lib/utils/stream/interface'
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
    import {entryOpen} from '$lib/core/stores'
    import {browser} from '$app/environment'
    import {onMount,tick} from 'svelte'
    let rightClickMenu: HTMLElement;
    onMount(()=>{
        rightClickQuery.subscribe( async(v:RightClickQuery) => {
            if (browser){
                if (v.status === "initializing"){
                    document.addEventListener('click',handleClick)
                    document.addEventListener('keydown', handleKeyDown)
                    await tick()
                    const menuRect = rightClickMenu.getBoundingClientRect();
                    const windowWidth = window.innerWidth;
                    const windowHeight = window.innerHeight;
                    console.log(v.y)
                    let newX = v.x
                    let newY = v.y
                    const halfMenuWidth = Math.floor(menuRect.width * .5)
                    const halfMenuHeight = Math.floor(menuRect.height * .5)
                    if (v.x && v.y){
                        if (v.x + halfMenuWidth > windowWidth) {
                            newX = windowWidth - halfMenuWidth - 40; // Add a small margin from the edge
                        }else if(v.x - halfMenuWidth< 0){
                            newX = halfMenuWidth + 40
                        }
                        if (v.y + menuRect.height > windowHeight) {
                            newY = windowHeight - halfMenuHeight - 40; // Add a small margin from the edge
                        }else if (v.y - halfMenuWidth < 0){
                            newY = halfMenuWidth + 40
                        }
                    }
                    console.log(v.y)
                    v.status = "active"
                    rightClickQuery.update((c:RightClickQuery)=>{
                        return {
                            ...c,
                            x:newX,
                            y:newY,
                            status: "active"
                        }
                    })
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

    function sSample(event:MouseEvent){
        querySetup(event).then((v:number)=>{
            if (v== null)return
            setSample(v,$rightClickQuery.instance)
        })

    }

</script>
{#if ["initializing","active"].includes($rightClickQuery.status)}
    <div bind:this={rightClickMenu} class="popup-container" style="top: {$rightClickQuery.y}px; left: {$rightClickQuery.x}px;">
        <div >{$rightClickQuery.instance.ticker} {UTCTimestampToESTString($rightClickQuery.instance.timestamp)} </div>
        <div ><button on:click={()=>newStudy(get(rightClickQuery).instance)}> Add to Study </button></div>
        <div><button on:click={(event)=>sSample(event)}> Add to Sample </button></div>
        <!--<div><button on:click={()=>newJournal(get(rightClickQuery).instance)}> Add to Journal </button></div>-->
        <div ><button on:click={(event)=>querySimilarInstances(event,get(rightClickQuery).instance)}> Similar Instances </button></div>
        <!--<div><button on:click={getStats}> Instance Stats </button></div>-->
        <div ><button on:click={()=>startReplay($rightClickQuery.instance)}>Begin Replay</button></div>
        {#if $entryOpen}
            <div ><button on:click={()=>embedInstance(get(rightClickQuery).instance)}> Embed </button></div>
        {/if}
        {#if $rightClickQuery.source === "chart"}
            <div><button on:click={()=>completeRequest("alert")}>Add Alert </button></div>
        {:else if $rightClickQuery.source === "embedded"}
            <div ><button on:click={()=>completeRequest("edit")}> Edit </button></div>
            <!--<div><button on:click={()=>completeRequest("embdedSimilar")}> Embed Similar </button></div>-->
        {:else if $rightClickQuery.source === "list"}
            <div><button on:click={()=>flagSecurity($rightClickQuery.instance)}>{$rightClickQuery.instance.flagged ? "Unflag" : "Flag"}</button></div>
        {/if}
    </div>
{/if}

<style>
    .popup-container {
        width: 180px;
    }
    button {
        width: 100%;
    }
</style>
