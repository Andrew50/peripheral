<script lang='ts'>
import {startReplay, stopReplay, pauseReplay,changeSpeed, resumeReplay, nextDay} from '$lib/utils/stream/interface';
import {queryInstanceInput} from '$lib/utils/popups/input.svelte'
import {UTCTimestampToESTString} from '$lib/core/timestamp'
import {streamInfo} from '$lib/core/stores'
import type{StreamInfo} from '$lib/core/stores'
import '$lib/core/global.css'

import type {Instance} from '$lib/core/types'
    function strtReplay(){
        queryInstanceInput(["timestamp"],["timestamp"],{timestamp:0,extendedHours:false})
        .then((v:Instance)=>{
/*            streamInfo.update((r:StreamInfo) => {
                r.startTimestamp = v.timestamp
                return r
            })*/
            startReplay(v)
        })
    }
    function changeReplaySpeed(event: Event) {
        const input = event.target as HTMLInputElement;
        const newSpeed = parseFloat(input.value); // Parse the speed as a decimal number
        if (!isNaN(newSpeed) && newSpeed > 0) {
            changeSpeed(newSpeed);
        }
    }

</script>

<div class='replay-controls' tabindex="-1"> 
    {#if $streamInfo.replayActive}
        <button on:click={stopReplay}>Stop</button>
        <button on:click={()=>{stopReplay;startReplay({timestamp:$streamInfo.startTimestamp,extendedHours:$streamInfo.extendedHours});}}>Reset
       <!-- to {UTCTimestampToESTString($replayInfo.startTimestamp)}-->
        </button>
    
        {#if $streamInfo.replayPaused}
            <button on:click={resumeReplay}>Play </button>
        {:else}
            <button on:click={pauseReplay}>Pause</button>
        <div>
        <label for="speed-input">Speed:</label>
        <input id="speed-input" type="number" step="0.1" min="0.1" value="1.0" on:input={changeReplaySpeed} />
        </div>
        <div>
        <button on:click={nextDay} >Jump to next market open (9:30 AM EST)</button>
        <!--<button on:click={jumpToNextDay} >Jump to next day (4 AM EST)</button>    -->
        </div>
        {/if}
    {:else}
        <button on:click={strtReplay}>Start</button>
    {/if}
</div> 
