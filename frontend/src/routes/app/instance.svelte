<script lang="ts">
    import { privateRequest } from '../../store';
    import { onMount } from 'svelte';
    import type { Writable } from 'svelte/store';

    let ticker = "";
    let isOpen = false;
    let selectedSecurityIndex = 0;

    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string;
        name: string;
    }
    export let securityId: Writable<number>;

    let securities: Security[] = [];
    
    function selectSecurity(index: number){
        securityId.set(securities[index].securityId);
        closePopup();
    }

    function getSecurities(event: KeyboardEvent){
        if (event.key === "Enter"){
            if (securities.length > 0) {
                selectSecurity(selectedSecurityIndex);
            }
        } else if (event.key === "Escape") {
            closePopup();
        } else {
            privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:ticker})
            .then((result: Security[]) => securities = result)
            .catch((error: string) => {});
        }
    }

    function openPopup() {
        isOpen = true;
    }

    function closePopup() {
        isOpen = false;
        securities = [];
    }

    onMount(() => {
        document.addEventListener('keydown', (event) => {
            if (event.key === 'Escape') {
                closePopup();
            } else if (event.key === 'Enter' && isOpen) {
                if (securities.length > 0) {
                    selectSecurity(selectedSecurityIndex);
                }
            }
        });
    });

</script>
{#if isOpen}
    <div class="popup">
        <table>
            {#if Array.isArray(securities)}
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
        display: block;
        position: absolute;
        background-color: white;
        border: 1px solid #ccc;
        z-index: 1000;
        padding: 10px;
        width: 300px;
    }
    .hidden {
        display: none;
    }
</style>

