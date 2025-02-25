<script lang="ts">
	import { writable } from 'svelte/store';
	import { startReplay, addStream } from '$lib/utils/stream/interface';
	import '$lib/core/global.css';

	let store = writable('blank');

	function test() {
		//startReplay(1725894480000)
		//store = geeam(400,"fast")
	}
</script>

<button on:click={test}>test</button>
<div style="margin-left:10px;">
	{$store.price}
</div>
