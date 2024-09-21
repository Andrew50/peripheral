<script lang='ts'>
    import { onMount, onDestroy } from 'svelte';
    import type { Writable } from 'svelte/store';
    import {writable} from 'svelte/store'
    import { getStream } from '$lib/utils/stream';
    import { privateRequest } from '$lib/core/backend';
    import type { TradeData } from '$lib/core/types';
    
    export let securityId: string;
    let releaseStream: Function;
    let unsubscribe: Function
    let change = writable(0)// Initialize `change` with a default value
    let prevClose: number | null = null;  // Initialize as null to handle when it's not available yet
    let priceStream: Writable<TradeData>;
    let prevCloseStream: Writable<number>;
    interface ChangeStore {
        price?: number
        prevClose?: number
        change: number 
    }
    let changeStore = writable<ChangeStore>({change:0})
    
    onMount(() => {
        [priceStream, releaseStream] = getStream<TradeData>(securityId, "fast")
        [prevCloseStream, unsubscribe] = getStream<number>(securityId,"close")
        priceStream.subscribe((v) => {
            if (prevClose !== null) {  // Ensure prevClose is available before calculating change
                if (v.price){
                    changeStore.update((s:ChangeStore)=>{
                        s.price = v.price
                        if (s.price && s.prevClose) s.change = getChange(s.price,s.prevClose)
                        return s
                    })
                }
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

    function getChange(price: number, prevClose: number) {
        return ((price / prevClose - 1) * 100)
    }
</script>

<td class={$changeStore.change < 0 ? "red" : "green"}> {$changeStore.change.toFixed(2) ?? ""}% </td>

<style>
    .green {
        color: green;
    }
    .red {
        color: red;
    }
</style>

