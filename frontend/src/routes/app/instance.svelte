<!-- instance.svlete -->
<script lang="ts" context="module">
    import { privateRequest} from '../../store';
    import { onMount, tick } from 'svelte';
    import { get, writable } from 'svelte/store';
    import type { Writable } from 'svelte/store';
    import type {Instance } from '../../store';
    export interface RightClickInstance extends Instance {
        x: number;
        y: number;
    }
    export let rightClickInstance: Writable<RightClickInstance | null> = writable(null);
    export let inputBind: Writable<Writable<Instance> | null> = writable(null);
</script>
<script lang="ts">
    let inputString = "";
    let inputType: string = "";
    let selectedSecurityIndex = 0;
    let prevFocus: HTMLElement | null = null;
    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string | null;
        name: string;
    }
    let securities: Security[] = [];

    inputBind.subscribe((v:Writable<Instance | null> | null) => {
        console.log('god')
        if ( v != null && typeof window !== 'undefined'){
            const element = document.getElementById("instanceInput");
            if (element){
                prevFocus = document.activeElement as HTMLElement;
                element.focus();
            }
        }
    })
    function closePopup() {
        inputBind.set(null)
        securities = [];
        inputString = "";
        inputType = "";
        if (prevFocus){
            prevFocus.focus()
        }
    }

    function enterInput(index: number = 0):void{
        if (inputType === "ticker"){
            if (Array.isArray(securities) && securities.length > 0) {
                get(inputBind)?.update((instance: Instance) => {
                    instance.securityId = securities[index].securityId
                    instance.ticker = securities[index].ticker
                    return instance
                })
            }
        }else if (inputType === 'interval'){
            get(inputBind)?.update((instance: Instance) => {
                instance.timeframe = inputString
                return instance
            })
        }else if (inputType === 'datetime'){
            get(inputBind)?.update((instance: Instance) => {
                instance.datetime = inputString
                return instance
            })
        }
        closePopup();

    }

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

    onMount(() => {
        document.addEventListener('keydown',  (event) => {
            if (event.key === 'Escape') {
                closePopup();
            }else if (event.key === 'Enter') {
                enterInput(0)
            }else {
                if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                    inputString += event.key.toUpperCase();
                }else if (/[-:]/.test(event.key)){ //for datetime
                    inputString += event.key.toUpperCase();
                }else if (event.key == "Space" && inputType === 'datetime') {
                    inputString += event.key;
                }else if (event.key == "Backspace") {
                    inputString = inputString.slice(0, -1);
                }
                inputType = classifyInput(inputString)
                if(inputType === "ticker") {
                    privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:inputString})
                    .then((result: Security[]) => securities = result)
                }
            }
        });
    });


    //    document.addEventListener('click', closeRightClickMenu)


</script>
{#if $inputBind !== null}
    <div class="popup">
    <div>{inputString}</div>
    <div>{inputType}</div>
        <table>
            {#if Array.isArray(securities) && securities.length > 0}
            <th> Ticker <th/>
            <th> Delist Date </th>
            {#each securities as sec, i}
                <tr class={selectedSecurityIndex === i ? 'selected' : ''} on:click={() => enterInput(i)}> 
                    <td>{sec.ticker}</td>
                    <td>{sec.maxDate}</td> 
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
            //    newStudy($rightClickInstance)
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

