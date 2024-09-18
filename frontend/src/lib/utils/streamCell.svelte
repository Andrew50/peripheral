<script lang='ts'>
    import { onMount, onDestroy } from 'svelte';
    import type { Writable } from 'svelte/store';
    import {writable} from 'svelte/store'
    import { getStream } from '$lib/utils/stream';
    import { privateRequest } from '$lib/core/backend';
    import type { TradeData } from '$lib/core/types';
    
    export let ticker: string;
    let releaseStream: Function;
    let change = writable(0)// Initialize `change` with a default value
    let prevClose: number | null = null;  // Initialize as null to handle when it's not available yet
    let priceStream: Writable<TradeData>;
    
    onMount(() => {
        Promise.all([
            privateRequest<number>("getPrevClose", { ticker }),
            privateRequest<number>("getLastTrade", { ticker })
        ]).then(([prevCloseValue, lastTradeValue]) => {
            const cha = (getChange(lastTradeValue, prevCloseValue));
            console.log(cha)
            change.set(cha)
            prevClose = prevCloseValue
        }).catch((err) => {
            console.error('Error loading initial values:', err);
        });
        [priceStream, releaseStream] = getStream(ticker, "fast")
        priceStream.subscribe((v) => {
            if (prevClose !== null) {  // Ensure prevClose is available before calculating change
                if (v.price){
                    change.set(getChange(v.price, prevClose));
                }
            }
        });
    });

    onDestroy(() => {
        releaseStream();
    });

    // Utility function to calculate the percentage change
    function getChange(price: number, prevClose: number) {
        return (price / prevClose - 1) * 100;
    }
</script>

<td class={$change < 0 ? "red" : "green"}> {$change.toFixed(2) ?? ""}% </td>

<style>
    .green {
        color: green;
    }
    .red {
        color: red;
    }
</style>

