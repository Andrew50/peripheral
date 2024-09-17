<script lang="ts" context="module">
    import type {Instance} from '$lib/core/types'
    export function newJournal(instance:Instance):void{
        console.log("new journal to implement")
    }
    import {UTCTimestampToESTString} from '$lib/core/timestamp'
    import type {Writable} from 'svelte/store'
    import {writable} from 'svelte/store'
    let selectedJournalId: number | null = null;
    interface Journal {
        journalId: number,
        timestamp: number,
        completed: boolean
    }
    let journals: Writable<Journal[]> = writable([])
    function selectJournal(journal: Journal) : void {
        if (journal.journalId === selectedJournalId){
            selectedJournalId = 0
        }else{
            selectedJournalId = journal.journalId
        }
    }
</script>

<div class="table-container">
    <table>
        <thead>
            <tr>
                <th>Date</th>
            </tr>
        </thead>
        <tbody>
            {#if Array.isArray($journals) && $journals.length > 0}
                {#each $journals as journal}
                    <tr class="table-row"  on:click={()=>selectJournal(journal)}>
                        <td>{UTCTimestampToESTString(journal.timestamp)}</td>
                    </tr>

                    {#if selectedJournalId == journal.journalId}
                        <tr>
                            <td colspan="2">
                                <Entry completed={journal.completed} func="Journal" id={journal.journalId} />
                            </td>
                        </tr>
                    {/if}
                {/each}
            {/if}
        </tbody>
    </table>
</div>
<style>
    @import "$lib/core/colors.css";

    /* Button styling */
    .controls {
        display: flex;
        justify-content: left;
        margin-bottom: 5px;
        margin-top: 5px;
    }

    .action-btn {
        background-color: var(--c3);
        color: var(--f1);
        border: none;
        padding: 10px 15px;
        margin: 5px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1rem;
    }

    .action-btn:hover {
        background-color: var(--c3-hover);
    }

    /* Table styling */
    .table-container {
        border-radius: 4px;
        overflow: hidden;
        margin-top: 0px;
        margin-left: 0px;
        margin-right: 0px;
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
        color: var(--f1);
    }

    .table-row:hover {
        background-color: var(--c1);
        cursor: pointer;
    }

    /* Highlight selected study */

</style>

