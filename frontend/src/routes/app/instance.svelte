<!-- instance.svlete -->
<script lang="ts">
    import { privateRequest, chartQuery , instanceInputVisible} from '../../store';
    import { onMount, tick } from 'svelte';
    import { get } from 'svelte/store';
    import type {ChartQuery} from '../../store';
    let inputString = "";
    let inputType = "";
    let selectedSecurityIndex = 0;
    let prevFocus: HTMLElement | null = null;
    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string;
        name: string;
    }
    let securities: Security[] = [];
    instanceInputVisible.subscribe(async (v) => {
        if ( v && typeof window !== 'undefined'){
            await tick();
            const element = document.getElementById("instanceInput");
            if (element){
                prevFocus = document.activeElement as HTMLElement;
                element.focus();
            }
        }
    })


    function selectSecurity(index: number){
        chartQuery.update((v:ChartQuery) => {
            v.securityId =  (securities[index].securityId);
            return v
        })
        closePopup();
    }

    function closePopup() {
        instanceInputVisible.set(false)
        securities = [];
        inputString = "";
        console.log(inputString)
        inputType = "";
        if (prevFocus){
            prevFocus.focus()
        }
    }

    function classifyInput(input){
        if (input) {
            return /^[0-9]$/.test(input[0]) ? "interval" : "ticker";
        }else{
            return null;
        }
    }

    onMount(() => {
        document.addEventListener('keydown',  (event) => {
            if (get(instanceInputVisible)){
                if (event.key === 'Escape') {
                    closePopup();
                }else if (event.key === 'Enter') {
                    if (Array.isArray(securities) && securities.length > 0) {
                        selectSecurity(0);
                    }
                    closePopup();
                }else {
                    if (/^[a-zA-Z0-9]$/.test(event.key.toLowerCase())) {
                        inputString += event.key;
                        console.log(inputString)
                    }else if (event.key == "Backspace") {
                        inputString = inputString.slice(0, -1);
                    }
                    inputType = classifyInput(inputString)
                    if(inputType === "ticker") {
                        privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:inputString})
                        .then((result: Security[]) => securities = result)
                        .catch((error: string) => {});
                    }
                }
            }
        });
    });

</script>
{#if $instanceInputVisible}
    <div class="popup">
    <div>{inputString}</div>
    <div>{inputType}</div>
        <table>
            {#if Array.isArray(securities) && securities.length > 0}
            {#each securities as sec, i}
                <tr class={selectedSecurityIndex === i ? 'selected' : ''} on:click={() => selectSecurity(i)}> 
                    <td>{sec.ticker}</td>
                    <td>{sec.maxDate}</td> 
                    <td>{sec.name}</td>
                </tr>
            {/each}
            {/if}
        </table>
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

