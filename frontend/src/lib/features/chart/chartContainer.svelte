<script lang="ts">
    import Chart from './chart.svelte';
    import {settings} from '$lib/core/stores'
    export let width: number;
</script>
<div class="chart-container">
    {#each Array.from({ length: $settings.chartRows }) as _, j}
        <div class="row" style="height: calc(100% / {$settings.chartRows})">
            {#each Array.from({length: $settings.chartColumns }) as _, i}
                    <Chart width={width / $settings.chartColumns} chartId={i + j * $settings.chartColumns} />
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
