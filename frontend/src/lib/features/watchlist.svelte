<script lang='ts'>

    import List from '$lib/utils/list.svelte'
    import type {Watch} from '$lib/utils/list.svelte'
    import type {Writable} from 'svelte/store'
    import {writable} from 'svelte/store'
    import type {Instance} from '$lib/core/types'
    let watchlist: Writable<Watch[]> = writable([])
    import {queryInstanceInput} from '$lib/utils/input.svelte'


    function addInstance(){
        const inst = {ticker:"",timestamp:0}
        queryInstanceInput(["ticker"],inst).then((i:Instance)=>{
            watchlist.update((v:Instace[])=>{
                return [i,...v]
            })
           setTimeout(()=>{
                addInstance()
            },10)
        }).catch()
    }

</script>


<div class="buttons-container">
    <button class="button" on:click={addInstance}>
      Add
    </button>
</div>
<List columns={["ticker", "change"]} list={watchlist}/>


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



