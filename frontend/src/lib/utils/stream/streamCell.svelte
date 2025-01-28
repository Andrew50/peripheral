<script lang='ts'>
    import { onMount, onDestroy } from 'svelte';
    import {writable} from 'svelte/store'
    import { addStream } from '$lib/utils/stream/interface';
    import type { TradeData,Instance,CloseData } from '$lib/core/types';
    
    export let instance: Instance;
    export let type: 'price' | 'change' = 'change';
    
    let releaseSlow: Function = () => {}
    let releaseClose: Function = () => {}
    
    interface ChangeStore {
        price?: number
        prevClose?: number
        change: string
    }
    let changeStore = writable<ChangeStore>({change:"--"})
    
    onMount(() => {
        releaseSlow = addStream<TradeData>(instance, "slow", (v:TradeData) => {
            if (v && v.price){
                changeStore.update((s:ChangeStore)=>{
                    s.price = v.price
                    if (s.price && s.prevClose) s.change = getChange(s.price,s.prevClose)
                    console.log(s.change)
                    return s
                })
            }
        });
        releaseClose = addStream<CloseData>(instance,"close",(v:CloseData)=>{
            changeStore.update((s:ChangeStore)=>{
                s.prevClose = v.price
                if (s.price && s.prevClose) s.change = getChange(s.price,s.prevClose)
                return s
            })
        })
    })
    
    onDestroy(() => {
        releaseClose();
        releaseSlow();
    });
    
    function getChange(price: number, prevClose: number): string {
        if (!price || !prevClose) return "--"
        return ((price / prevClose - 1) * 100).toFixed(2) + "%"
    }
</script>
<td class={type === 'change' ? 
    ($changeStore.price - $changeStore.prevClose < 0 ? "red" : $changeStore.change === "--"? "white":"green") 
    : type === 'change %' ? ($changeStore.change.includes("-") ? "red" : "green") : ""}>

    {#if type === 'change'}
        {($changeStore.price - $changeStore.prevClose).toFixed(2)}
    {:else if type === 'price'}
        {$changeStore.price?.toFixed(2) ?? '--'}
    {:else if type === 'change %'}
        {$changeStore.change}
    {:else}
        {'--'}
    {/if}
</td>


