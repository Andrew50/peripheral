<script lang="ts">
	export let shiftOverlay;
</script>

{#if $shiftOverlay.isActive}
	<div
		class="shiftOverlayStyle"
		style="left: {$shiftOverlay.x}px; top: {$shiftOverlay.y}px;
    width: {$shiftOverlay.width}px; height: {$shiftOverlay.height}px;"
	>
		<div class="percentageText">
			${Math.round(($shiftOverlay.currentPrice - $shiftOverlay.startPrice) * 100) / 100}
		</div>
		<div class="percentageText">
			{Math.round(($shiftOverlay.currentPrice / $shiftOverlay.startPrice - 1) * 10000) / 100}%
		</div>
	</div>
{/if}

<style>
	.shiftOverlayStyle {
		position: absolute;
		border: 2px dashed rgba(255, 255, 255, 0.6);
		background: transparent;
		border-radius: 8px;
		z-index: 1000; /* Ensure the overlay is on top of everything */
		pointer-events: none; /* Prevent blocking clicks on other elements */
		display: flex;
		flex-direction: column;
		justify-content: center;
		align-items: center;
		gap: 4px;
		padding: 8px;
	}
	.percentageText {
		color: #ffffff;
		font-size: 1.2rem;
		font-weight: 600;
		text-align: center;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		line-height: 1.2;
		white-space: nowrap;
		background: rgba(255, 255, 255, 0.1);
		padding: 4px 8px;
		border-radius: 4px;
		border: 1px solid rgba(255, 255, 255, 0.2);
		backdrop-filter: blur(4px);
	}
</style>
