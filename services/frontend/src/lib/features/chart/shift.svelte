<script lang="ts">
	export let shiftOverlay;
</script>

{#if $shiftOverlay.isActive}
	<div
		class="shift-overlay"
		style="left: {$shiftOverlay.x}px; top: {$shiftOverlay.y}px;
    width: {$shiftOverlay.width}px; height: {$shiftOverlay.height}px;"
	>
		<div class="percentage-text">
			${Math.round(($shiftOverlay.currentPrice - $shiftOverlay.startPrice) * 100) / 100}
		</div>
		<div class="percentage-text">
			{Math.round(($shiftOverlay.currentPrice / $shiftOverlay.startPrice - 1) * 10000) / 100}%
		</div>
	</div>
{/if}

<style>
	.shift-overlay {
		position: absolute;
		border: 2px dashed rgb(255 255 255 / 60%);
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

	.percentage-text {
		color: #fff;
		font-size: 1.2rem;
		font-weight: 600;
		text-align: center;
		text-shadow: 0 1px 2px rgb(0 0 0 / 80%);
		line-height: 1.2;
		white-space: nowrap;
		background: rgb(255 255 255 / 10%);
		padding: 4px 8px;
		border-radius: 4px;
		border: 1px solid rgb(255 255 255 / 20%);
		backdrop-filter: blur(4px);
	}
</style>
