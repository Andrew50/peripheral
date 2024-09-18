<script lang='ts'>

    import List from '$lib/utils/list.svelte'
    import type {Writable} from 'svelte/store'
    import {writable,get} from 'svelte/store'
    import type {Instance,Watch,Watchlist} from '$lib/core/types'
    import {onMount} from "svelte"
    import {privateRequest} from "$lib/core/backend"
    let activeList: Writable<Watch[]> = writable([])
    import {queryInstanceInput} from '$lib/utils/input.svelte'

    let watchlists: Writable<Watchlist[]> = writable([])
    let newWatchlistName="";
    let currentWatchlistId = 1;


    onMount(()=>{
        privateRequest<Watchlist[]>("getWatchlists",{})
        .then((v:Watchlist[])=>{
            console.log(v)
            watchlists.set(v)
        })
        selectWatchlist("1")
    })


    function addInstance(){
        const inst = {ticker:"",timestamp:0}
        queryInstanceInput(["ticker"],inst).then((i:Instance)=>{
            if (!get(activeList).find((l:Instance)=>l.ticker === i.ticker)){
                activeList.update((v:Watch[])=>{
                    privateRequest<number>("newWatchlistItem",{watchlistId:currentWatchlistId,securityId:i.securityId})
                    if (!Array.isArray(v)){
                        return [i]
                    }
                    return [i,...v]
                })
            }
           setTimeout(()=>{
                addInstance()
            },10)
        }).catch()
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
    function deleteItem(item:Watch){
        if (!item.watchlistItemId){
            throw new Error("missing id on delete")
        }
        privateRequest<void>("deleteWatchlistItem",{watchlistItemId:item.watchlistItemId})
    }
    function selectWatchlist(watchlistIdString:string){
        const watchlistId = parseInt(watchlistIdString)
        privateRequest<Watch[]>("getWatchlistItems",{watchlistId:watchlistId})
        .then((v:Watch[])=>{
            activeList.set(v)
        })
    }

</script>


<div class="buttons-container">

{#if Array.isArray($watchlists)}
    <select id="watchlists" on:change={(event) => selectWatchlist(event.target.value)}>
    {#each $watchlists as watchlist}
        <option value={watchlist.watchlistId}>
            {watchlist.watchlistName}
        </option>
    {/each}
</select>
    {/if}
    <button class="button" on:click={newWatchlist}>New</button>
    <input bind:value={newWatchlistName} placeholder="Name"/>
    <button class="button" on:click={addInstance}>
      Add
    </button>
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



