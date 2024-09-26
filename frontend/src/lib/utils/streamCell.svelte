<script lang='ts'>
    import { onMount, onDestroy } from 'svelte';
    import type { Writable } from 'svelte/store';
    import {writable} from 'svelte/store'
    import { getStream } from '$lib/utils/stream';
    import type { TradeData,Instance } from '$lib/core/types';
    export let instance: Instance;
    let releaseStream: Function = () => {}
    let unsubscribe: Function = () => {}
    let priceStream: Writable<TradeData>;
    let prevCloseStream: Writable<number>;
    interface ChangeStore {
        price?: number
        prevClose?: number
        change: string
    }
    let changeStore = writable<ChangeStore>({change:"--"})
    
    onMount(() => {
        const [ps,rs] = getStream<TradeData>(instance, "slow")
        priceStream = ps 
        releaseStream = rs
        const [p, u] = getStream<number>(instance,"close")
        prevCloseStream = p
        unsubscribe = u
        priceStream.subscribe((v) => {
            v = v[v.length-1]
            if (v && v.price){
                changeStore.update((s:ChangeStore)=>{
                    s.price = v.price
                    if (s.price && s.prevClose) s.change = getChange(s.price,s.prevClose)
                    return s
                })
            }
        });
        prevCloseStream.subscribe((v:number)=>{
            changeStore.update((s:ChangeStore)=>{
                s.prevClose = v
                if (s.price && s.prevClose) s.change = getChange(s.price,s.prevClose)
                return s
            })
        })
    });

    onDestroy(() => {
        releaseStream();
        unsubscribe();
    });
    function getChange(price: number, prevClose: number): string {
        if (!price || !prevClose) return "--"
        return ((price / prevClose - 1) * 100).toFixed(2) + "%"
    }
</script>

<td class={$changeStore.price - $changeStore.prevClose < 0 ? "red" : $changeStore.change === "--"? "white":"green"}> {$changeStore.change} </td>


