<script lang='ts'>
import {startReplay, stopReplay} from '$lib/utils/stream';
import {replayStream} from '$lib/utils/stream';
import {queryInstanceInput} from '$lib/utils/input.svelte'
import {UTCTimestampToESTString} from '$lib/core/timestamp'
import {replayInfo} from '$lib/core/stores'
import type{ReplayInfo} from '$lib/core/stores'
import type {Instance} from '$lib/core/types'
    function strtReplay(){
        queryInstanceInput(["timestamp"],{timestamp:0})
        .then((v:Instance)=>{
            replayInfo.update((r:ReplayInfo) => {
                r.startTimestamp = v.timestamp
                return r
            })
            startReplay(v.timestamp)
        })
    }
    function changeReplaySpeed(event: Event) {
        const input = event.target as HTMLInputElement;
        const newSpeed = parseFloat(input.value); // Parse the speed as a decimal number
        if (!isNaN(newSpeed) && newSpeed > 0) {
            replayStream.changeSpeed(newSpeed);
        }
    }

</script>

<div class='replay-controls' tabindex="-1"> 
    {#if ["active","paused"].includes($replayInfo.status)}
        <button on:click={stopReplay}>Stop</button>
        <button on:click={()=>{stopReplay;startReplay($replayInfo.startTimestamp);}}>Reset to {UTCTimestampToESTString($replayInfo.startTimestamp)}</button>
        {#if $replayInfo.status === "paused"}
            <button on:click={replayStream.resume}>Play </button>
        {:else}
            <button on:click={replayStream.pause}> Pause</button>
        <div>
        <label for="speed-input">Speed:</label>
        <input id="speed-input" type="number" step="0.1" min="0.1" value="1.0" on:input={changeReplaySpeed} />
        </div>
        {/if}
    {:else}
        <button on:click={strtReplay}>Start</button>
    {/if}
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
    input[type="number"] {
    margin-left: 4px;
    font-size: 14px;
  }
</style>
