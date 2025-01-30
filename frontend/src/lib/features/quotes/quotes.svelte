<script lang='ts'>
    import L1 from './l1.svelte';
    import TimeAndSales from './timeAndSales.svelte';
    import { get, writable } from 'svelte/store';
    import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
    import type { Instance } from '$lib/core/types';
    import type {Writable} from 'svelte/store'
    import TickerInfo from '$lib/features/tickerInfo.svelte';

    let instance :Writable<Instance> = writable({})
    let container: HTMLDivElement;

    function handleKey(event: KeyboardEvent) {
        if (event.key == "Tab" || /^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
            const v = get(instance)
            queryInstanceInput(["ticker"], v)
                .then((i: Instance) => {
                    instance.set(i)
                })
                .catch();
        }
    }

    $: if (container) {
        container.addEventListener("keydown", handleKey);
    }
</script>

<div bind:this={container} tabindex="-1" class="container">
    <div class="ticker-display">
        <span>{$instance.ticker || "--"}</span>  <!-- Display '--' when there's no ticker -->
    </div>
    <div class="content-wrapper">
    <L1 instance={instance} />
    <TimeAndSales instance={instance} />
    </div>
    <TickerInfo />
</div>

<style>

    .ticker-display {
        font-family: Arial, sans-serif;
        font-size: 24px;       /* Larger font size for ticker display */
        color: white;          /* White color for the ticker */
        background-color: black; /* Black background for contrast */
        border: 1px solid #333; /* Border around the ticker */
        border-radius:5px;
        width: 100%;          /* Fixed width for consistent display */
        text-align: center;    /* Center-align the ticker text */
        height: 40px;          /* Fixed height for consistent layout */
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .ticker-display span {
        display: inline-block;
    }
    .content-wrapper {
        width: 100%;              /* Take up full width of parent container */
        display: flex;
        flex-direction: column;
        align-items: flex-start;   /* Ensure everything is aligned to the left */
    }
</style>

