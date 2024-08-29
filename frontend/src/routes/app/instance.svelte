<script lang="ts">
    import { privateRequest } from '../../store'

    let ticker = "";
    interface Security {
        securityId: number;
        ticker: string;
        maxDate: string;
        name: string;
    }
    export let securityId: number;

    let securities: Security[] = []
    function getSecurities(){
        privateRequest<Security[]>("getSecuritiesFromTicker",{ticker:ticker})
        .then((result: Security[]) => securities = result)
        .catch((error: string) => {})
    }

    function rowClick(secId: number): void{
        console.log("selected id ", securityId)
        securityId = secId;
    }

</script>



<div>
<input placeholder="ticker" on:keyup={getSecurities} bind:value={ticker}>
</div>
<div>
<table>
    {#if Array.isArray(securities)}
    {#each securities as sec}
    <button on:click={() => rowClick(sec.securityId)}>
        <tr> 
            <td>{sec.ticker} </td>
            <td>{sec.maxDate} </td> 
            <td>{sec.name} </td>
        </tr>
    </button>
    {/each}
    {/if}
</table>
</div>
