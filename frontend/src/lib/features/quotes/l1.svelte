<script lang='ts'>
    import { onMount, onDestroy } from 'svelte';
    import type { Writable } from 'svelte/store';
    import type { QuoteData, Instance } from '$lib/core/types';
    import { getStream } from '$lib/utils/stream';

    export let instance: Writable<Instance>;
    let store: Writable<QuoteData>;
    let release: Function = () => {};

    let previousBidPrice = 0;
    let previousAskPrice = 0;
    let bidPriceChange = 'no-change';  // Can be 'increase', 'decrease', or 'no-change'
    let askPriceChange = 'no-change';  // Can be 'increase', 'decrease', or 'no-change'

    instance.subscribe((inst: Instance) => {
        release();
        const [s, r] = getStream(inst, "quote");
        store = s;
        release = r;

        store.subscribe(($store: QuoteData) => {
            // Check bid price change
            if ($store.bidPrice !== undefined && $store.bidPrice !== previousBidPrice) {
                bidPriceChange = $store.bidPrice > previousBidPrice ? 'increase' : 'decrease';
                previousBidPrice = $store.bidPrice;
            }

            // Check ask price change
            if ($store.askPrice !== undefined && $store.askPrice !== previousAskPrice) {
                askPriceChange = $store.askPrice > previousAskPrice ? 'increase' : 'decrease';
                previousAskPrice = $store.askPrice;
            }
        });
    });

    onDestroy(() => {
        release();
    });
</script>

<div class="quote-container">
    <div class="quote-row">
        <div class="price">
            <span class="label">Ask:</span> 
            <span class="value {askPriceChange}">{$store.askPrice?.toFixed(2) ?? "--"}</span>
        </div>
        <div class="size">
            <span class="value">x {$store.askSize ?? "--"}</span>
        </div>
    </div>
    <div class="quote-row">
        <div class="price">
            <span class="label">Bid:</span> 
            <span class="value {bidPriceChange}">{$store.bidPrice?.toFixed(2) ?? "--"}</span>
        </div>
        <div class="size">
            <span class="value">x {$store.bidSize ?? "--"}</span>
        </div>
    </div>
</div>

<style>
    .quote-container {
        font-family: Arial, sans-serif;
        font-size: 14px;
        color: white;
        background-color: black;
        width: 100%;
        border: 1px solid #333;
        margin: 0 auto;
        border-radius: 5px;
    }

    .quote-row {
        display: flex;
        padding: 10px;
        flex-direction: column;
        margin-bottom: 2px;
    }

    .price {
        display: flex;
        justify-content: space-between;
        margin-bottom: 2px;
    }

    .size {
        text-align: right;
    }

    .label {
        font-weight: bold;
        color: #ccc;
    }

    .value {
        font-family: monospace;
        color: white;
    }

    /* Styling for bid/ask colors based on price change */
    .increase {
        color: green;
    }

    .decrease {
        color: red;
    }

    .no-change {
        color: white;
    }
</style>

