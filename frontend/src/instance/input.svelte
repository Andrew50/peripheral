<!-- instance.svlete -->
<script lang="ts" context="module">
    import { privateRequest} from '../../store';
    import { get, writable } from 'svelte/store';
    import { parse} from 'date-fns';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '../../store';
    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string | null;
        name: string;
    }
    type InstanceAttributes = "ticker" | "timeframe" | "datetime"
    interface InputQuery {
        //inactive is default, no ui shown | complete is succesful completeion, return instance
        // cancelled is user closed with escape, dont return anything
        // active is window is open, waiting for cancellation or completion
        //init is setting up event handlers |  c

        status: "inactive" | "initializing" | "active" | "complete" | "cancelled"
        inputString: string
        inputType: string
        instance: Instance
        requiredKeys: InstanceAttributes[] | "any"
        securities?: Security[]
    }

    const inactiveInputQuery: InputQuery = {
        status: "inactive",
        inputString: "",
        inputType:"",
        requiredKeys: "any",
        instance: {}
    }
    let inputQuery: Writable<InputQuery> = writable(inactiveInputQuery)

    export async function queryInstanceInput(requiredKeys: InstanceAttributes[] | "any",instance:Instance={}): Promise<Instance> {
        //init the query with passsed info
        inputQuery.update((v:InputQuery)=>{
            v.requiredKeys = requiredKeys
            v.instance = instance
            v.status = "initializing"
            return v
        })
        return new Promise<Instance>((resolve, reject) => {
            const unsubscribe = inputQuery.subscribe((iQ: InputQuery) => {
                if (iQ.status === "cancelled"){
                    deactivate()
                    reject()
                }else if (iQ.status === "active"){
                    let complete = true
                    if(iQ.requiredKeys === "any"){ //otherwise it is completed becuase something was changed so any fullfilled
                        if (Object.keys(iQ.instance).length === 0){complete = false}
                    }else{
                        for (const attribute of iQ.requiredKeys){
                            if (!iQ.instance[attribute]){
                                complete = false
                                break;
                            }
                        }
                    }
                    if (complete){
                        const re = iQ.instance
                        deactivate()
                        resolve(re)
                    }
                }
            })
            function deactivate(){
                unsubscribe()
                inputQuery.set(inactiveInputQuery)
            }
        })
    }
        
</script>
<script lang="ts">
    import {browser} from '$app/environment'
    import {onMount} from 'svelte'
    let securities: Security[] = [];
    let isValid = false;
    function validateInput(inputString = get(inputQuery).inputString, inputType = get(inputQuery).inputType):boolean{
        if (inputType == "ticker"){
            if (!(Array.isArray(securities) && securities.length > 0)){
                return false
            }
            return true
        }else if(inputType == "timeframe"){
            console.log("gid")
            const regex = /^\d{1,3}[yqmwds]?$/i;
            return regex.test(inputString)
        }else if(inputQuery == "datetime"){
            const formats = ["yyyy-MM-dd H:m:ss","yyyy-MM-dd H:m","yyyy-MM-dd H","yyyy-MM-dd",];
            for (const format of formats) {
                try {
                    const parsedDate = parse(inputString, format, new Date())
                    if (parsedDate != "Invalid Date"){
                        privateRequest<boolean>("validateDateString",{dateString:inputString})
                        .then((v:boolean)=>{return v})
                    }
                }catch{} }
            return false
        }
        return false
    }

    function classifyInput(inputString: string): string{
        if (inputString) {
            if(/-/.test(inputString)){
                return "datetime"
            }
            else if(/^[0-9]$/.test(inputString[0])){
                return "timeframe"
            }else{
                return "ticker"
            }
        }else{
            return "";
        }
    }
    function enterInput(tickerIndex: number = 0):void{
        const inputType = get(inputQuery).inputType
        const inputString = get(inputQuery).inputString
        if (inputType === "ticker"){
            if (Array.isArray(securities) && securities.length > 0) {
                inputQuery.update((instance: Instance) => {
                    instance.securityId = securities[tickerIndex].securityId
                    instance.ticker = securities[tickerIndex].ticker
                    return instance
                })
            }
        }else if (inputType === 'timeframe'){
            inputQuery.update((instance: Instance) => {
                instance.timeframe = inputString
                return instance
            })
        }else if (inputType === 'datetime'){
            inputQuery.update((instance: Instance) => {
                instance.datetime = inputString
                return instance
            })
        }
        inputQuery.update((v:InputQuery)=>{
            v.inputString = ""
            return v
        })
    }
    function handleKeyDown(event:KeyboardEvent):void {
        let inputType = get(inputQuery).inputType
        let inputString = get(inputQuery).inputString
        let status = get(inputQuery).status
        if (event.key === 'Escape') {
            status = "cancelled"
        }else if (event.key === 'Enter') {
            event.preventDefault()
            enterInput(0)
        }else {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase()) 
                || (/[-:]/.test(event.key)) 
                || (event.key == " " && get(inputType) === 'datetime') ) {
                let key: string;
                if (inputType === 'timeframe'){
                    key = event.key
                }else{
                    key = event.key.toUpperCase()
                }
                inputString += key
            }else if (event.key == "Backspace") {
                inputString = inputString.slice(0,-1)
            }
            inputType = classifyInput(inputString)
            if(inputType === "ticker") {
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
        inputQuery.update((v:InputQuery)=>{
            return {...v, inputType:inputType,inputString:inputString}
        })
    }
    onMount(()=>{
        inputQuery.subscribe((v:InputQuery)=>{
            if (browser){
                if (v.status === "initializing"){
                    document.addEventListener('keydown',  handleKeyDown);
                    v.status = "active"
                    return v
                }else if(v.status==="inactive"){
                    document.removeEventListener('keydown',handleKeyDown);
                }
            }
        })


</script>


{#if $inputQuery.status === "active"}
    <div class="popup">
    {#each $inputQuery.requiredKeys as key}
        <div class="{validateInput(key, $inputQuery.instance[key]) ? 'normal' : 'red'}">{key} {$inputQuery.instance[key]}</div>
    {/each}
    <div>{$inputQuery.inputString}</div>
    <div class="{isValid ? 'normal' : 'red'}">{$inputQuery.inputType}</div>
        <table>
            {#if Array.isArray(securities) && securities.length > 0}
            <th> Ticker <th/>
            <th> Date </th>
            {#each inputQuery.securities as sec, i}
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

