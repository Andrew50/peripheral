<!-- screen.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { writable, get } from 'svelte/store';
  import List from '$lib/utils/list.svelte'
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

<div class="setup-buttons-container">
  {#if Array.isArray($setups) && $setups.length > 0}
   {#each $setups as setup (setup.setupId)}
    <button 
      class="setup-button {setup.activeScreen ? 'active' : ''}" 
      on:click={() => {setup.activeScreen = !setup.activeScreen}}
    >
      {setup.name}
    </button>
  {/each}
  {/if}
</div>

<button on:click={runScreen}> Screen </button>

<List list={screens} columns={["ticker","setup","score"]}/>




<style>
  @import "$lib/core/colors.css";
  @import "$lib/core/features.css";
  .setup-buttons-container {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 20px;
}

.setup-button {
    padding: 10px 20px;
    background-color: var(--c2);
    border: 1px solid var(--c4);
    border-radius: 8px;
    cursor: pointer;
    transition: background-color 0.3s ease;
    font-size: 16px;
    display: inline-block;
}

.setup-button:hover {
    background-color: var(--c3-hover);
}

.setup-button.active {
    background-color: var(--c3); /* Change to blue or another color when active */
    color: var(--f1);
}




  .table-container {
    border: 1px solid var(--c4);
    border-radius: 4px;
    margin-top: 10px;
    width: 100%;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-family: Arial, sans-serif;
  }

  th, td {
    padding: 10px;
    text-align: left;
  }

  th {
    background-color: var(--c1);
    color: var(--f1);
  }

  tr {
    border-bottom: 1px solid var(--c4);
  }

  tr:hover {
    background-color: var(--c2);
    cursor: pointer;
  }

</style>

