<!-- instance.svlete -->
<script lang="ts" context="module">
    import { privateRequest} from '$lib/core/backend';
    import { get, writable } from 'svelte/store';
    import { parse} from 'date-fns';
    import {tick} from 'svelte';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '$lib/core/types';
    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string | null;
        name: string;
    }
    const possibleDisplayKeys = ["ticker","timestamp","timeframe","extendedHours"]
    type InstanceAttributes = typeof possibleDisplayKeys[number];
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
    let inputQuery: Writable<InputQuery> = writable({...inactiveInputQuery})

    export async function queryInstanceInput(requiredKeys: InstanceAttributes[] | "any",instance:Instance={}): Promise<Instance> {
        //init the query with passsed info
        if (get(inputQuery).status === "inactive"){
            inputQuery.update((v:InputQuery)=>{
                v.requiredKeys = requiredKeys
                v.instance = instance //instance must be set up to have required fields as blank
                v.status = "initializing"
                return v
            })
            return new Promise<Instance>((resolve, reject) => {
                const unsubscribe = inputQuery.subscribe((iQ: InputQuery) => {
                    if (iQ.status === "cancelled"){
                        deactivate()
                        tick()
                        reject()
                    }else if(iQ.status === "complete"){
                        const re = iQ.instance
                        deactivate()
                        resolve(re)
                    }
                })
                function deactivate(){
                    unsubscribe()
                    inputQuery.set({...inactiveInputQuery})
                }
            })
        }else{
            return Promise.reject(new Error("input query already active"))
        }

    }
        
</script>
<script lang="ts">
    import {browser} from '$app/environment'
    import {onMount} from 'svelte'
	import { ESTStringToUTCTimestamp, UTCTimestampToESTString } from '$lib/core/timestamp';
    let prevFocusedElement: Element | null

    interface ValidateResponse {
        inputValid: boolean
        securities: Security[]
    }

    async function validateInput(inputString: string, inputType: string):Promise<ValidateResponse>{ //auto wraps sync returns in Promise.resolve()
        if (inputType === "ticker"){
            const securities = await privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:inputString})
            if (Array.isArray(securities) && securities.length > 0){
                console.log(securities)
                return {inputValid: securities.some((v:Security)=>v.ticker === inputString), securities: securities}
            }else{
                return {inputValid: false, securities: []}
            }
        }else if(inputType == "timeframe"){
            const regex = /^\d{1,3}[yqmwds]?$/i;
            return {inputValid:regex.test(inputString),securities:[]}
        }else if(inputType == "timestamp"){
            const formats = ["yyyy-MM-dd H:m:ss","yyyy-MM-dd H:m","yyyy-MM-dd H","yyyy-MM-dd",];
            for (const format of formats) {
                try {
                    const parsedDate = parse(inputString, format, new Date())
                    if (parsedDate != "Invalid Date"){
                        console.log('valid')
                    //if (isNaN(parsedDate.getTime())){
                        //return {inputValid: await privateRequest<boolean>("validateDateString",{dateString:inputString}),securities:[]}
                        return {inputValid:true,securities:[]}
                    }
                }catch{} }
            return {inputValid:false,securities:[]}
        }
        return {inputValid:false,securities:[]}
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
        }else if (iQ.inputType === 'timestamp'){
            console.log(iQ.inputString)
            iQ.instance.timestamp = ESTStringToUTCTimestamp(iQ.inputString)
            console.log("Testing", iQ.instance.timestamp)
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
        console.log(iQ.status)

        iQ.inputString = ""
        iQ.inputType = ""
        iQ.inputValid = true
        return iQ
    }
    function handleKeyDown(event:KeyboardEvent):void {
        event.stopPropagation();
        //event.preventDefault()
        let iQ = get(inputQuery)
        if (event.key === 'Escape') {
            iQ.status = "cancelled"
            inputQuery.set(iQ)
        }else if (event.key === 'Enter') {
            event.preventDefault()
            if (iQ.inputValid) {
                iQ = enterInput(iQ,0)
            }
            inputQuery.set(iQ)
        }else if(event.key == "Tab"){
            console.log("god")
            event.preventDefault()
            iQ.instance.extendedHours = ! iQ.instance.extendedHours
            console.log(iQ)
            inputQuery.set({...iQ})
        }else {
            if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase()) 
                || (/[-:]/.test(event.key)) 
                || (event.key == " " && iQ.inputType === 'timestamp') ) {
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
            //classify input
            if (iQ.inputString !== "") { 
                if (/^[A-Z]$/.test(iQ.inputString)) {
                    iQ.inputType = "ticker";
                }else if (/^\d{1,2}(?:[hdwmqs])?$/.test(iQ.inputString)) {
                    iQ.inputType = "timeframe";
                    iQ.securities = [];
                } else if (/^\d{3}?.*$/.test(iQ.inputString)) {
                    iQ.inputType = "timestamp";
                    iQ.securities = [];
                } else { //assume its 
                    iQ.inputType = "ticker"
                }
            }else{
                iQ.inputType = ""
            }
            validateInput(iQ.inputString,iQ.inputType).then((validateResponse:ValidateResponse)=>{
                inputQuery.update((v:InputQuery)=>{
                    return {
                        ...iQ,
                        ...validateResponse
                    }

                })
           })
        }
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
        /*return () => {
        document.removeEventListener('keydown', handleKeyDown);
    };*/
    })

    function displayValue(q:InputQuery,key:string):string{
        if (key === q.inputType){
            return q.inputString
        }else if(q.instance[key] !== undefined){
            if (key === 'timestamp'){
                return UTCTimestampToESTString(q.instance.timestamp)
            }else if (key === 'extendedHours'){
                return q.instance.extendedHours ? "True" : "False"
            }else{
                return q.instance[key]
            }
        }
        return ""
    }
    /*    key === $inputQuery.inputType ? $inputQuery.inputString : ($inputQuery.instance[key] ? (key === "timestamp" ? UTCTimestampToESTString($inputQuery.instance[key]):$inputQuery.instance[key] ) : ""))}
    }*/


</script>

{#if $inputQuery.status === "active"}
    <div class="popup">
        <div class="content">
            {#if $inputQuery.instance && Object.keys($inputQuery.instance).length > 0}
                <!--{#each Object.entries($inputQuery.instance) as [key, value]}-->
                {#each possibleDisplayKeys as key}
                    <!--{#if possibleDisplayKeys.includes(key)}-->
                        <div class="entry">
                            <span class={$inputQuery.requiredKeys.includes(key) && !$inputQuery.instance[key] ? 'red' : 'normal'}>{key}</span>
                            <span class={key === $inputQuery.inputType ? $inputQuery.inputValid ? 'highlight' : 'red' : 'normal'}> 
                            {displayValue($inputQuery,key)} </span>
                        </div>
                    <!--{/if}-->
                {/each}
            {/if}
            <div class="table-container">
                {#if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
                    <table>
                        <thead>
                            <tr>
                                <th>Ticker</th>
                                <th>Delist Date</th>
                            </tr>
                        </thead>
                        <tbody>
                            {#each $inputQuery.securities as sec, i}
                                <tr on:click={() => {inputQuery.set(enterInput(get(inputQuery),i))}}>
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
        font-weight: bold;
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

