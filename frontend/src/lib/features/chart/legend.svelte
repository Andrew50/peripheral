
<script lang='ts'>
    export let hoveredCandleData;
    import type {Instance} from '$lib/core/types'
    export let instance: Instance
</script>
<div class="legend">
    <div class="query">
    {instance?.ticker ?? "NA"}
    {instance?.timeframe ?? "NA"}
    </div>
    <div class="ohlcv" style="color: {$hoveredCandleData.chgprct < 0 ? 'red' : 'green'}">
        O: {$hoveredCandleData.open.toFixed(2)}
        H: {$hoveredCandleData.high.toFixed(2)}
        L: {$hoveredCandleData.low.toFixed(2)}
        C: {$hoveredCandleData.close.toFixed(2)}
        AR: {$hoveredCandleData.adr?.toFixed(2)}
        CHG: {$hoveredCandleData.chg.toFixed(2)}
        ({$hoveredCandleData.chgprct.toFixed(2)}%)
        V: {$hoveredCandleData.volume}
    </div>
</div>
<style>
    .legend {
    position: absolute;
    top: 10px;
    left: 10px;
    background-color: rgba(0, 0, 0, 0.5); /* Semi-transparent black background */
    padding: 5px;
    border-radius: 4px;
    font-family: Arial, sans-serif;
    color: white; /* White text */
    z-index: 900;
}

.query {
    font-size: 20px; /* Smaller font for stock name and timeframe */
    margin-bottom: 5px;
}

.ohlcv {
    display: grid;
    grid-template-columns: auto auto; /* Align labels and values */
    font-size: 16px; /* Smaller, cleaner font size */
    gap: 4px;
}

.label {
    text-align: left;
}

.value {
    text-align: right;
    width: 80px; /* Adjust this value to suit the longest expected number */
    font-family: monospace; /* This will ensure equal spacing for each character */
}

</style>
