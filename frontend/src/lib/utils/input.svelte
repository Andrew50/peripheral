<!-- instance.svlete -->
<script lang="ts" context="module">
    import '$lib/core/global.css'

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

        status: "inactive" | "initializing" | "active" | "complete" | "cancelled" | "shutdown"
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
        await tick()
        if (get(inputQuery).status === "inactive"){
            inputQuery.update((v:InputQuery)=>{
                v.requiredKeys = requiredKeys
                v.instance = instance //instance must be set up to have required fields as blank
                v.status = "initializing"
                return v
            })
            return new Promise<Instance>((resolve, reject) => {
                const unsubscribe = inputQuery.subscribe((iQ: InputQuery) => {
                    console.log(iQ)
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
                    inputQuery.update((v:InputQuery)=>{
                        v.status = "shutdown"
                        return v
                        })
                }
            })
        }else{
            return Promise.reject(new Error("input query already active"))
        }

    }
        
</script>
<script lang="ts">
    import {browser} from '$app/environment'
    import {onDestroy,onMount} from 'svelte'
	import { ESTStringToUTCTimestamp, UTCTimestampToESTString } from '$lib/core/timestamp';
    let prevFocusedElement: HTMLElement | null;

    interface ValidateResponse {
        inputValid: boolean
        securities: Security[]
    }

    async function validateInput(inputString: string, inputType: string):Promise<ValidateResponse>{ //auto wraps sync returns in Promise.resolve()
        if (inputType === "ticker"){
            const securities = await privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:inputString})
            if (Array.isArray(securities) && securities.length > 0){
                return {inputValid: securities.some((v:Security)=>v.ticker === inputString), securities: securities}
            }else{
                return {inputValid: false, securities: []}
            }
        }else if(inputType == "timeframe"){
            const regex = /^\d{1,3}[yqmwhds]?$/i;
            return {inputValid:regex.test(inputString),securities:[]}
        }else if(inputType == "timestamp"){
            const formats = ["yyyy-MM-dd H:m:ss","yyyy-MM-dd H:m","yyyy-MM-dd H","yyyy-MM-dd",];
            for (const format of formats) {
                try {
                    const parsedDate = parse(inputString, format, new Date())
                    if (parsedDate != "Invalid Date"){
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
            iQ.instance.timestamp = ESTStringToUTCTimestamp(iQ.inputString)
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
            inputQuery.set(iQ)
        }else if (event.key === 'Enter') {
            event.preventDefault()
            if (iQ.inputValid) {
                iQ = enterInput(iQ,0)
            }
            inputQuery.set(iQ)
        }else if(event.key == "Tab"){
            event.preventDefault()
            iQ.instance.extendedHours = ! iQ.instance.extendedHours
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
        inputQuery.subscribe(async(v:InputQuery)=>{
            if (browser){
                if (v.status === "initializing"){
                    await tick()
                    document.addEventListener('keydown',  handleKeyDown);
                    prevFocusedElement = document.activeElement as HTMLElement;
                    const inputWindow = document.getElementById("input-window")
                    inputWindow.focus()
                    v.status = "active"
                    //await tick()
                }else if(v.status === "shutdown"){
                    prevFocusedElement?.focus()
                    document.removeEventListener('keydown',handleKeyDown);
                    v.status = "inactive"
                    v.inputString = ""
                }
            }
        })
    })
    onDestroy(()=>{
        try {
            document.removeEventListener('keydown',handleKeyDown);
        }catch{}
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

{#if $inputQuery.status === "active" || $inputQuery.status === "initializing"}
    <div class="popup-container" id="input-window" tabindex="-1">
        <div class="content-container">
            {#if $inputQuery.instance && Object.keys($inputQuery.instance).length > 0}
                {#each possibleDisplayKeys as key}
                    <div class="span-container">
                        <span class={$inputQuery.requiredKeys.includes(key) && !$inputQuery.instance[key] ? 'red' : ''}>{key}</span>
                        <span class={key === $inputQuery.inputType ? $inputQuery.inputValid ? 'blue' : 'red' : ''}> 
                        {displayValue($inputQuery,key)} </span>
                    </div>
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
    .popup-container {
        width: 400px;
        height: 500px;
    }
</style>

