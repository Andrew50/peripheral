<script>
    import Header from '$lib/utils/modules/header.svelte';
    import '$lib/core/global.css'
    import {browser} from '$app/environment'
    import MarketChart from './MarketChart.svelte';
    import { onMount } from 'svelte';
    
    if (browser){
        document.title = "Atlantis"
    }

    let showChart = true;  // Start with chart visible
    let showAtlantis = false;  // Start with Atlantis text hidden

    onMount(() => {
        // After 3 seconds (instead of 2), transition from chart to Atlantis
        setTimeout(() => {
            showChart = false;
            showAtlantis = true;
        }, 3000);  // Changed from 2000 to 3000 milliseconds
    });
</script>

<main class="main-container">
    <div class="chart-container">
        <MarketChart />
    </div>
    
    <div class="atlantis-container" class:fade-in={showAtlantis}>
      <Header />
        <h1>ATLANTIS</h1>
    </div>
</main>

<style>
    :global(body) {
        margin: 0;
        padding: 0;
        overflow: hidden;
        background: black;
    }

    :global(html) {
        margin: 0;
        padding: 0;
    }

    .main-container {
        position: relative;
        width: 100vw;
        height: 100vh;
        margin: 0;
        padding: 0;
    }

    .chart-container {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        opacity: 1;
        transition: opacity 1s ease-out;
    }

    .chart-container.fade-out {
        opacity: 0;
    }

    .atlantis-container {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        opacity: 0;
        transition: opacity 1s ease-in;
        background: black;
    }

    .atlantis-container.fade-in {
        opacity: 1;
    }

    .atlantis-container h1 {
        color: #3b82f6;
        font-family: monospace;
        font-size: 4rem;
        letter-spacing: 0.5rem;
        text-transform: uppercase;
    }
</style>
