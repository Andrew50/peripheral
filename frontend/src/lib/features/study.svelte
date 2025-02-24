<!-- study.svelte-->
<script lang="ts" context="module">
	import type { Writable } from 'svelte/store';
	import '$lib/core/global.css';

	import { get, writable } from 'svelte/store';
	import { queryChart } from '$lib/features/chart/interface';
	import Entry from '$lib/utils/modules/entry.svelte';
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import { queryInstanceRightClick } from '$lib/utils/popups/rightClick.svelte';
	import type { Instance } from '$lib/core/types';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { setups } from '$lib/core/stores';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	interface Study extends Instance {
		studyId: number;
		completed: boolean;
		setupId?: number;
	}
	let studies: Writable<Study[]> = writable([]);
	export function newStudy(v: Instance): void {
		privateRequest<number>('newStudy', { securityId: v.securityId, timestamp: v.timestamp })
			.then((studyId: number) => {
				const study: Study = { completed: false, studyId: studyId, ...v };
				studies.update((vv: Study[]) => {
					if (Array.isArray(vv)) {
						return [...vv, study];
					} else {
						return [study];
					}
				});
			})
			.catch();
	}
</script>

<script lang="ts">
	let selectedStudyId: number | null = null;
	let entryStore = writable('');
	let completedFilter = writable(false);
	entryStore.subscribe((v: string) => {
		if (v !== '') {
		}
	});
	function newStudyRequest(): void {
		const insTemplate: Instance = { ticker: '', timestamp: 0 };
		queryInstanceInput(['ticker', 'timestamp'], ['ticker', 'timestamp'], insTemplate).then(
			(v: Instance) => {
				newStudy(v);
			}
		);
	}
	function selectStudy(study: Study): void {
		if (study.studyId === selectedStudyId) {
			selectedStudyId = 0;
		} else {
			queryChart(study);
			selectedStudyId = study.studyId;
		}
	}

	function toggleCompletionFilter(): void {
		completedFilter.update((v) => !v); // = !completedFilter
		loadStudies();
	}

	function loadStudies(): void {
		privateRequest<Study[]>('getStudies', { completed: get(completedFilter) }).then(
			(result: Study[]) => {
				studies.set(result);
			}
		);
	}
	onMount(() => {
		loadStudies();
	});
	function getSetupNameById(setupId: number) {
		setupId;
		const setup = $setups.find((s) => s.setupId === setupId);
		return setup ? setup.name : null; // Return setupName if found, otherwise return null
	}
</script>

<div class="study-container">
	<div class="controls-container">
		<button on:click={toggleCompletionFilter}>
			{$completedFilter ? 'Completed' : 'Uncompleted'}
		</button>
		<button on:click={newStudyRequest}> New </button>
	</div>

	<div class="table-container">
		<table>
			<thead>
				<tr class="defalt-tr">
					<th class="defalt-th">Ticker</th>
					<th class="defalt-th">Setup</th>
					<th class="defalt-th">Date</th>
				</tr>
			</thead>
			<tbody>
				{#if Array.isArray($studies) && $studies.length > 0}
					{#each $studies as study}
						<tr
							class="study-row"
							on:contextmenu={(event) => queryInstanceRightClick(event, study, 'header')}
							on:click={() => selectStudy(study)}
						>
							<td class="defalt-td">{study.ticker || 'N/A'}</td>
							<td class="defalt-td">{study.setupId ? getSetupNameById(study.setupId) : 'N/A'}</td>
							<td class="defalt-td"
								>{study.timestamp ? UTCTimestampToESTString(study.timestamp) : 'N/A'}</td
							>
						</tr>

						{#if selectedStudyId == study.studyId}
							<tr class="defalt-tr">
								<td colspan="3" class="entry-cell">
									<Entry
										completed={study.completed}
										setupId={study.setupId}
										func="Study"
										id={study.studyId}
									/>
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
	.study-container {
		padding: 20px;
		color: var(--text-primary);
	}

	.controls-container {
		display: flex;
		gap: 10px;
		margin-bottom: 20px;
	}

	.controls-container button:hover {
		background: var(--ui-bg-hover);
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

	.study-row {
		cursor: pointer;
		transition: background 0.2s ease;
	}

	.study-row:hover {
		background: var(--ui-bg-hover);
	}

	.entry-cell {
		background: var(--ui-bg-secondary);
		padding: 16px;
	}
</style>
