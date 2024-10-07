<!-- study.svelte-->
<script lang="ts" context="module">
    import type { Writable } from 'svelte/store'
    import '$lib/core/global.css'

    import { get,writable } from 'svelte/store'
    import {queryChart} from "$lib/features/chart/interface"
    import Entry from '$lib/utils/modules/entry.svelte'
    import {onMount} from 'svelte'
    import {privateRequest} from '$lib/core/backend'
    import {queryInstanceRightClick} from '$lib/utils/popups/rightClick.svelte'
    import type {Instance} from '$lib/core/types'
    import  {UTCTimestampToESTString} from '$lib/core/timestamp'
    import {queryInstanceInput} from '$lib/utils/popups/input.svelte'
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
            queryChart(study)
            selectedStudyId = study.studyId
/*            privateRequest<JSON>("getStudyEntry",{studyId:study.studyId})
            .then((entry: JSON) => {
                selectedStudyId = study.studyId
            })*/
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

<div class="controls-container">
    <button on:click={toggleCompletionFilter}> 
        {$completedFilter ? "Completed" : "Uncompleted"} 
    </button>
    <button on:click={newStudyRequest}> New </button>
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
                    <tr on:contextmenu={(event)=>queryInstanceRightClick(event,study,"header")} on:click={() => selectStudy(study)}>
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
