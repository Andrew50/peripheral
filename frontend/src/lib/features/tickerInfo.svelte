<script lang="ts">
    import { onMount } from 'svelte';
    import { tickerInfoState } from '$lib/core/stores';
    let startY = 0;
    let isDragging = false;
    let container: HTMLDivElement;

    function handleMouseDown(e: MouseEvent) {
        if (e.target instanceof HTMLButtonElement) return;
        isDragging = true;
        startY = e.clientY;
        document.body.style.cursor = 'ns-resize';
        document.body.style.userSelect = 'none';
    }

    function handleMouseMove(e: MouseEvent) {
        if (!isDragging) return;
        const deltaY = startY - e.clientY;
        startY = e.clientY;
        
        tickerInfoState.update(state => ({
            ...state,
            currentHeight: Math.min(Math.max(state.currentHeight + deltaY, 50), 400)
        }));
    }

    function handleMouseUp() {
        isDragging = false;
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
    }

    function toggleExpand() {
        tickerInfoState.update(state => ({
            ...state,
            isExpanded: !state.isExpanded,
            currentHeight: !state.isExpanded ? state.currentHeight : 200
        }));
    }

    onMount(() => {
        document.addEventListener('mousemove', handleMouseMove);
        document.addEventListener('mouseup', handleMouseUp);

        return () => {
            document.removeEventListener('mousemove', handleMouseMove);
            document.removeEventListener('mouseup', handleMouseUp);
        };
    });
</script>

<div 
    class="ticker-info-container {$tickerInfoState.isExpanded ? 'expanded' : ''}" 
    style="height: {$tickerInfoState.isExpanded ? $tickerInfoState.currentHeight : '30'}px" 
    bind:this={container}
>
    <div 
        class="drag-handle" 
        on:mousedown={handleMouseDown}
        on:touchstart|preventDefault={handleMouseDown}
    >
        <button 
            class="expand-button" 
            on:click|stopPropagation={toggleExpand}
        >
            {$tickerInfoState.isExpanded ? '▼' : '▲'}
        </button>
        <span>Ticker Info</span>
    </div>
    
    {#if $tickerInfoState.isExpanded}
        <div class="content">
            <div class="info-row">
                <span class="label">Ticker:</span>
            </div>
            <div class="info-row">
                <span class="label">Market Cap:</span>
                <span class="value">$50.2B</span>
            </div>
            <div class="info-row">
                <span class="label">Float:</span>
                <span class="value">125.3M</span>
            </div>
            <div class="info-row">
                <span class="label">Short Float:</span>
                <span class="value">15.2%</span>
            </div>
            <div class="info-row">
                <span class="label">Industry:</span>
                <span class="value">Technology</span>
            </div>
        </div>
    {/if}
</div>

<style>
    .ticker-info-container {
        position: fixed;
        bottom: 0;
        width: 100%;
        background: #1e222d;
        border-top: 1px solid #363a45;
        overflow: hidden;
        z-index: 1000;
        will-change: height; /* Optimize for animations */
    }

    .ticker-info-container.expanded {
        transition: none; /* Remove transition when expanded for better drag response */
    }

    .ticker-info-container:not(.expanded) {
        transition: height 0.2s ease; /* Only animate when collapsing/expanding */
    }

    .drag-handle {
        width: 100%;
        height: 30px;
        background: #2a2e39;
        cursor: ns-resize;
        display: flex;
        align-items: center;
        padding: 0 10px;
        user-select: none;
        touch-action: none; /* Improve touch handling */
    }

    .expand-button {
        background: none;
        border: none;
        color: #fff;
        cursor: pointer;
        padding: 5px;
        margin-right: 10px;
        z-index: 2; /* Ensure button is clickable */
    }

    .expand-button:hover {
        background: rgba(255, 255, 255, 0.1);
    }

    .content {
        padding: 10px;
        overflow-y: auto;
        height: calc(100% - 30px);
    }

    .info-row {
        display: flex;
        justify-content: space-between;
        margin-bottom: 8px;
        color: #fff;
        font-size: 12px;
        padding: 4px 0;
    }

    .label {
        color: #8f95a3;
    }

    .value {
        font-family: monospace;
    }
</style> 