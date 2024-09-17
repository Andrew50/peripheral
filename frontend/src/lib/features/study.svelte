<!-- study.svelte-->
<script lang="ts" context="module">
    import type { Writable } from 'svelte/store'
    import { get,writable } from 'svelte/store'
    import {changeChart} from "$lib/features/chart/interface"
    import Entry from './entry.svelte'
    import {onMount} from 'svelte'
    import {privateRequest} from '$lib/core/backend'
    import type {Instance} from '$lib/core/types'
    import  {UTCTimestampToESTString} from '$lib/core/timestamp'
    import {queryInstanceInput} from '$lib/utils/input.svelte'
    interface Study extends Instance{
        studyId: number;
        completed: boolean;
    }
    let studies : Writable<Study[]> = writable([])
    export function newStudy(v:Instance):void{
            privateRequest<number>("newStudy",{securityId:v.securityId,timestamp:v.timestamp})
            .then((studyId:number) => {
                const study: Study = {completed:false,studyId:studyId,...v}
                studies.update((vv:Study[]) => {
                    if (Array.isArray(vv)){
                        return [...vv,study]
                    }else{
                        return [study]
                    }
                })
            }).catch()
    }

</script>
<script lang="ts">
    let selectedStudyId: number | null = null;
    let entryStore = writable('');
    let completedFilter = writable(false);
    entryStore.subscribe((v:string)=>{
        if (v !== ""){
        }
    })
    function newStudyRequest():void{
        const insTemplate: Instance = {ticker:"",timestamp:0}
        queryInstanceInput(["ticker", "timestamp"],insTemplate)
        .then((v:Instance) => {newStudy(v)})
    }
    function selectStudy(study: Study) : void {
        if (study.studyId === selectedStudyId){
            selectedStudyId = 0
        }else{
            changeChart(study)
            privateRequest<JSON>("getStudyEntry",{studyId:study.studyId})
            .then((entry: JSON) => {
                selectedStudyId = study.studyId
            })
        }
    }
    function deleteStudy(study: Study):void{
        privateRequest<void>('deleteStudy',{studyId:study.studyId})
        .then(() => {studies.update((v:Study[]) => {
            return v.filter(item => item.studyId !== study.studyId)});
        })}

    function toggleCompletionFilter():void{
        completedFilter.update(v=>!v)// = !completedFilter
        loadStudies()
    }


    function loadStudies():void{
        privateRequest<Study[]>("getStudies",{completed:get(completedFilter)})
        .then((result: Study[]) => {studies.set(result)})
    }
    onMount(() => {
        loadStudies()
    })

</script>

<div class="controls">
    <button on:click={toggleCompletionFilter} class="action-btn"> 
        {$completedFilter ? "Completed" : "Uncompleted"} 
    </button>
    <button on:click={newStudyRequest} class="action-btn"> New </button>
</div>

<div class="table-container">
    <table>
        <thead>
            <tr>
                <th>Ticker</th>
                <th>Date</th>
            </tr>
        </thead>
        <tbody>
            {#if Array.isArray($studies) && $studies.length > 0}
                {#each $studies as study}
                    <tr class="table-row" on:click={() => selectStudy(study)}>
                        <td>{study.ticker}</td>
                        <td>{UTCTimestampToESTString(study.timestamp)}</td>
                    </tr>

                    {#if selectedStudyId == study.studyId}
                        <tr>
                            <td colspan="2">
                                <Entry completed={study.completed} func="Study" id={study.studyId} />
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
