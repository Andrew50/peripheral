<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { getColumnStore, register } from '$lib/utils/stream/streamHub';
    import type { Writable } from 'svelte/store';
    
    export let instance: any;      // { securityId, totalShares, … }
    export let type: string;       // 'price' | 'changePct' | 'change' | 'marketCap' | 'chgExt'
  
    let unregister = () => {};
    let store: Writable<any> | undefined;
    let lastPrice: number | undefined = undefined;
    let pulseClass = '';
    let unchanged = '';
    let changed = '';
    let lastPriceStr: string | undefined = undefined;

    // Map types to store column types
    const typeMapping: Record<string, string> = {
        'price': 'price',
        'change': 'change', 
        'change %': 'changePct',
        'change % extended': 'chgExt',
    };
    
    const columnType = typeMapping[type] || 'price';
    $: if (instance?.securityId) {
        // Clean up previous registration
        unregister();
        
        // Register for streaming data
        unregister = register(Number(instance.securityId));
        
        // Get the store for this security/column combination
        store = getColumnStore(Number(instance.securityId), columnType as any);
    }
    // Determine styling based on values
    $: isPositive = (type === 'change' && data?.change > 0) || 
                ((type === 'change %' || type === 'change % extended') && data?.formatted && !data.formatted.includes('-'));
    $: isNegative = (type === 'change' && data?.change < 0) || 
                   ((type === 'change %' || type === 'change % extended') && data?.formatted && data.formatted.includes('-'));
    $: isNeutral = !isPositive && !isNegative;
    onDestroy(() => {
        unregister();
    });
    
    $: data = store ? $store : {};
    
    // Handle price formatting and pulse effects
    $: if (type === 'price' && data?.price !== undefined) {
        handlePriceUpdate(data.price);
    }
    
    function handlePriceUpdate(newPrice: number) {
        if (lastPrice !== undefined && newPrice !== lastPrice) {
            const dir = newPrice > lastPrice ? 1 : newPrice < lastPrice ? -1 : 0;
            if (dir) firePulse(dir);
        }
        updateSlices(newPrice);
        lastPrice = newPrice;
    }
    
    function updateSlices(newPrice: number) {
        const next = newPrice.toFixed(2);
        const prev = lastPriceStr ?? '';

        // find the first differing character
        let idx = 0;
        while (idx < next.length && next[idx] === prev[idx]) idx++;

        unchanged = next.slice(0, idx);
        changed = next.slice(idx);
        lastPriceStr = next;
    }

    function firePulse(dir: number) {
        pulseClass = '';
        requestAnimationFrame(() => {
            pulseClass = dir === 1 ? 'flash-up' : 'flash-down';
        });
    }
    
    function formatMarketCap(marketCap: number | undefined): string {
        if (!marketCap) return 'N/A';
        if (marketCap >= 1e12) {
            return `$${(marketCap / 1e12).toFixed(2)}T`;
        } else if (marketCap >= 1e9) {
            return `$${(marketCap / 1e9).toFixed(2)}B`;
        } else if (marketCap >= 1e6) {
            return `$${(marketCap / 1e6).toFixed(2)}M`;
        } else {
            return `$${marketCap.toFixed(2)}`;
        }
    }
    
</script>

<div 
    class="price-cell {pulseClass}"
    class:positive={isPositive}
    class:negative={isNegative}
    class:neutral={isNeutral}
>
    {#if type === 'price'}
        {#if changed === ''}
            {unchanged}
        {:else}
            {unchanged}<span
                class="diff"
                class:up={pulseClass === 'flash-up'}
                class:down={pulseClass === 'flash-down'}>
                {changed}
            </span>
        {/if}
    {:else if type === 'change'}
        {data?.formatted || ''}
    {:else if type === 'change %' || type === 'change % extended'}
        {data?.formatted || ''}
    {:else if type === 'market cap'}
        {formatMarketCap(data?.marketCap)}
    {:else}
        {data?.formatted || data?.price || ''}
    {/if}
</div>



<style>
    @keyframes flashGreen {
        0% { color: var(--positive, rgb(72, 225, 72)); }
        100% { color: currentColor; }
    }
    @keyframes flashRed {
        0% { color: var(--negative, rgb(225, 72, 72)); }
        100% { color: currentColor; }
    }
    
    .flash-up { animation: flashGreen .5s ease-out forwards; }
    .flash-down { animation: flashRed .5s ease-out forwards; }

    .diff.up { color: var(--positive, rgb(72, 225, 72)); }
    .diff.down { color: var(--negative, rgb(225, 72, 72)); }

    .positive { color: var(--positive, rgb(72, 225, 72)); }
    .negative { color: var(--negative, rgb(225, 72, 72)); }
    .neutral { color: white; }

    .price-cell { 
        padding: 0 4px; 
        transition: color 0.2s ease;
    }
</style>
  