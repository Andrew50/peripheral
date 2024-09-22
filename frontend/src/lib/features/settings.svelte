<script lang='ts'>
    import { settings } from '$lib/core/stores';
    import { get } from 'svelte/store';
    import type { Settings } from '$lib/core/types';
    import {privateRequest} from '$lib/core/backend'
    import '$lib/core/global.css'

    
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
            errorMessage = 'invalid settings';
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
        <label>Chart Rows:</label>
        <input 
            type="number" 
            bind:value={tempSettings.chartRows} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    <div>
        <label>Chart Columns:</label>
        <input 
            type="number" 
            bind:value={tempSettings.chartColumns} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    <div>
        <label>Dollar Volume:</label>
        <select id="dolvol" bind:value={tempSettings.dolvol} on:keypress={handleKeyPress}>
            <option value={true}>Yes</option>
            <option value={false}>No</option>
        </select>
    </div>
    <div>
        <label>AR Period:</label>
        <input 
            type="number" 
            bind:value={tempSettings.adrPeriod} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    <div>
        <label for="time and sales <100 filter">Display Trades Less than 100 shares</label>
        <select bind:value={tempSettings.filterTaS} on:keypress={handleKeyPress}>
            <option value={true}>Yes</option>
            <option value={false}>No</option>
        </select>
    </div>
    <div>
        <label>Divide Time and Sales by 100:</label>
        <select bind:value={tempSettings.divideTaS} on:keypress={handleKeyPress}>
            <option value={true}>Yes</option>
            <option value={false}>No</option>
        </select>
    </div>
    {#if errorMessage}
        <p style="color: red;">{errorMessage}</p>
    {/if}
    <button on:click={updateLayout}>Apply</button>
</div>

