<script lang="ts">
    let ticker = '';
    let relatedTickers: string[] = [];
    import {privateRequest} from '$lib/api/backend';

    function getRelatedTickers(): void {
        privateRequest<string[]>("getRelatedTickers",{ticker:ticker})
        .then((result) => { relatedTickers = result});
    }
</script>

<div>
<input bind:value={ticker} placeholder='ticker'/>
</div>
<div>
<button on:click={getRelatedTickers}> get related tickers </button>
</div>
{#each relatedTickers as ticker}
<div>
{ticker}
</div>
{/each}
