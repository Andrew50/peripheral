<script lang="ts">
    import type { Setup, Instance } from '$lib/core/types';
    import {privateRequest} from "$lib/core/backend"
    import {onMount} from 'svelte'
    import {queryChart} from '$lib/features/chart/interface'
    import {ESTSecondstoUTCSeconds} from '$lib/core/timestamp'
    interface TrainingInstance extends Instance {
        sampleId: number
    }

    export let setup: Setup | null;
    export let handleExit: Function;
    let trainingQueue: TrainingInstance[] = []
    function showInstance(instance){
        console.log(instance)
        instance.timestamp = ESTSecondstoUTCSeconds(instance.timestamp)*1000
        queryChart(instance)
    }
    function refillQueue(){
        privateRequest<TrainingInstance[]>("getTrainingQueue",{setupId:setup.setupId},true)
        .then((v:TrainingInstance[])=>{
            trainingQueue = v
            showInstance(trainingQueue[0])
        })
    }
    onMount(()=>{
        refillQueue()
    })
    function label(c:string){
        if (c === "yes" || c === "no"){
            const boolLabel = (c === "yes" ? true : false)
            privateRequest<TrainingInstance[]>("labelTrainingQueueInstance",{sampleId:trainingQueue[0].sampleId,label:boolLabel})
        }
        trainingQueue.shift()
        if (!Array.isArray(trainingQueue) || trainingQueue.length == 0){
            refillQueue()
        }else{
            showInstance(trainingQueue[0])
        }
    }

</script>

<div class="feature-container">
    <button on:click={handleExit}>Exit</button>
    <h1>Setup: {setup?.name}</h1>
    <h1>Score: {setup?.score}</h1>
    
    <div>
    <h1>Is this a {setup?.name}?</h1>
    </div>
    <div>
    <button on:click={()=>label("yes")}>Yes</button>
    <button on:click={()=>label("no")}>No</button>
    <button on:click={()=>label("skip")}>Skip</button>
    </div>
</div>

<style>
</style>





