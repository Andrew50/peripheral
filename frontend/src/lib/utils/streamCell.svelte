<script lang='ts'>
    import { onMount, onDestroy } from 'svelte';
    import type { Writable } from 'svelte/store';
    import {writable} from 'svelte/store'
    import { getStream } from '$lib/utils/stream';
    import { privateRequest } from '$lib/core/backend';
    import type { TradeData,Instance } from '$lib/core/types';
    
    export let instance: Instance;
    let releaseStream: Function = () => {}
    let unsubscribe: Function = () => {}
    let change = writable(0)// Initialize `change` with a default value
    let prevClose: number | null = null;  // Initialize as null to handle when it's not available yet
    let priceStream: Writable<TradeData>;
    let prevCloseStream: Writable<number>;
    interface ChangeStore {
        price?: number
        prevClose?: number
        change: string
    }
    let changeStore = writable<ChangeStore>({change:"--"})
    
    onMount(() => {
        [priceStream, releaseStream] = getStream<TradeData>(instance, "fast")
        const [p, u] = getStream<number>(instance,"close")
        prevCloseStream = p
        unsubscribe = u
        priceStream.subscribe((v) => {
            //console.log(v)
            if (v.price){
                changeStore.update((s:ChangeStore)=>{
                    s.price = v.price
                    if (s.price && s.prevClose) s.change = getChange(s.price,s.prevClose)
                    return s
                })
            }
        });
        prevCloseStream.subscribe((v:number)=>{
            //console.log(v)
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

<style>
    .green {
        color: green;
    }
    .red {
        color: red;
    }
    .white {
        color: white;
    }
</style>

