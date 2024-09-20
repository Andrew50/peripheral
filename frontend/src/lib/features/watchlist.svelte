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
    let container: HTMLDivElement;
    onMount(()=>{
        selectWatchlist(flagWatchlistId)
    })

    $: if (container){
        container.addEventListener("keydown",handleKey)
    }
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
                            return [...v,i]
                        }
                    })
                })
            }
           setTimeout(()=>{
                addInstance()
            },10)
        })
    }

    function handleKey(event: KeyboardEvent) {
        if (event.key == "Tab" || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
            addInstance()
        }
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
    function deleteWatchlist(id:number){
        privateRequest<void>("deleteWatchlist",{watchlistId:id})
        .then(()=>{
            watchlists.update((v:Watchlist[])=>{
                return v.filter((v:Watchlist)=>v.watchlistId !== id)
            })
            if (id === flagWatchlistId){
                flagWatchlist.set([])
            }
        })
    }

</script>

<div tabindex="-1" class="container" bind:this={container}>
    <div  class="buttons-container">

    {#if Array.isArray($watchlists)}
      <div class="watchlist-container">
        <select id="watchlists" bind:value={currentWatchlistId} on:change={(event) => selectWatchlist(event.target.value)}>
            {#each $watchlists as watchlist}
                <option value={watchlist.watchlistId}>
                    {watchlist.watchlistName}
                </option>
            {/each}
        </select>
            <button class="button delete-watchlist" on:click={() => deleteWatchlist(currentWatchlistId)}>x</button>
      </div>
    {/if}
    <button class="button" on:click={newWatchlist}>New</button>
    <input class="input" bind:value={newWatchlistName} placeholder="Name"/>
    </div>

    <List parentDelete={deleteItem} columns={["ticker", "change"]} list={activeList}/>
</div>

<style>
  @import "$lib/core/colors.css";

    .container {
        outline: none;
        height: 100%;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        width:100%;
        justify-content: flex-start;
    }
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
  }
  .input {
    padding: 10px 20px;
    background-color: var(--c2);
    border: 1px solid var(--c4);
    border-radius: 8px;
    cursor: pointer;
    font-size: 16px;

  }

  .button:hover {
    background-color: var(--c3-hover);
  }

  /* Style for select dropdown */
  .watchlist-container {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  select {
    padding: 10px;
    background-color: var(--c1);
    color: var(--f1);
    border: 1px solid var(--c4);
    border-radius: 8px;
    font-size: 16px;
    transition: background-color 0.3s ease;
  }

  select:hover {
    background-color: var(--c2);
  }

  /* Style for delete watchlist button */
  .delete-watchlist {
    background-color: var(--c3);
    color: var(--f1);
    border: none;
    padding: 5px 10px;
    cursor: pointer;
    border-radius: 8px;
    font-size: 16px;
  }

  .delete-watchlist:hover {
    background-color: var(--c3-hover);
  }
  
</style>
