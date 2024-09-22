<script lang='ts'>
    import {privateRequest} from '$lib/core/backend'
    import type {Instance} from '$lib/core/types'
    import {changeChart} from '$lib/features/chart/interface'
    import {onMount, onDestroy} from 'svelte'
    import '$lib/core/global.css'

    const tfs = ["1w","1d","1h","1"]
    let instances: Instance[] = [];

    let loopActive = false;
    let securityIndex = 0;
    let tfIndex = 0;
    let speed = 5; //seconds

    function loop(){
        const instance = instances[securityIndex]
        instance.timeframe = tfs[tfIndex];
        changeChart(instance)
        tfIndex ++
        if (tfIndex >= tfs.length){
            tfIndex = 0
            securityIndex ++
            if (securityIndex >= instances.length){
                securityIndex = 0
            }
        }
        if (loopActive){
            setTimeout(()=>{
                loop()
            },speed*1000)
        }
    }

    onMount(() => {
        privateRequest<Instance[]>("getScreensavers",{}).then((v:Instance[]) => {
            instances = v;
            loopActive = true
            loop()
        })
    })
    
    onDestroy(()=>{
        loopActive = false
    })


</script>

<div>
Speed
</div>
<div>
<input bind:value={speed}/>
</div>


        

