<script lang="ts" context="module">
export interface ActiveStream {
    securityId: number;
    streamType: "fast" | "slow" | "quote";
    openCount: number;
}

    import {changeChart} from '$lib/features/chart/interface'
    import { writable } from 'svelte/store';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '$lib/core/types';
    import {privateRequest} from '$lib/core/backend';
    import {queryInstanceRightClick} from './rightClick.svelte'
    type Status = "active" | "inactive"
    
    interface SimilarQuery {
        x: number
        y: number
        similarInstances: Instance[]
        status: Status
    }
    
    const inactiveSimilarQuery = {x:0,y:0,similarInstances:[],status:"inactive" as Status}
    let similarQuery: Writable<SimilarQuery> = writable(inactiveSimilarQuery);
    
    export function querySimilarInstances(event:MouseEvent,baseIns:Instance):void{
        privateRequest<Instance[]>("getSimilarInstances",{ticker:baseIns.ticker, securityId:baseIns.securityId, timeframe:baseIns.timeframe, timestamp:baseIns.timestamp})
        .then((v:Instance[])=>{
            const simInst: SimilarQuery = {
                x: event.clientX,
                y: event.clientY,
                status: "active",
                similarInstances: v
            }
            similarQuery.set(simInst)
        })
    }
</script>

<script lang="ts">
    let menu: HTMLElement;
    import {browser} from '$app/environment'
    import {onMount} from 'svelte'

    let startX: number;
    let startY: number;
    let initialX: number;
    let initialY: number;
    let isDragging = false;

    onMount(() => {
        similarQuery.subscribe((v:SimilarQuery) => {
            if (browser){
                if (v.status === "active"){
                    document.addEventListener('click',handleClick);
                    document.addEventListener('keydown',handleKeyDown);
                    document.addEventListener('mousedown',handleMouseDown);
                } else if(v.status == "inactive") {
                    document.removeEventListener('click',handleClick);
                    document.removeEventListener('keydown',handleKeyDown);
                    document.removeEventListener('mousedown',handleMouseDown);
                }
            }
        })
    })

    function handleClick(event:MouseEvent):void {
        if (menu && !menu.contains(event.target as Node)) {
            closeMenu();
        }
    }

    function handleKeyDown(event:KeyboardEvent):void {
        if (event.key == "Escape") {
            closeMenu();
        }
    }

    function closeMenu():void {
        similarQuery.update((v:SimilarQuery) => {
            return inactiveSimilarQuery;
        })
    }

    function handleMouseDown(event:MouseEvent):void {
        if (event.target && (event.target as HTMLElement).classList.contains('context-menu')) {
            startX = event.clientX;
            startY = event.clientY;
            initialX = $similarQuery.x;
            initialY = $similarQuery.y;
            isDragging = true;
            document.addEventListener('mousemove',handleMouseMove);
            document.addEventListener('mouseup',handleMouseUp);
        }
    }

    function handleMouseMove(event:MouseEvent):void {
        if (isDragging) {
            const deltaX = event.clientX - startX;
            const deltaY = event.clientY - startY;
            similarQuery.update(v => {
                return {...v, x: initialX + deltaX, y: initialY + deltaY};
            });
        }
    }

    function handleMouseUp():void {
        isDragging = false;
        document.removeEventListener('mousemove',handleMouseMove);
        document.removeEventListener('mouseup',handleMouseUp);
    }
</script>

{#if $similarQuery.status === "active"}
    <div class="context-menu" bind:this={menu} style="top: {$similarQuery.y}px; left: {$similarQuery.x}px;">
        <div class="content">
            {#if $similarQuery.similarInstances && $similarQuery.similarInstances.length > 0}
                <table>
                    <thead>
                        <tr>
                            <th>Ticker</th>
                        </tr>
                    </thead>
                    <tbody>
                        {#each $similarQuery.similarInstances as instance} 
                            <tr>
                                <td on:click={()=>changeChart(instance, true)} 
                                on:contextmenu={(e)=>{e.preventDefault();
                                closeMenu();
                                queryInstanceRightClick(e,instance,"similar")}}>{instance.ticker}</td>
                            </tr>
                        {/each}
                    </tbody>
                </table>
            {/if}
        </div>
    </div>
{/if}

<style>
    @import '$lib/core/colors.css';
    .context-menu {
        position: absolute;
        background-color: var(--c2);
        border: 1px solid var(--c4);
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        padding: 20px;
        width: 200px;
        height: 500px; /* Fixed height */
        border-radius: 4px;
        overflow: hidden;
    }

    .content {
        width: 100%;
        height: 100%;
        overflow: hidden;
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    table {
        width: 100%;
        border-collapse: collapse;
        overflow-y: auto;
    }

    th, td {
        padding: 10px;
        text-align: left;
    }

    th {
        background-color: var(--c1);
        color: var(--f1);
    }

    tr {
        border-bottom: 1px solid var(--c4);
    }

    tr:hover {
        background-color: var(--c1);
    }
</style>

