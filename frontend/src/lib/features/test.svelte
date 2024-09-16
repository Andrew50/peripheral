<script lang='ts'>

import {writable} from 'svelte/store'
import {startReplay,getStream} from "$lib/utils/stream"
let store = writable("blank")

function test(){
    //startReplay(1725894480000)
    store = getStream("NVDA","fast")
}

</script>

<button on:click={test}>test</button>
<div style="margin-left:10px;">
{$store.price}
</div>
