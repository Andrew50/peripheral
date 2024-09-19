<script lang="ts">
    import Chart from './chart.svelte';
    import {chartLayout} from '$lib/core/stores'
    export let width: number;
</script>
<div class="chart-container">
    {#each Array.from({ length: $chartLayout.rows }) as _, j}
        <div class="row" style="height: calc(100% / {$chartLayout.rows})">
            {#each Array.from({length: $chartLayout.columns }) as _, i}
                    <Chart width={width / $chartLayout.columns} chartId={i + j * $chartLayout.columns} />
            {/each}
        </div>
    {/each}
</div>

<style>
    .chart-container {
        display: flex;
        flex-direction: column; /* Stack the rows vertically */
        width: 100%;
        flex-basis:0;
    }

    .row {
        display: flex;
        width: 100%; /* Ensure the row takes up full width */
        justify-content: space-between; /* Ensure charts are evenly distributed */
        flex-grow: 1;
    }

</style>
