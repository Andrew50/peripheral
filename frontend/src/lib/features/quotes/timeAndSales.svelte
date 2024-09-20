<script lang="ts">
    import { onDestroy } from 'svelte';
    import type { Writable } from 'svelte/store';
    import type { TradeData, QuoteData } from '$lib/core/types';
    import { getStream } from '$lib/utils/stream';

    export let ticker: Writable<string>;
    let store: Writable<TradeData>;
    let quoteStore: Writable<QuoteData>;
    let releaseTrade: Function = () => {};
    let releaseQuote: Function = () => {};
    let unsubscribeTrade: Function = () => {};
    let unsubscribeQuote: Function = () => {};
    interface TaS extends TradeData {
        color: string
    }
    let allTrades: TaS[] = [];
    let currentBid = 0;
    let currentAsk = 0;
    const maxLength = 20;
    let prevTick = "";

    ticker.subscribe((tick: string) => {
        if (tick !== prevTick){
        unsubscribeTrade();
        releaseTrade();
        unsubscribeQuote();
        releaseQuote();
        const [s, r] = getStream<TradeData>(tick, "fast");
        store = s;
        releaseTrade = r;
        const [qs, qr] = getStream<QuoteData>(tick,"quote");
        quoteStore = qs;
        releaseQuote = qr;
        allTrades = []
        unsubscribeTrade = store.subscribe((newTrade: TradeData) => {
            if (newTrade.timestamp !== undefined) {
                const newRow: TaS = {color:getPriceColor(newTrade.price),...newTrade}
                allTrades = [...allTrades, newRow].slice(-maxLength);
            }
        });
        unsubscribeQuote = quoteStore.subscribe((quote) => {
            currentBid = quote.bidPrice;
            currentAsk = quote.askPrice;
        });
        prevTick = tick
        }
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
                    <th>Size</th>
                </tr>
            </thead>
            <tbody>
            {#each Array(maxLength - allTrades.length).fill(0) as _}
                <tr>
                    <td>&nbsp;</td>  <!-- Empty cell -->
                    <td>&nbsp;</td>  <!-- Empty cell -->
                    <td>&nbsp;</td>  <!-- Empty cell -->
                </tr>
            {/each}
            {#each allTrades as trade}
                <tr class="{trade.color}">
                    <td>{new Date(trade.timestamp).toLocaleTimeString()}</td>
                    <td>{trade.price?.toFixed(3)}</td>
                    <td>{trade.size}</td>
                </tr>
            {/each}
            
            </tbody>
        {/if}
    </table>
</div>

<style>
    @import "$lib/core/colors.css";
    .dark-green td{
        color: #006400;  /* Dark green */
        font-weight: bold;
    }

    .green td{
        color: #00ff00;  /* Bright green */
    }

    .red td{
        color: #ff0000;  /* Bright red */
    }

    .dark-red td{
        color: #8b0000;  /* Dark red */
        font-weight: bold;
    }

    .white td{
        color: white;
    }

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

