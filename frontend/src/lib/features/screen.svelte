<!-- screen.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { writable, get } from 'svelte/store';
  import List from '$lib/utils/list.svelte'
  import '$lib/core/global.css'

  import { privateRequest, queueRequest } from '$lib/core/backend';
  import {queryInstanceRightClick} from '$lib/utils/rightClick.svelte'
  import type {RightClickResult} from "$lib/utils/rightClick.svelte"
  import type { Writable } from 'svelte/store';
  import type {Instance} from '$lib/core/types'
  import {setups} from '$lib/core/stores'
  import {changeChart} from '$lib/features/chart/interface'

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
/*  function getSetupName(setupId:number){
      return get(setups).find(v=> v.setupId == setupId).name
  }*/



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

<List list={screens} columns={["ticker","change","setup","score"]}/>
