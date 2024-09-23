
<script lang='ts'>
    export let hoveredCandleData;
    import type {Instance} from '$lib/core/types'
    import {changeChart} from './interface'
    export let instance: Instance
    import {queryInstanceInput} from '$lib/utils/input.svelte'
    function handleClick(event:MouseEvent |TouchEvent){
        queryInstanceInput("any",instance)
        .then((v:Instance)=>{
            changeChart(instance)
        })

    }
</script>
<div tabindex="-1" on:click={handleClick} on:touchstart={handleClick} class="legend">
    <div class="query">
    {instance?.ticker ?? "NA"}
    {instance?.timeframe ?? "NA"}
    </div>
    <div  class="ohlcv" style="color: {$hoveredCandleData.chgprct < 0 ? 'red' : 'green'}">
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


</style>
