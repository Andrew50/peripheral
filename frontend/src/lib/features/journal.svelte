<script lang="ts">
    import {privateRequest} from '$lib/core/backend'
    import Entry from "$lib/utils/modules/entry.svelte"
    import {onMount} from 'svelte'
    import {ESTTimestampToESTString} from '$lib/core/timestamp'
    import type {Writable} from 'svelte/store'
    import {writable} from 'svelte/store'
    let selectedJournalId: number | null = null;
    import '$lib/core/global.css'
    interface Journal {
        journalId: number,
        timestamp: number,
        completed: boolean
    }
    let journals: Writable<Journal[]> = writable([])
    onMount(() => {
        privateRequest<Journal[]>("getJournals",{})
        .then((v:Journal[]) => { journals.set(v)})
    })
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
                    <tr class={journal.completed ? "" : "active"}  on:click={()=>selectJournal(journal)}>
                        <td>{ESTTimestampToESTString(journal.timestamp,true)}</td>
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
