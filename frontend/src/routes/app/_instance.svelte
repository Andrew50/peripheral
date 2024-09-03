<!-- instance.svlete -->
<script lang="ts" context="module">
    import { privateRequest} from '../../store';
    import {newStudy} from './study.svelte';
    import {newJournal} from './journal.svelte';
    import {newSample} from './sample.svelte'
    import {changeChart} from './chart.svelte'
    import { get, writable } from 'svelte/store';
    import { parse} from 'date-fns';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '../../store';
    interface RightClickQuery extends Instance {
        x: number;
        y: number;
        source: string
        result: "edit" | null | "exit" | "embedSimilar" | "done"
    }
    interface SimilarInstance {
        x: number
        y: number
        instances: Instance[]
    }
    let rightClickQuery: Writable<RightClickQuery | null> = writable(null);
    let similarInstance: Writable<SimilarInstance | null> = writable(null);

    export async function queryInstanceRightClick(event:MouseEvent,instance:Instance,source:string):Promise<RightClickResult>{
        rightClickQuery.set(null)
        return new Promise<string>((resolve, reject) => {
            const rightClick: RightClickQuery = {
                x: event.clientX,
                y: event.clientY,
                source: source,
                result: null,
                ...instance
            }
            rightClickQuery.set(rightClick)
            const unsubscribe = rightClickQuery.subscribe((r: RightClickQuery)=>{
                if (r.result === "exit"){
                    unsubscribe()
                    reject()
                }else if(r.result !== null){
                    unsubscribe()
                    const res = r.result
                    rightClickQuery.set(null)
                    resolve(res)
                }
            })
        })
    }

    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string | null;
        name: string;
    }
    type InstanceQuery = Instance | "cancelled"
    let isVisible = writable(false);
    let inputString = writable("")
    let inputType = writable("")
    let requiredKeys: Writable<Array<string>> = writable([])

    interface InputQuery extends Instance {
        result: null | "exit" | "done" 
    }

    let inputQuery: Writable<Instance> = writable({})

    export async function queryInstanceInput(required: Array<keyof Instance> | "any",instance:Instance={}): Promise<Instance> {
        if (required != "any"){
            requiredKeys.set(required)
        }
        function cleanup(){
            inputString.set("")
            inputType.set("")
            isVisible.set(false)
            inputQuery.set({})
        }
        if(instance){
            inputQuery.set(instance)
        }
        return new Promise<Instance>((resolve, reject) => {
            isVisible.set(true);
            const unsubscribe = inputQuery.subscribe((ins: InstanceQuery) => {
                if (ins == "cancelled"){
                    unsubscribe()
                    cleanup()
                    reject()
                }else{
                    let complete = true
                    if(required === "any"){ //otherwise it is completed becuase something was changed so any fullfilled
                        if (Object.keys(ins).length === 0){complete = false}
                    }else{
                        for (const typ of required){
                            if (!ins[typ]){
                                complete = false
                                break;
                            }
                        }
                    }
                    if (complete){
                        unsubscribe()
                        const re = get(inputQuery)
                        cleanup()
                        resolve(re as Instance)
                    } } }) }) }
        
</script>
<script lang="ts">
    import {browser} from '$app/environment'
    import {onMount} from 'svelte'
    let securities: Security[] = [];
    let isValid = false;
    let rightClickMenu: HTMLElement;
    function validateInput(input= get(inputString), typ= get(inputType)):boolean{
        //const input = get(inputString)
        //const typ = get(inputType)
        if (typ == "ticker"){
            if (!(Array.isArray(securities) && securities.length > 0)){
                return false
            }
            return true
        }else if(typ == "timeframe"){
            console.log("gid")
            const regex = /^\d{1,3}[yqmwds]?$/i;
            return regex.test(input)
        }else if(typ == "datetime"){
            const formats = ["yyyy-MM-dd H:m:ss","yyyy-MM-dd H:m","yyyy-MM-dd H","yyyy-MM-dd",];
            for (const format of formats) {
                try {
                    const parsedDate = parse(input, format, new Date())
                    if (parsedDate != "Invalid Date"){
                        privateRequest<boolean>("validateDateString",{dateString:input})
                        .then((v:boolean)=>{return v})
                    }
                }catch{} }
            return false
        }
        return false
    }

    function classifyInput(input: string): string{
        if (input) {
            if(/-/.test(input)){
                return "datetime"
            }
            else if(/^[0-9]$/.test(input[0])){
                return "timeframe"
            }else{
                return "ticker"
            }
        }else{
            return "";
        }
    }
    function enterInput(index: number = 0):void{
        if (get(inputType) === "ticker"){
            if (Array.isArray(securities) && securities.length > 0) {
                inputQuery.update((instance: Instance) => {
                    instance.securityId = securities[index].securityId
                    instance.ticker = securities[index].ticker
                    return instance
                })
            }
        }else if (get(inputType) === 'timeframe'){
            inputQuery.update((instance: Instance) => {
                instance.timeframe = get(inputString)
                return instance
            })
        }else if (get(inputType) === 'datetime'){
            inputQuery.update((instance: Instance) => {
                instance.datetime = get(inputString)
                return instance
            })
        }
        inputString.set("")
    }
    function handleKeyDown(event:KeyboardEvent):void {
        if (event.key === 'Escape') {
            inputQuery.set("cancelled")
        }else if (event.key === 'Enter') {
            event.preventDefault()
            enterInput(0)
        }else {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase()) 
                || (/[-:]/.test(event.key)) 
                || (event.key == " " && get(inputType) === 'datetime') ) {
                let key: string;
                if (get(inputType) === 'timeframe'){
                    key = event.key
                }else{
                    key = event.key.toUpperCase()
                }
                inputString.update((v:string)=>{return v + key})
            }else if (event.key == "Backspace") {
                inputString.update((v)=>{return v.slice(0, -1)})
            }
            inputType.set(classifyInput(get(inputString)))
            if(get(inputType) === "ticker") {
                isValid = true
                privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:get(inputString)})
                .then((result: Security[]) => {
                    securities = result
                    isValid = validateInput()
                    })
            }else{
               isValid = validateInput() 
               securities = []
            }
        }
    }
    onMount(()=>{
        isVisible.subscribe((v:boolean)=>{
            if (browser){
                if (v){
                    document.addEventListener('keydown',  handleKeyDown);
                }else{
                    document.removeEventListener('keydown',handleKeyDown);
                }
            }
        })
        rightClickQuery.subscribe((v:RightClickQuery) => {
            if (browser){
                if (v !== null){
                    document.addEventListener('click',handleClick)
                    document.addEventListener('keydown', rightClickKeyDown)
                }else{
                    document.removeEventListener('click',handleClick)
                    document.removeEventListener('keydown', rightClickKeyDown)
                }
            }
        })
    })
    function handleClick(event:MouseEvent):void{
        if (rightClickMenu && !rightClickMenu.contains(event.target as Node)) {
            closeRightClickMenu()
        }
    }
    function closeRightClickMenu():void{
        rightClickQuery.set(null)
    }
    function rightClickKeyDown(event:KeyboardEvent):void{
        if (event.key == "Escape"){
            closeRightClickMenu()
        }
    }

    function getStats():void{}
    function replay():void{}
    function addAlert():void{}
    function embed():void{}
    function edit():void{
        rightClickQuery.update((v:RightClickQuery)=>{
            v.result = "edit"
            return v
        })
    }
    function embedSimilar():void{
        rightClickQuery.update((v:RightClickQuery)=>{
            v.result = "embedSimilar"
            return v
        })
    }

    function getSimilarInstances(event:MouseEvent):void{
        const baseIns = get(rightClickQuery)
        privateRequest<Instance[]>("getSimilarInstances",{ticker:baseIns.ticker,securityId:baseIns.securityId,timeframe:baseIns.timeframe,datetime:baseIns.datetime})
        .then((v:Instance[])=>{
            console.log(v)
            const simInst: SimilarInstance = {
                x: event.clientX,
                y: event.clientY,
                instances: v
            }
            console.log(simInst)
            similarInstance.set(simInst)
        })
    }

</script>
{#if $rightClickQuery !== null}
    <div bind:this={rightClickMenu} class="context-menu" style="top: {$rightClickQuery.y}px; left: {$rightClickQuery.x}px;">
        <div>{$rightClickQuery.ticker} {$rightClickQuery.datetime} </div>
        <div><button on:click={()=>newStudy(get(rightClickQuery))}> Add to Study </button></div>
        <div><button on:click={()=>newSample(get(rightClickQuery))}> Add to Sample </button></div>
        <div><button on:click={()=>newJournal(get(rightClickQuery))}> Add to Journal </button></div>
        <div><button on:click={getSimilarInstances}> Similar Instances </button></div>
        <div><button on:click={getStats}> Instance Stats </button></div>
        <div><button on:click={replay}> Replay </button></div>
        {#if $rightClickQuery.source === "chart"}
            <div><button on:click={addAlert}>Add Alert </button></div>
            <div><button on:click={embed}> Embed </button></div>
        {:else if $rightClickQuery.source === "embedded"}
            <div><button on:click={edit}> Edit </button></div>
            <div><button on:click={embedSimilar}> Embed Similar </button></div>
        {/if}
    </div>
{/if}

{#if $similarInstance !== null}
    <div class="context-menu" style="top: {$similarInstance.y}px; left: {$similarInstance.x}px;">
        <table>
        {#each $similarInstance.instances as instance} 
            <tr>
                <td on:click={()=>changeChart(instance)} on:contextmenu={(e)=>{e.preventDefault();queryInstanceRightClick(e,instance,"similar")}}>{instance.ticker}</td>
            </tr>
        {/each}
        </table>
    </div>
{/if}

{#if $isVisible}
    <div class="popup">
    {#each $requiredKeys as key}
        <div class="{validateInput(key, $inputQuery[key]) ? 'normal' : 'red'}">{key} {$inputQuery[key]}</div>
    {/each}
    <div>{$inputString}</div>
    <div class="{isValid ? 'normal' : 'red'}">{$inputType}</div>
        <table>
            {#if Array.isArray(securities) && securities.length > 0}
            <th> Ticker <th/>
            <th> Date </th>
            {#each securities as sec, i}
                <tr on:click={() => enterInput(i)}> 
                    <td>{sec.ticker}</td>
                    <td>{sec.maxDate === null ? 'Current' : sec.maxDate}</td> 
                </tr>
            {/each}
            {/if}
        </table>
    </div>
{/if}


<style>
    .red {
        color: red;
    }
    .normal {
        color: black;
    }
    .context-menu {
        position: absolute;
        background-color: white;
        border: 1px solid #ccc;
        box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
        z-index: 1000;
        padding: 10px;
        border-radius: 4px;
    }

    .context-menu-item {
        background-color: transparent;
        border: none;
        padding: 5px 10px;
        text-align: left;
        cursor: pointer;
        width: 100%;
    }

    .context-menu-item:hover {
        background-color: #f0f0f0;
    }
    .popup {
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        background-color: white;
        border: 1px solid #ccc;
        z-index: 1000;
        padding: 20px;
        width: 300px;
        box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.5);
    }
    .hidden {
        display: none;
    }
</style>

