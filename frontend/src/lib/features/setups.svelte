<script lang="ts">
  import { onMount } from 'svelte';
  import { writable } from 'svelte/store';
  import { privateRequest , queueRequest} from '$lib/core/backend';
  import type { Writable } from 'svelte/store';

  interface Setup {
    setupId: number;
    name: string;
    timeframe: string;
    bars: number;
    threshold: number;
    dolvol: number;
    adr: number;
    mcap: number;
  }

  let setups: Writable<Setup[]> = writable([]);
  let selectedSetupId: number | null = null;
  let editedSetup: Setup | null = null;

  // Fetch setups on component mount
  onMount(() => {
    privateRequest<Setup[]>('getSetups', {})
      .then((v: Setup[]) => {
        setups.set(v);
      })
      .catch((error) => {
        console.error('Error fetching setups:', error);
      });
  });

  // Function to start editing a setup
  function editSetup(setup: Setup) {
    selectedSetupId = setup.setupId;
    editedSetup = { ...setup }; // Create a copy for editing
  }

  // Function to save the edited setup
  function saveSetup() {
    if (!editedSetup) return;

    privateRequest('updateSetup', editedSetup)
      .then(() => {
        setups.update((currentSetups) => {
          return currentSetups.map((setup) =>
            setup.setupId === editedSetup!.setupId ? editedSetup! : setup
          );
        });
        selectedSetupId = null;
        editedSetup = null;
      })
      .catch((error) => {
        console.error('Error updating setup:', error);
      });
  }

  // Function to cancel editing
  function cancelEdit() {
    selectedSetupId = null;
    editedSetup = null;
  }
  function train(setupId:number){
      queueRequest<null>('train',{setupId:setupId})
  }

</script>

<div class="table-container">
  <table>
    <thead>
      <tr>
        <th>Name</th>
        <th>Timeframe</th>
        <th>Bars</th>
        <th>Threshold</th>
        <th>DolVol</th>
        <th>ADR</th>
        <th>MCap</th>
        <th>Actions</th>
      </tr>
    </thead>
    <tbody>
      {#if Array.isArray($setups) && $setups.length > 0}
        {#each $setups as setup}
          <tr>
            {#if selectedSetupId === setup.setupId}
              <!-- Editable fields -->
              <td><input type="text" bind:value={editedSetup.name}></td>
              <td><input type="text" bind:value={editedSetup.timeframe}></td>
              <td><input type="number" bind:value={editedSetup.bars}></td>
              <td><input type="number" bind:value={editedSetup.threshold}></td>
              <td><input type="number" bind:value={editedSetup.dolvol}></td>
              <td><input type="number" bind:value={editedSetup.adr}></td>
              <td><input type="number" bind:value={editedSetup.mcap}></td>
              <td>
                <button class="action-btn save" on:click={saveSetup}>Save</button>
                <button class="action-btn cancel" on:click={cancelEdit}>Cancel</button>
              </td>
            {:else}

              <!-- Display fields -->
              <td><button on:click={() => train(setup.setupId)}>Train</button></td>
              <td>{setup.name}</td>
              <td>{setup.timeframe}</td>
              <td>{setup.bars}</td>
              <td>{setup.threshold}</td>
              <td>{setup.dolvol}</td>
              <td>{setup.adr}</td>
              <td>{setup.mcap}</td>
              <td>
                <button class="action-btn edit" on:click={() => editSetup(setup)}>Edit</button>
              </td>
            {/if}
          </tr>
        {/each}
      {:else}
        <tr>
          <td colspan="8">No setups available.</td>
        </tr>
      {/if}
    </tbody>
  </table>
</div>

<style>
  @import "$lib/core/colors.css";

  /* Table styling */
  .table-container {
    border: 1px solid var(--c4);
    border-radius: 4px;
    overflow: hidden;
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

  /* Button styling */
  .action-btn

    /* Button styling */
    .controls {
        display: flex;
        justify-content: space-between;
        margin-bottom: 20px;
    }

    .action-btn {
        background-color: var(--c3);
        color: var(--f1);
        border: none;
        padding: 10px 15px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1rem;
    }

    .action-btn:hover {
        background-color: var(--c3-hover);
    }

    /* Table styling */
    .table-container {
        border: 1px solid var(--c4);
        border-radius: 4px;
        overflow: hidden;
        margin-top: 10px;
    }

    table {
        width: 100%;
        border-collapse: collapse;
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

    .table-row:hover {
        background-color: var(--c1);
        cursor: pointer;
    }

    /* Highlight selected study */
    tr.selected {
        background-color: var(--c6);
    }

    .highlight {
        color: var(--c3);
        font-weight: bold;
    }
</style>
