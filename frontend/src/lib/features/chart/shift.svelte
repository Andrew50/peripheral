
<script lang='ts'>
export let shiftOverlay;
</script>
{#if ($shiftOverlay).isActive}
    
    <div class="shiftOverlayStyle" style="left: {$shiftOverlay.x}px; top: {$shiftOverlay.y}px;
    width: {$shiftOverlay.width}px; height: {$shiftOverlay.height}px;">
    <div class="percentageText">
        ${Math.round(($shiftOverlay.currentPrice - $shiftOverlay.startPrice) * 100)/100}
    </div>
    <div class = "percentageText">
        {Math.round(($shiftOverlay.currentPrice / $shiftOverlay.startPrice - 1) * 10000)/100}%
    </div>
    </div>
{/if}
<style>
    .shiftOverlayStyle {
        position: absolute;
        border: 2px dashed lightgrey;
        background-color: rgba(211, 211, 211, 0.2); /* Light grey with transparency */
        z-index: 1000; /* Ensure the overlay is on top of everything */
        pointer-events: none; /* Prevent blocking clicks on other elements */
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
    }
    .percentageText {
        color: var(--highlight-color); /* Use your theme's highlight color */
        font-size: 1.5rem;
        font-weight: bold;
        text-align: center;
    }
</style>
