<script lang="ts">
	import { privateRequest } from '$lib/core/backend';
	import Entry from '$lib/utils/modules/entry.svelte';
	import { onMount } from 'svelte';
	import { ESTTimestampToESTString } from '$lib/core/timestamp';
	import type { Writable } from 'svelte/store';
	import { writable } from 'svelte/store';
	let selectedJournalId: number | null = null;
	import '$lib/core/global.css';
	interface Journal {
		journalId: number;
		timestamp: number;
		completed: boolean;
	}
	let journals: Writable<Journal[]> = writable([]);
	onMount(() => {
		privateRequest<Journal[]>('getJournals', {}).then((v: Journal[]) => {
			journals.set(v);
		});
	});
	function selectJournal(journal: Journal): void {
		if (journal.journalId === selectedJournalId) {
			selectedJournalId = 0;
		} else {
			selectedJournalId = journal.journalId;
		}
	}
</script>

<div class="journal-container">
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
						<tr
							class="journal-row {journal.completed ? '' : 'active'}"
							on:click={() => selectJournal(journal)}
						>
							<td>{ESTTimestampToESTString(journal.timestamp, true)}</td>
						</tr>

						{#if selectedJournalId == journal.journalId}
							<tr>
								<td colspan="2" class="entry-cell">
									<Entry completed={journal.completed} func="Journal" id={journal.journalId} />
								</td>
							</tr>
						{/if}
					{/each}
				{/if}
			</tbody>
		</table>
	</div>
</div>

<style>
	.journal-container {
		padding: 20px;
		color: var(--text-primary);
	}

	.table-container {
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		overflow: hidden;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	th {
		text-align: left;
		padding: 12px 16px;
		background: var(--ui-bg-secondary);
		color: var(--text-secondary);
		font-size: 12px;
		font-weight: 500;
		border-bottom: 1px solid var(--ui-border);
	}

	td {
		padding: 12px 16px;
		border-bottom: 1px solid var(--ui-border);
		font-size: 14px;
	}

	.journal-row {
		cursor: pointer;
		transition: background 0.2s ease;
	}

	.journal-row:hover {
		background: var(--ui-bg-hover);
	}

	.journal-row.active {
		color: var(--text-accent);
	}

	.entry-cell {
		background: var(--ui-bg-secondary);
		padding: 16px;
	}
</style>
