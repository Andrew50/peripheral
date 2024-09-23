<!-- screen.svelte -->
<script lang="ts">
  import { writable, get } from 'svelte/store';
  import List from '$lib/utils/list.svelte'
  import '$lib/core/global.css'
  import {queueRequest } from '$lib/core/backend';
  import type { Writable } from 'svelte/store';
  import type {Instance} from '$lib/core/types'
  import {setups} from '$lib/core/stores'
  let screens:Writable<Screen[]> = writable([])
  interface Screen extends Instance {
      setupType: string
      score: number
      flagged: boolean
  }

  function runScreen() {
      const setupIds = get(setups).filter(v => v.activeScreen).map(v => v.setupId)
      queueRequest<Screen[]>('screen', { setupIds: setupIds}).then((response) => {
          console.log(response)
          screens.set(response)
      });
  }
</script>

<div class="controls-container">
  {#if Array.isArray($setups) && $setups.length > 0}
   {#each $setups as setup (setup.setupId)}
    <button 
      class="{setup.activeScreen ? '' : 'inactive'}" 
      on:click={() => {setup.activeScreen = !setup.activeScreen}}
    >
      {setup.name}
    </button>
  {/each}
  {/if}
</div>

<button on:click={runScreen}> Screen </button>

<List on:contextmenu={(event)=>{event.preventDefault()}} list={screens} columns={["ticker","change","setup","score"]}/>

