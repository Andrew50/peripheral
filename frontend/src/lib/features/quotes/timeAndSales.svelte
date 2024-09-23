<script lang="ts">
    import { onDestroy, onMount } from 'svelte';
    import type { Writable } from 'svelte/store';
    import type { TradeData, QuoteData ,Instance} from '$lib/core/types';
    import { getStream } from '$lib/utils/stream';
    import '$lib/core/global.css'
    import {settings} from '$lib/core/stores'
    import {get} from 'svelte/store'
    import {privateRequest} from "$lib/core/backend"
    export let instance: Writable<Instance>;
    let store: Writable<TradeData>;
    let quoteStore: Writable<QuoteData>;
    let releaseTrade: Function = () => {};
    let releaseQuote: Function = () => {};
    let unsubscribeTrade: Function = () => {};
    let unsubscribeQuote: Function = () => {};
    interface TaS extends TradeData {
        color: string
        exchangeName: string
    }
    type Exchanges = {[exchangeId:number]:string}
    let exchanges: Exchanges = {}
    let allTrades: TaS[] = [];
    let currentBid = 0;
    let currentAsk = 0;
    const maxLength = 20;
    let prevSecId = -1;
    let divideTaS = get(settings).divideTaS
    let filterTaS = get(settings).filterTaS
    onMount(()=>{
        privateRequest<Exchanges>("getExchanges",{})
        .then((v:Exchanges)=>{
            exchanges = v
        })
    })

    instance.subscribe((instance: Instance) => {
        if (!instance.securityId || instance.securityId === prevSecId) return;
        unsubscribeTrade();
        releaseTrade();
        unsubscribeQuote();
        releaseQuote();
        const [s, r] = getStream<TradeData>(instance, "fast");
        store = s;
        releaseTrade = r;
        const [qs, qr] = getStream<QuoteData>(instance,"quote");
        quoteStore = qs;
        releaseQuote = qr;
        allTrades = []
        unsubscribeTrade = store.subscribe((newTrade: TradeData) => {
            if (newTrade.timestamp !== undefined && newTrade.timestamp !== 0) {
                if(!filterTaS){
                    if (newTrade.size < 100) return;
                }

                if (divideTaS){
                    newTrade.size = Math.floor(newTrade.size / 100)
                }
                console.log(newTrade)
                const exchangeName = exchanges[newTrade.exchange]
                const newRow: TaS = {color:getPriceColor(newTrade.price),...newTrade,exchangeName:exchangeName}
                allTrades = [newRow,...allTrades].slice(0,maxLength);
            }
        });
        unsubscribeQuote = quoteStore.subscribe((quote) => {
            currentBid = quote.bidPrice;
            currentAsk = quote.askPrice;
        });
        prevSecId = instance.securityId ?? -1
    });

    onDestroy(() => {
        unsubscribeTrade();
        releaseTrade();
        unsubscribeQuote();
        releaseQuote();
    });

    function getPriceColor(price: number): string {
        if (price > currentAsk) {
            return 'dark-green';
        }else if (price === currentAsk){
            return 'green'
        } else if (price === currentBid) {
            return 'red';
        }else if (price <= currentBid){
            return 'dark-red'
        } else {
            return 'white'; 
        }
    }
</script>

<!-- Table for displaying time and sales data -->
<div class="time-and-sales">
    <table class="trade-table">
        {#if Array.isArray(allTrades)}
            <thead>
                <tr>
                    <th>Time</th>
                    <th>Price</th>
                    <th>{$settings.divideTaS ? "Size*100":"Size"}</th>
                    <th>Exchange</th>
                </tr>
            </thead>
            <tbody>
            {#each allTrades as trade}
                <tr class="{trade.color}">
                    <td>{new Date(trade.timestamp).toLocaleTimeString()}</td>
                    <td>{trade.price?.toFixed(3)}</td>
                    <td>{trade.size}</td>
                    <td>{trade.exchangeName}</td>
                </tr>
            {/each}
            {#each Array(maxLength - allTrades.length).fill(0) as _}
                <tr>
                    <td>&nbsp;</td>  <!-- Empty cell -->
                    <td>&nbsp;</td>  <!-- Empty cell -->
                    <td>&nbsp;</td>  <!-- Empty cell -->
                    <td>&nbsp;</td>  <!-- Empty cell -->
                </tr>
            {/each}
            
            </tbody>
        {/if}
    </table>
</div>

<style>
    .time-and-sales {
        font-family: Arial, sans-serif;
        font-size: 12px;
        width: 100%;
        overflow-y: auto;
        background-color: black;  /* Background color as in the image */
    }

    .trade-table {
        width: 100%;
        border-collapse: collapse; /* Remove lines between cells */
    }

    .trade-table th, .trade-table td {
        padding: 0;  /* Remove padding to match compact look */
        text-align: left;
        font-size: 12px;  /* Smaller font size */
        border: none;  /* Remove borders */
        background-color: black;  /* Background of the table */
    }

    .trade-table th {
        color: white;
        font-weight: bold;
        background-color: #333;  /* Darker header */
        padding: 5px;
        font-size: 12px;  /* Adjust font size to match screenshot */
    }

    .trade-table td {
        padding: 5px;
        font-family: monospace; /* Monospace font for numbers */
    }

</style>

