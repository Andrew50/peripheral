<script lang='ts'>

    import List from '$lib/utils/list.svelte'
    import type {Writable} from 'svelte/store'
    import {writable,get} from 'svelte/store'
    import type {Instance,Watchlist} from '$lib/core/types'
    import {onMount} from "svelte"
    import {privateRequest} from "$lib/core/backend"
    let activeList: Writable<Instance[]> = writable([])
    import {queryInstanceInput} from '$lib/utils/input.svelte'
    import {flagWatchlistId,watchlists, flagWatchlist} from '$lib/core/stores'


    let newWatchlistName="";
    let currentWatchlistId: number;


    onMount(()=>{
        selectWatchlist(flagWatchlistId)
    })


    function addInstance(){
        const inst = {ticker:"",timestamp:0}
        queryInstanceInput(["ticker"],inst).then((i:Instance)=>{
            const aList = get(activeList)
            const empty = !Array.isArray(aList) 
            if (empty || !aList.find((l:Instance)=>l.ticker === i.ticker)){
                privateRequest<number>("newWatchlistItem",{watchlistId:currentWatchlistId,securityId:i.securityId})
                .then((watchlistItemId:number)=>{
                    activeList.update((v:Instance[])=>{
                        i.watchlistItemId = watchlistItemId
                        if (empty){
                            return [i]
                        }else{
                            return [i,...v]
                        }
                    })
                })
            }
           setTimeout(()=>{
                addInstance()
            },10)
        })
    }

    function newWatchlist(){
        privateRequest<number>("newWatchlist",{watchlistName:newWatchlistName})
        .then((newId:number)=>{
            watchlists.update((v:Watchlist[])=>{
                const w: Watchlist = {
                    watchlistName: newWatchlistName,
                    watchlistId: newId
                }
                if(!Array.isArray(v)){
                    return [w]
                }
                return [w,...v]
            })
        })
    }
    function deleteItem(item:Instance){
        if (!item.watchlistItemId){
            throw new Error("missing id on delete")
        }
        privateRequest<void>("deleteWatchlistItem",{watchlistItemId:item.watchlistItemId})
    }
    function selectWatchlist(watchlistIdString:string){
        const watchlistId = parseInt(watchlistIdString)
        if (watchlistId === flagWatchlistId){
            activeList = flagWatchlist
        }else{
            activeList = writable<Instance[]>([])
        }
        currentWatchlistId = watchlistId
        privateRequest<Instance[]>("getWatchlistItems",{watchlistId:watchlistId})
        .then((v:Instance[])=>{
            activeList.set(v)
        })
    }

</script>


<div class="buttons-container">

{#if Array.isArray($watchlists)}
    <select id="watchlists" bind:value={currentWatchlistId} on:change={(event) => selectWatchlist(event.target.value)}>
        {#each $watchlists as watchlist}
            <option value={watchlist.watchlistId}>
                {watchlist.watchlistName}
            </option>
        {/each}
    </select>
    {/if}
    <button class="button" on:click={newWatchlist}>New</button>
    <input bind:value={newWatchlistName} placeholder="Name"/>
    <button class="button" on:click={addInstance}> Add </button>
</div>
<List parentDelete={deleteItem} columns={["ticker", "change"]} list={activeList}/>


<style>
  @import "$lib/core/colors.css";
  .buttons-container {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 20px;
}
    .button {
        padding: 10px 20px;
        background-color: var(--c2);
        border: 1px solid var(--c4);
        border-radius: 8px;
        cursor: pointer;
        transition: background-color 0.3s ease;
        font-size: 16px;
        display: inline-block;
    }

    .button:hover {
        background-color: var(--c3-hover);
    }
</style>



