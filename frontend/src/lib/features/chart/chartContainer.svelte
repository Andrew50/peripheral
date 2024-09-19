<script context='module'>
    export let activeChartId = 0
</script>
<script lang='ts'>
    import Chart from './chart.svelte'
    let rows = 2
    let columns = 2



    </script>


<div class='top-container'>
{#each Array.from({length:rows}) as _, i}
    <div class='column'>
    {#each Array.from({length:columns}) as _,j}
        <Chart chartId={j + i * columns}/>
    {/each}
    </div>
{/each}
</div>
