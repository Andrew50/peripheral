<script lang="ts" context="module">
    import type { Writable } from 'svelte/store'
    import { writable } from 'svelte/store'
    import Entry from './entry.svelte'
    import {onMount} from 'svelte'
    import {privateRequest} from '../../store'
    import type {Instance} from '../../store'
    interface Study extends Instance{
        studyId: number;
    }
    let studies : Writable<Study[]> = writable([])
    export function newStudy(instance:Instance):void{
        //instanceInputTarget.set(newInstance)
        privateRequest<number>("newStudy",{instance:instance})
        .then((studyId:number) => {
            const study: Study = {studyId:studyId,...instance}
            studies.update((v:Study[]) => {return [...v,study]})
        })

    }
</script>
<script lang="ts">
    let selectedStudyId: number | null = null;
    let entryStore = writable('');
    entryStore.subscribe((v:string)=>{
        if (v !== ""){
            privateRequest<void>('saveStudy',{studyId:selectedStudyId,entryString:v})
            .then(selectedStudyId = null)
        }
    })

    let newInstance: Writable<Instance> = writable({})
    newInstance.subscribe((v:Instance) => {
        if (v.datetime && v.ticker){
            newInstance.set({})
        }
    })


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
{$newInstance.ticker}
{$newInstance.datetime}
{#if selectedStudyId != null}
    <Entry store={entryStore}/>
{:else if Array.isArray($studies) && $studies.length > 0 }
    <table>
        <th> Ticker </th>
        <th> Date </th>
        {#each $studies as study}
            <tr> {study.ticker} </tr>
            <tr> {study.datetime} </tr>
        {/each}
    </table>
{/if}
