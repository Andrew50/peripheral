<script lang="ts">
    let ticker = '';
    let datetime = '';
    import {privateRequest} from '../../store';

    function getRelatedTickers(): void {
        privateRequest<string[]>("getRelatedTickers",{ticker:ticker})
        .then((result) => { relatedTickers = result});
    }
</script>

<div>
<input bind:value={ticker} default='ticker'/>
</div>
<div>
<input bind:value={datetime} default='datetime'/>
</div>
<div>
<button on:click={getRelatedTickers}> get related tickers </button>
</div>
{#each ticker as relatedTickers}
<div>
{ticker}
</div>
{/each}
