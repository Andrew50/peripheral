<!-- instance.svlete -->
<script lang="ts" context="module">
    import { privateRequest} from '$lib/api/backend';
    import { get, writable } from 'svelte/store';
    import { parse} from 'date-fns';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '$lib/api/backend';
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
        inputValid: boolean
        instance: Instance
        requiredKeys: InstanceAttributes[] | "any"
        securities?: Security[]
    }

    const inactiveInputQuery: InputQuery = {
        status: "inactive",
        inputString: "",
        inputValid: true,
        inputType:"",
        requiredKeys: "any",
        instance: {}
    }
    let inputQuery: Writable<InputQuery> = writable(inactiveInputQuery)

    export async function queryInstanceInput(requiredKeys: InstanceAttributes[] | "any",instance:Instance={}): Promise<Instance> {
        //init the query with passsed info
        inputQuery.update((v:InputQuery)=>{
            v.requiredKeys = requiredKeys
        /*    v.instance = {
                ticker:"",
                datetime:"",
                timeframe:"",
                ...instance
            },*/
            v.instance = instance //instance must be set up to have required fields as blank
            v.status = "initializing"
            return v
        })
        return new Promise<Instance>((resolve, reject) => {
            const unsubscribe = inputQuery.subscribe((iQ: InputQuery) => {
                if (iQ.status === "cancelled"){
                    deactivate()
                    reject()
                }else if(iQ.status === "complete"){
                    const re = iQ.instance
                    deactivate()
                    resolve(re)
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
    function validateInput(inputString: string, inputType: string):boolean{
        if (inputType === "ticker"){
            const securities = get(inputQuery).securities
            if (!(Array.isArray(securities) && securities.length > 0)){
                return false
            }
            return true
        }else if(inputType == "timeframe"){
            const regex = /^\d{1,3}[yqmwds]?$/i;
            return regex.test(inputString)
        }else if(inputQuery == "datetime"){
            const formats = ["yyyy-MM-dd H:m:ss","yyyy-MM-dd H:m","yyyy-MM-dd H","yyyy-MM-dd",];
            for (const format of formats) {
                try {
                    const parsedDate = parse(inputString, format, new Date())
                    if (parsedDate != "Invalid Date"){
                        return true
                    //    privateRequest<boolean>("validateDateString",{dateString:inputString})
                     //   .then((v:boolean)=>{return v})
                    }
                }catch{} }
            return false
        }
        return false
    }

    function enterInput(iQ:InputQuery,tickerIndex: number = 0):InputQuery{
        if (iQ.inputType === "ticker"){
            const securities = iQ.securities
            if (Array.isArray(securities) && securities.length > 0) {
                iQ.instance.securityId = securities[tickerIndex].securityId
                iQ.instance.ticker = securities[tickerIndex].ticker
            }
        }else if (iQ.inputType === 'timeframe'){
            iQ.instance.timeframe = iQ.inputString
        }else if (iQ.inputType === 'datetime'){
            iQ.instance.datetime = iQ.inputString
        }
        iQ.status = "complete" // temp setting, following code will set back to active
        if(iQ.requiredKeys === "any"){ 
            if (Object.keys(iQ.instance).length === 0){iQ.status = "active"}
        }else{
            for (const attribute of iQ.requiredKeys){
                if (!iQ.instance[attribute]){
                    iQ.status = "active"
                    break;
                }
            }
        }
        iQ.inputString = ""
        iQ.inputType = ""
        iQ.inputValid = true
        return iQ
    }
    function handleKeyDown(event:KeyboardEvent):void {
        let iQ = get(inputQuery)
        if (event.key === 'Escape') {
            iQ.status = "cancelled"
        }else if (event.key === 'Enter') {
            event.preventDefault()
            iQ = enterInput(iQ,0)
        }else {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase()) 
                || (/[-:]/.test(event.key)) 
                || (event.key == " " && iQ.inputType === 'datetime') ) {
                let key: string;
                if (iQ.inputType === 'timeframe'){
                    key = event.key
                }else{
                    key = event.key.toUpperCase()
                }
                iQ.inputString += key
            }else if (event.key == "Backspace") {
                iQ.inputString = iQ.inputString.slice(0,-1)
            }
            if (iQ.inputString !== "") { 
                if(/-/.test(iQ.inputString)){
                    iQ.inputType = "datetime"
                }
                else if(/^[0-9]$/.test(iQ.inputString[0])){
                    iQ.inputType = "timeframe"
                }else{
                    iQ.inputType =  "ticker"
                }
            }else{
                iQ.inputType = ""
            }
            if(iQ.inputType === "ticker") {
                privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:iQ.inputString})
                .then((result: Security[]) => {
                    inputQuery.update((v:InputQuery)=>{
                    const valid = validateInput(v.inputString,v.inputType)
                        v.inputValid = valid
                        v.securities = result
                        return v
                    })
                })
            }else{
               iQ.inputValid = validateInput(iQ.inputString,iQ.inputType) 
               iQ.securities = []
            }
        }
        inputQuery.set(iQ)
    }

    onMount(()=>{
        inputQuery.subscribe((v:InputQuery)=>{
            if (browser){
                if (v.status === "initializing"){
                    document.addEventListener('keydown',  handleKeyDown);
                    v.status = "active"
                    return v
                }else if(v.status === "inactive"){
                    document.removeEventListener('keydown',handleKeyDown);
                }
            }
        })
    })


</script>

{#if $inputQuery.status === "active"}
    <div class="popup">
        <div class="content">
            {#if $inputQuery.instance && Object.keys($inputQuery.instance).length > 0}
                {#each Object.entries($inputQuery.instance) as [key, value]}
                    <div class="entry">
                        <span class={key === $inputQuery.inputType ? 'highlight' : ''}>{key}:</span>
                        <span class={validateInput(key === $inputQuery.inputType ? $inputQuery.inputString : value, key) ? 'normal' : 'red'}> 
                            {key === $inputQuery.inputType ? $inputQuery.inputString : value}
                        </span>
                    </div>
                {/each}
            {/if}

            <!--<div class="input-display {$inputQuery.inputValid ? 'normal' : 'red'}">{$inputQuery.inputString}</div>-->

            <div class="table-container">
                {#if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
                    <table>
                        <thead>
                            <tr>
                                <th>Ticker</th>
                                <th>Date</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each $inputQuery.securities as sec, i}
                                <tr on:click={() => {inputQuery.set(enterInput(get(inputQuery,i)))}}>
                                    <td>{sec.ticker}</td>
                                    <td>{sec.maxDate === null ? 'Current' : sec.maxDate}</td> 
                                </tr>
                            {/each}
                        </tbody>
                    </table>
                {/if}
            </div>
        </div>
    </div>
{/if}

<style>
    @import "$lib/core/colors.css";

    .popup {
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        background-color: var(--c2);
        border: 1px solid var(--c4);
        z-index: 1000;
        padding: 20px;
        width: 400px;
        height: 500px; /* Fixed height */
        box-shadow: 0px 0px 20px rgba(0, 0, 0, 0.7);
        color: var(--f1);
        overflow: hidden; /* Prevent content from overflowing */
    }

    .content {
        width: 100%;
        height: 100%;
        overflow: hidden;
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    .entry {
        display: flex;
        justify-content: space-between;
        width: 100%;
        padding: 10px 0;
        border-bottom: 1px solid var(--c4);
    }

    .key {
        color: var(--f2);
    }

    .red {
        color: var(--c5);
    }

    .normal {
        color: var(--f1);
    }

    .highlight {
        color: var(--c3);
        font-weight: bold;
    }

    .table-container {
        flex-grow: 1; /* Ensures the table can grow to fill available space */
        overflow-y: auto; /* Make the table scrollable */
        margin-top: 10px;
        border-top: 1px solid var(--c4); /* Visual separation */
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
    }

    tr:hover {
        background-color: var(--c1);
    }

    .input-display {
        margin-top: 10px;
        font-size: 1.2em;
    }
</style>

