<script lang='ts'>
    import { chartLayout } from '$lib/core/stores';
    import { get } from 'svelte/store';
    let rows: number = get(chartLayout).rows;
    let columns: number = get(chartLayout).columns;
    let errorMessage: string = '';

    function updateLayout() {
        if (rows > 0 && columns > 0) {
            chartLayout.set({ rows, columns });
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
        <label for="rows">Chart Rows:</label>
        <input 
            type="number" 
            id="rows" 
            bind:value={rows} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    <div>
        <label for="columns">Chart Columns:</label>
        <input 
            type="number" 
            id="columns" 
            bind:value={columns} 
            min="1" 
            on:keypress={handleKeyPress} 
        />
    </div>
    {#if errorMessage}
        <p style="color: red;">{errorMessage}</p>
    {/if}
    <button on:click={updateLayout}>Apply</button>
</div>

