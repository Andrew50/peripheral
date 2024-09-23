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
    import '$lib/core/global.css'



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
        const target = event.target as HTMLElement;
        if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable) {
            // Do nothing if the target is an input, textarea, or contentEditable element
            return;
        }
        if (event.key == "Tab" || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
            addInstance()
        }
    }

    function newWatchlist(){
        if (newWatchlistName === "") return;
        privateRequest<number>("newWatchlist",{watchlistName:newWatchlistName})
        .then((newId:number)=>{
            watchlists.update((v:Watchlist[])=>{
                const w: Watchlist = {
                    watchlistName: newWatchlistName,
                    watchlistId: newId
                }
                newWatchlistName = ""
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

<div tabindex="-1" class="feature-container" bind:this={container}>
    <div  class="controls-container">
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
    <input class="input" on:keydown={(event)=>{if (event.key == "Enter"){newWatchlist()}}} bind:value={newWatchlistName} placeholder="Name"/>
    </div>

    <List parentDelete={deleteItem} columns={["ticker", "change"]} list={activeList}/>
</div>

<style>
  .watchlist-container {
    display: flex;
    align-items: center;
    gap: 10px;
  }


  
</style>
