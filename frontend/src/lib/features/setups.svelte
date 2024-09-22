<script lang="ts">
  import { privateRequest , queueRequest} from '$lib/core/backend';
  import type {Setup} from '$lib/core/types'
  import {setups} from '$lib/core/stores'
      import '$lib/core/global.css'


  let selectedSetupId: number | null = null;
  let editedSetup: Setup | null = null;

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
                <button on:click={saveSetup}>Save</button>
                <button on:click={cancelEdit}>Cancel</button>
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
                <button on:click={() => editSetup(setup)}>Edit</button>
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
