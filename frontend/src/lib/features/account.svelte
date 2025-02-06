<script lang="ts">
    import { privateFileRequest } from '$lib/core/backend';
    import { queueRequest } from '$lib/core/backend';
    import type { Instance } from '$lib/core/types';
    import List from '$lib/utils/modules/list.svelte';
    import { writable } from 'svelte/store';

    let files: FileList;
    let uploading = false;
    let message = '';
    let trades = writable<Trade[]>([]);

    interface Trade extends Instance {
        direction: string;
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
            const result = await queueRequest<Trade[]>('grab_user_trades', {});
            trades.set(result);
            console.log(result);
            message = 'Trades loaded successfully';
        } catch (error) {
            message = `Error: ${error}`;
            console.error('Load trades error:', error);
        }
    }
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
        <button on:click={pullTrades}>Pull Trades</button>
    </div>
    {#if message}
        <p class="message">{message}</p>
    {/if}
</div>

<List 
    on:contextmenu={(event) => {event.preventDefault();}} 
    list={trades} 
    columns={["date", "ticker", "direction", "status", "openQuantity", "closedPnL"]}
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
</style>