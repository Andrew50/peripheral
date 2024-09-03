<!-- instance.svlete -->
<script lang="ts" context="module">
    import {changeChart} from './chart.svelte'
    import { writable } from 'svelte/store';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '$lib/api/backend';
    import {queryInstanceRightClick} from './rightClick.svelte'
    interface SimilarInstance {
        x: number
        y: number
        similarInstances: Instance[]
        status: "active" | "inactive"
    }
    let similarInstance: Writable<SimilarInstance> = writable({});
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
</style>

