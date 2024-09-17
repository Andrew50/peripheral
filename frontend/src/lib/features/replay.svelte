<script lang='ts'>
import {startReplay, stopReplay} from '$lib/utils/stream'
import {replayStream} from '$lib/utils/stream'


function toggleReplay() {
    if(replayStream.replayStatus) {
        stopReplay()
    } else {
        return //startReplay() 
    }
}


</script>

<div class='replay-controls'> 
    <button on:click={toggleReplay}>
    {#if !replayStream.replayStatus}
        Start Replay 
    {/if}
    {#if replayStream.replayStatus}
        Stop Replay
    {/if}
    </button>
    <button> Pause Replay</button>

</div> 

<style>
  @import "$lib/core/colors.css";
  .buttons-container {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 20px;
}
    .button {
        padding: 10px 20px;
        background-color: var(--c2);
        border: 1px solid var(--c4);
        border-radius: 8px;
        cursor: pointer;
        transition: background-color 0.3s ease;
        font-size: 16px;
        display: inline-block;
    }

    .button:hover {
        background-color: var(--c3-hover);
    }
</style>
