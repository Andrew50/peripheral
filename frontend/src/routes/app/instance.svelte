<!-- instance.svlete -->
<script lang="ts" context="module">
    import { privateRequest} from '../../store';
    import { get, writable } from 'svelte/store';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '../../store';
    export interface RightClickInstance extends Instance {
        x: number;
        y: number;
    }
    export let rightClickInstance: Writable<RightClickInstance | null> = writable(null);
    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string | null;
        name: string;
    }
    type InstanceQuery = Instance | "cancelled"
    let isVisible = writable(false);
    let inputString = writable("")
    let instanceQuery: Writable<Instance> = writable({})
    let inputType = writable("")
    let requiredKeys: Writable<Array<string>> = writable([])

    export async function queryInstanceInput(required: Array<keyof Instance> | "any"): Promise<Instance> {
        if (required != "any"){
            requiredKeys.set(required)
        }
        function cleanup(){
            inputString.set("")
            inputType.set("")
            isVisible.set(false)
            instanceQuery.set({})
        }
        return new Promise<Instance>((resolve, reject) => {
            isVisible.set(true);
            const unsubscribe = instanceQuery.subscribe((ins: InstanceQuery) => {
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
                        const re = get(instanceQuery)
                        cleanup()
                        resolve(re as Instance)
                    } } }) }) }
        
</script>
<script lang="ts">
    import {browser} from '$app/environment'
    let securities: Security[] = [];
    function classifyInput(input: string): string{
        if (input) {
            if(/-/.test(input)){
                return "datetime"
            }
            else if(/^[0-9]$/.test(input[0])){
                return "interval"
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
                instanceQuery.update((instance: Instance) => {
                    instance.securityId = securities[index].securityId
                    instance.ticker = securities[index].ticker
                    return instance
                })
            }
        }else if (get(inputType) === 'interval'){
            instanceQuery.update((instance: Instance) => {
                instance.timeframe = get(inputString)
                return instance
            })
        }else if (get(inputType) === 'datetime'){
            instanceQuery.update((instance: Instance) => {
                instance.datetime = get(inputString)
                return instance
            })
        }
        inputString.set("")
    }
    function handleKeyDown(event:KeyboardEvent):void {
        if (event.key === 'Escape') {
            instanceQuery.set("cancelled")
        }else if (event.key === 'Enter') {
            event.preventDefault()
            enterInput(0)
        }else {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase()) 
                || (/[-:]/.test(event.key)) 
                || (event.key == "Space" && get(inputType) === 'datetime') ) {
                inputString.update((v:string)=>{return v + event.key.toUpperCase()})
            }else if (event.key == "Backspace") {
                inputString.update((v)=>{return v.slice(0, -1)})
            }
            inputType.set(classifyInput(get(inputString)))
            if(get(inputType) === "ticker") {
                privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:get(inputString)})
                .then((result: Security[]) => securities = result)
            }
        }
    }
    isVisible.subscribe((v:boolean)=>{
        if (browser){
            if (v){
                document.addEventListener('keydown',  handleKeyDown);
            }else{
                document.removeEventListener('keydown',handleKeyDown);
            }
        }
    })

</script>

{#if $isVisible}
    <div class="popup">
    {#each $requiredKeys as key}
        <div>{key} {$instanceQuery[key]}</div>
    {/each}
    <div>{$inputString}</div>
    <div>{$inputType}</div>
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

{#if $rightClickInstance !== null}
    <div class="context-menu" style="top: {$rightClickInstance.y}; left: {$rightClickInstance.x};">
        <div>{$rightClickInstance.ticker} {$rightClickInstance.datetime} </div>
        <button class="context-menu-item" on:click={() => {
            if ($rightClickInstance !== null){
            //    newStudy($rightClickInstance)TODO
            }
        }}> Add to Study </button>
    </div>
{/if}

<style>
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

