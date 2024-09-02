<!-- study.svelte-->
<script lang="ts" context="module">
    import type { Writable } from 'svelte/store'
    import { writable } from 'svelte/store'
    import Entry from './entry.svelte'
    import {onMount} from 'svelte'
    import {privateRequest} from '../../store'
    import type {Instance} from '../../store'
    import {queryInstanceInput} from './instance.svelte'
    interface Study extends Instance{
        studyId: number;
        completed: boolean;
    }
    let studies : Writable<Study[]> = writable([])

</script>
<script lang="ts">
    let selectedStudyId: number | null = null;
    let entryStore = writable('');
    let completedFilter = false;
    entryStore.subscribe((v:string)=>{
        if (v !== ""){
        }
    })
    function newStudy():void{
        queryInstanceInput(["ticker", "datetime"])
        .then((v:Instance) => {
            privateRequest<number>("newStudy",{securityId:v.securityId,datetime:v.datetime})
            .then((studyId:number) => {
                const study: Study = {completed:false,studyId:studyId,...v}
                studies.update((vv:Study[]) => {
                    if (Array.isArray(vv)){
                        return [...vv,study]
                    }else{
                        return [study]
                    }
                })
            })

        })
    }
    function selectStudy(study: Study) : void {
        if (study.studyId === selectedStudyId){
            selectedStudyId = 0
        }else{
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
        completedFilter = !completedFilter
        loadStudies()
    }


    function loadStudies():void{
        privateRequest<Study[]>("getStudies",{completed:completedFilter})
        .then((result: Study[]) => {studies.set(result)})
    }
    onMount(() => {
        loadStudies()
    })

</script>

<h1> Study </h1>
<button on:click={toggleCompletionFilter}> {completedFilter ? "Completed" : "Uncompeted"}</button>
<button on:click={newStudy}> new </button>
    <table>
        <th> Ticker </th>
        <th> Date </th>
{#if Array.isArray($studies) && $studies.length > 0 }
        {#each $studies as study}
            <tr on:click={()=>selectStudy(study)}>
                <td> {study.ticker} </td>
                <td> {study.datetime} </td>
            </tr>

            {#if selectedStudyId == study.studyId}
                <tr>
                <Entry completed={study.complete} func="Study" id={study.studyId}/>
                </tr>
            {/if}
        {/each}
{/if}
    </table>
