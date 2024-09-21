<script lang='ts'>
    import { settings } from '$lib/core/stores';
    import { get } from 'svelte/store';
    import type { Settings } from '$lib/core/types';
    import {privateRequest} from '$lib/core/backend'
    
    let errorMessage: string = '';
    let tempSettings: Settings = { ...get(settings) }; // Create a local copy to work with

    function updateLayout() {
        if (tempSettings.chartRows > 0 && tempSettings.chartColumns > 0) {
            privateRequest<void>("setSettings",{settings:tempSettings})
            .then(()=>{
                settings.set(tempSettings); // Update the store with new settings
                errorMessage = '';
            })
        } else {
            errorMessage = 'Please enter valid numbers greater than 0 for both rows and columns.';
        }
    }

    function handleKeyPress(event: KeyboardEvent) {
        if (event.key === 'Enter') {
            updateLayout();
        }
    }
</script>

<div>
    <div>
        <label for="chart rows">Chart Rows:</label>
        <input 
            type="number" 
            id="rows" 
            bind:value={tempSettings.chartRows} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    <div>
        <label for="chart columns">Chart Columns:</label>
        <input 
            type="number" 
            id="columns" 
            bind:value={tempSettings.chartColumns} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    <div>
        <label for="dollar volume">Dollar Volume:</label>
        <select id="dolvol" bind:value={tempSettings.dolvol} on:keypress={handleKeyPress}>
            <option value={true}>Yes</option>
            <option value={false}>No</option>
        </select>
    </div>
    {#if errorMessage}
        <p style="color: red;">{errorMessage}</p>
    {/if}
    <button on:click={updateLayout}>Apply</button>
</div>

<style>
  /* Add some basic styles to make the form look better */
  div {
      margin-bottom: 10px;
  }
  label {
      margin-right: 10px;
  }
  input, select, button {
      padding: 5px;
      font-size: 14px;
  }
</style>

