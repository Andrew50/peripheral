<script lang="ts">
    import { privateFileRequest } from '$lib/core/backend';
    import { queueRequest } from '$lib/core/backend';
    import type { Instance } from '$lib/core/types';
    import List from '$lib/utils/modules/list.svelte';
    import { writable } from 'svelte/store';
    import { UTCTimestampToESTString } from '$lib/core/timestamp';

    let files: FileList;
    let uploading = false;
    let message = '';
    let trades = writable<Trade[]>([]);

    // Add filter states
    let sortDirection = "desc";
    let selectedDate = "";
    let selectedHour: number | "" = "";

    interface Trade extends Instance {
        trade_direction: string;
        status: string;
        openQuantity: number;
        closedPnL: number | null;
    }

    async function handleFileUpload() {
        if (!files || !files[0]) {
            message = 'Please select a file first';
            return;
        }

        uploading = true;
        message = 'Uploading...';

        try {
            const result = await privateFileRequest<{trades: Trade[]}>(
                'handle_trade_upload', 
                files[0]
            );
            trades.set(result.trades);
            message = 'Upload successful!';
        } catch (error) {
            message = `Error: ${error}`;
            console.error('Upload error:', error);
        } finally {
            uploading = false;
        }
    }

    async function pullTrades() {
        try {
            console.log("pulling trades");
            const params: any = { sort: sortDirection };
            
            if (selectedDate) {
                params.date = selectedDate;
            }
            
            if (selectedHour !== "") {
                params.hour = selectedHour;
            }

            const result = await queueRequest<Trade[]>('grab_user_trades', params);
            trades.set(result);
            console.log(result);
            message = 'Trades loaded successfully';
        } catch (error) {
            message = `Error: ${error}`;
            console.error('Load trades error:', error);
        }
    }

    // Generate hours array for the select dropdown
    const hours = Array.from({ length: 24 }, (_, i) => ({
        value: i,
        label: `${i.toString().padStart(2, '0')}:00`
    }));
</script>

<div class="account-container">
    <h2>Trade History Upload</h2>
    <div class="upload-section">
        <input 
            type="file" 
            accept=".csv"
            bind:files
            disabled={uploading}
        />
        <button 
            on:click={handleFileUpload}
            disabled={uploading || !files}
        >
            Upload
        </button>
    </div>

    <div class="filters-section">
        <select bind:value={sortDirection} on:change={pullTrades}>
            <option value="desc">Newest First</option>
            <option value="asc">Oldest First</option>
        </select>

        <input 
            type="date" 
            bind:value={selectedDate}
            on:change={pullTrades}
        />

        <select bind:value={selectedHour} on:change={pullTrades}>
            <option value="">All Hours</option>
            {#each hours as hour}
                <option value={hour.value}>{hour.label}</option>
            {/each}
        </select>

        <button on:click={pullTrades}>Refresh Trades</button>
    </div>

    {#if message}
        <p class="message">{message}</p>
    {/if}
</div>

<List 
    on:contextmenu={(event) => {event.preventDefault();}} 
    list={trades} 
    columns={["timestamp", "ticker", "trade_direction", "status", "openQuantity", "closedPnL"]}
    formatters={{
        timestamp: (value) => value ? UTCTimestampToESTString(value) : 'N/A',
        closedPnL: (value) => value !== null ? value.toFixed(2) : 'N/A'
    }}
/>

<style>
    .account-container {
        padding: 20px;
        color: white;
    }

    .upload-section {
        display: flex;
        gap: 10px;
        align-items: center;
        margin-bottom: 20px;
    }

    .filters-section {
        display: flex;
        gap: 10px;
        align-items: center;
        margin-bottom: 20px;
    }

    button {
        padding: 8px 16px;
        background-color: #333;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .message {
        margin-top: 10px;
        color: #ddd;
    }

    select, input[type="date"] {
        padding: 8px;
        background-color: #333;
        color: white;
        border: 1px solid #444;
        border-radius: 4px;
    }

    select option {
        background-color: #333;
        color: white;
    }
</style>