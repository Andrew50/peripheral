
<script lang='ts'>
    export let hoveredCandleData;
    import {chartQuery} from './interface'
    import type {Instance} from '$lib/core/types'
    export let instance: Instance
</script>
<div class="legend">
    <div class="query">
    {instance?.ticker ?? "NA"}
    {instance?.timeframe ?? "NA"}
    </div>
    <div class="ohlcv">
        O: {$hoveredCandleData.open}
        H: {$hoveredCandleData.high}
        L: {$hoveredCandleData.low}
        C: {$hoveredCandleData.close}
        V: {$hoveredCandleData.volume}
    </div>
</div>
<style>
    .legend {
    position: absolute;
    top: 10px;
    left: 10px;
    /*background-color: rgba(255, 255, 255, 0.7); /* More transparency */
    padding: 5px; /* Smaller padding */
    border-radius: 2px;
    font-family: Arial, sans-serif;
    color: white; /* Grey text color */
    z-index: 900;
}
    .query {
        font-size: 30px;
    }
    .ohlcv {
        font-size: 20px;
    }
</style>
