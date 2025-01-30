<script lang="ts">
    import { onDestroy, onMount } from 'svelte';
    import type { Writable } from 'svelte/store';
    import type { TradeData, QuoteData, Instance } from '$lib/core/types';
    import { addStream } from '$lib/utils/stream/interface';
    import '$lib/core/global.css';
    import { settings } from '$lib/core/stores';
    import { get } from 'svelte/store';
    import { privateRequest } from '$lib/core/backend';
    
    export let instance: Writable<Instance>;
    let store: Writable<TradeData>;
    let quoteStore: Writable<QuoteData>;
    let releaseTrade = () => {};
    let releaseQuote = () => {};
    let unsubscribeTrade = () => {};
    let unsubscribeQuote = () => {};

    interface TaS extends TradeData {
        color: string;
        exchangeName: string;
    }

    type Exchanges = { [exchangeId: number]: string };
    let exchanges: Exchanges = {};
    let allTrades: TaS[] = [];
    let currentBid = 0;
    let currentAsk = 0;
    const maxLength = 20;
    let prevSecId = -1;
    let divideTaS = get(settings).divideTaS;
    let filterTaS = get(settings).filterTaS;

    // Fetch exchanges on mount
    onMount(() => {
        privateRequest<Exchanges>("getExchanges", {})
            .then((v: Exchanges) => {
                exchanges = v;
            });
    });

   function updateTradeStore(newTrade: TradeData) {
        if (newTrade.timestamp === undefined || newTrade.timestamp === 0) {
            return;
        }
        if (!filterTaS && newTrade.size < 100) {
            return; 
        }
        if (divideTaS) {
            newTrade.size = Math.floor(newTrade.size / 100);
        }
        const exchangeName = exchanges[newTrade.exchange];
        const newRow: TaS = {
            color: getPriceColor(newTrade.price),
            ...newTrade,
            exchangeName,
        };
        allTrades = [newRow, ...allTrades].slice(0, maxLength);
    }
    function updateQuoteStore(last: QuoteData) {
        currentBid = last.bidPrice;
        currentAsk = last.askPrice;
    }

    // Subscribe to instance changes
    let currentSecurityId: number | null = null;
    instance.subscribe((instance: Instance) => {
        if (!instance.securityId || instance.securityId === prevSecId) return;
        currentSecurityId = instance.securityId;
        // Release previous streams
        releaseTrade();
        releaseQuote();

        // Add new streams using the passed update functions
        releaseTrade = addStream<TradeData>(instance, "all", updateTradeStore);
        releaseQuote = addStream<QuoteData>(instance, "quote", updateQuoteStore);
        
        // Reset trades
        allTrades = [];

        prevSecId = instance.securityId ?? -1;
    });

    // Cleanup on destroy
    onDestroy(() => {
        unsubscribeTrade();
        releaseTrade();
        unsubscribeQuote();
        releaseQuote();
    });

    function getPriceColor(price: number): string {
        if (price > currentAsk) {
            return 'dark-green';
        } else if (price === currentAsk) {
            return 'green';
        } else if (price === currentBid) {
            return 'red';
        } else if (price < currentBid) {
            return 'dark-red';
        } else {
            return 'white';
        }
    }

    // Modify the time format to be more compact
    function formatTime(timestamp: number): string {
        const date = new Date(timestamp);
        return date.toLocaleTimeString([], { 
            hour12: false,
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });
    }
</script>

<!-- Table for displaying time and sales data -->
<div class="time-and-sales">
    <table class="trade-table">
        {#if Array.isArray(allTrades)}
            <thead>
                <tr>
                    <th>Price</th>
                    <th>{$settings.divideTaS ? "Sz*100" : "Size"}</th>
                    <th>Exch</th>
                    <th>Time</th>
                </tr>
            </thead>
            <tbody>
                {#each allTrades as trade}
                    <tr class="{trade.color}">
                        <td>{trade.price?.toFixed(2)}</td>
                        <td>{trade.size}</td>
                        <td>{trade.exchangeName.substring(0,4)}</td>
                        <td>{formatTime(trade.timestamp)}</td>
                    </tr>
                {/each}
                {#each Array(maxLength - allTrades.length).fill(0) as _}
                    <tr>
                        <td>&nbsp;</td>
                        <td>&nbsp;</td>
                        <td>&nbsp;</td>
                        <td>&nbsp;</td>
                    </tr>
                {/each}
            </tbody>
        {/if}
    </table>
</div>

<style>
    .time-and-sales {
        font-family: Arial, sans-serif;
        font-size: 10px;
        width: 100%;
        overflow-y: auto;
        background-color: black;
    }

    .trade-table {
        width: 100%;
        border-collapse: collapse;
        table-layout: fixed;
    }

    .trade-table th, .trade-table td {
        padding: 1px 2px;
        text-align: left;
        font-size: 10px;
        border: none;
        background-color: black;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .trade-table th:nth-child(1), .trade-table td:nth-child(1) { width: 25%; }
    .trade-table th:nth-child(2), .trade-table td:nth-child(2) { width: 25%; }
    .trade-table th:nth-child(3), .trade-table td:nth-child(3) { width: 25%; }
    .trade-table th:nth-child(4), .trade-table td:nth-child(4) { width: 25%; }

    .trade-table th {
        color: white;
        font-weight: bold;
        background-color: #333;
        padding: 2px;
    }
</style>

