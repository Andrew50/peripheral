<script lang='ts'>
    import { settings } from '$lib/core/stores';
    import { get } from 'svelte/store';
    import type {Settings} from '$lib/core/types'
    let errorMessage: string = '';
    let tempSettings:Settings=get(settings)

    function updateLayout() {
        if (tempSettings.rows > 0 && tempSettings.columns > 0) {
            settings.set(tempSettings)
            errorMessage = '';
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
        <label for="dollar volume">Chart Columns:</label>
        <input 
            type="button" 
            id="dolvol" 
            bind:value={tempSettings.dolvol} 
            on:keypress={handleKeyPress} 
        />
    </div>
    {#if errorMessage}
        <p style="color: red;">{errorMessage}</p>
    {/if}
    <button on:click={updateLayout}>Apply</button>
</div>

