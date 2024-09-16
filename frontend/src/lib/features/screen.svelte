<!-- screen.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { writable, get } from 'svelte/store';
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
  $: markedScreens = $screens.filter(setup => setup.flagged);
  $: unmarkedScreens = $screens.filter(setup => !setup.flagged);
  function getSetupName(setupId:number){
      return get(setups).find(v=> v.setupId == setupId).name
  }

    function rowRightClick(event:MouseEvent,screen:Screen){
        event.preventDefault();
        queryInstanceRightClick(event,screen,'list').then((v:RightClickResult)=>{
            if (v === "flag"){
                screen.flagged = ! screen.flagged
                screens.update(s=>s)
            }

        })
    }
function toggleSetup(setup) {
      setup.activeScreen = !setup.activeScreen;
      setups.update(s => s); // Trigger update to rerender
      console.log(setup)
  }


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


{#each [markedScreens,unmarkedScreens] as s}
<div class="table-container">
  {#if Array.isArray(s) && s.length > 0}
  <table>
    <thead>
      <tr>
        <th>Ticker</th>
        <th>Setup</th>
        <th>Score</th>
      </tr>
    </thead>
    <tbody>
        {#each s as screen}
          <tr on:click={()=>changeChart(screen)}

            on:contextmenu={(event)=>rowRightClick(event,screen)}
          >
              <td>{screen.ticker}</td>
              <td>{getSetupName(screen.setupId)}</td>
              <td>{screen.score}</td>
          </tr>
        {/each}
    </tbody>
  </table>
  {/if}
</div>
{/each}


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

