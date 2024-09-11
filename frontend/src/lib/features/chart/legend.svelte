
<script lang='ts'>
    export let hoveredCandleData;
</script>
<div class="legend">
    O: {$hoveredCandleData.open}
    H: {$hoveredCandleData.high}
    L: {$hoveredCandleData.low}
    C: {$hoveredCandleData.close}
    V: {$hoveredCandleData.volume}
</div>
<style>
    .legend {
    position: absolute;
    top: 10px;
    left: 10px;
    background-color: rgba(255, 255, 255, 0.7); /* More transparency */
    padding: 5px; /* Smaller padding */
    border-radius: 5px;
    font-size: 12px; /* Smaller font */
    font-family: Arial, sans-serif;
    color: grey; /* Grey text color */
    z-index: 1000;
}
</style>
