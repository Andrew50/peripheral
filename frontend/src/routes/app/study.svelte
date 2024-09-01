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
    }
    let studies : Writable<Study[]> = writable([])
</script>
<script lang="ts">
    let selectedStudyId: number | null = null;
    let entryStore = writable('');
    entryStore.subscribe((v:string)=>{
        if (v !== ""){
        }
    })
    function newStudy():void{
        queryInstanceInput(["ticker", "datetime"])
        .then((v:Instance) => {
            privateRequest<number>("newStudy",{securityId:v.securityId,datetime:v.datetime})
            .then((studyId:number) => {
                const study: Study = {studyId:studyId,...v}
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
        privateRequest<JSON>("getStudyEntry",{studyId:study.studyId})
        .then((entry: JSON) => {
            selectedStudyId = study.studyId
        })
    }
    function deleteStudy(study: Study):void{
        privateRequest<void>('deleteStudy',{studyId:study.studyId})
        .then(() => {studies.update((v:Study[]) => {
            return v.filter(item => item.studyId !== study.studyId)});
        })}

    onMount(() => {
        privateRequest<Study[]>("getStudies",{})
        .then((result: Study[]) => {studies.set(result)})
    })

</script>

<button on:click={newStudy}> new </button>
{#if Array.isArray($studies) && $studies.length > 0 }
    <table>
        <th> Ticker </th>
        <th> Date </th>
        {#each $studies as study}
            <tr on:click={()=>selectStudy(study)}>
                <td> {study.ticker} </td>
                <td> {study.datetime} </td>
            </tr>

            {#if selectedStudyId == study.studyId}
                <tr>
                <Entry func="Study" id={study.studyId}/>
                </tr>
            {/if}
        {/each}
    </table>
{/if}
