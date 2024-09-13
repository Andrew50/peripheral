<script lang='ts'>
    import Chart from '$lib/features/chart/chart.svelte';
    import RightClick from '$lib/utils/rightClick.svelte';
    import Input from '$lib/utils/input.svelte';
    import Similar from '$lib/utils/similar.svelte';
    import Study from '$lib/features/study.svelte';
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/core/backend';
    import { goto } from '$app/navigation';
    import { get, writable } from 'svelte/store';
    import { browser } from '$app/environment';

    type Menu = 'study' | 'screener' | 'setups' | 'none';
    let active_menu: Menu = 'none';
    let minWidth: number;
    let maxWidth: number;
    let close: number;
    let pix: number;
    let menuWidth = writable(0);
    let buttonWidth: number;

    onMount(() => {
        privateRequest<string>("verifyAuth", {}).catch(() => {
            goto('/login');
        });
        if (browser) {
            function handleResize() {
                pix = window.innerWidth;
            }

            window.addEventListener('resize', handleResize);

            // Set initial value
            handleResize();

            return () => {
                window.removeEventListener('resize', handleResize);
            };
        }
    });

    $: if (browser) {
        pix = window.innerWidth;
        minWidth = pix * 0.15;
        maxWidth = pix * 0.7;
        buttonWidth = pix * 0.04;
        close = 0;
    }

    function toggle_menu(menuName: Menu) {
        if (active_menu == menuName) {
            active_menu = 'none'; 
            menuWidth.set(close);
        } else {
            active_menu = menuName;
            if (get(menuWidth) < minWidth) {
                menuWidth.set(minWidth);
            }
        }
    }

    let resizing = false;

    function startResize(event: MouseEvent) {
        console.log('starting resize')
        event.preventDefault();  
        resizing = true;
        document.addEventListener('mousemove', resize);
        document.addEventListener('mouseup', stopResize);
    }

    function resize(event: MouseEvent) {
        console.log('god')
        if (resizing) {
            let width = window.innerWidth - event.clientX - buttonWidth;
            if (width > maxWidth) {
                width = maxWidth;
            } else if (width < minWidth) {
                width = minWidth;
            }
            console.log(width)
            menuWidth.set(width);
        }
    }

    function stopResize(event: MouseEvent) {
        if (resizing) {
            console.log('stop resize')
            document.removeEventListener('mousemove', resize);
            document.removeEventListener('mouseup', stopResize);
            resizing = false;
        }
    }
</script>

<div class="page">
    <Input/>
    <RightClick/>
    <Similar/>
    <div class="container">
        <Chart width = {pix - $menuWidth - buttonWidth}/>
        <div
            on:mousedown={startResize}
            class="resize-handle"
            style="right: {$menuWidth + buttonWidth}px"
        ></div>
        <div
            class="menu-container"
            style="width: {$menuWidth}px; right: {buttonWidth}px"
        >
            {#if active_menu == 'study'}
                <Study/>
            {/if}
        </div>
    </div>
    <div class="button-container">
        <button
            class="button {active_menu == 'study' ? 'active' : ''}"
            on:click={() => toggle_menu('study')}
        >
            <img class="icon" src="/study.png" alt="" />
        </button>
        <button
            class="button {active_menu == 'screener' ? 'active' : ''}"
            on:click={() => toggle_menu('screener')}
        >
            <img class="icon" src="/screener.png" alt="" />
        </button>
        <button
            class="button {active_menu == 'setups' ? 'active' : ''}"
            on:click={() => toggle_menu('setups')}
        >
            <img class="icon" src="/setups.png" alt="" />
        </button>
    </div>
</div>

<style>
    @import "$lib/core/colors.css";
    @import "$lib/core/components.css";
    .chart {
        flex: 1;
        height: 100%;
    }
    .container {
        /*position: relative;*/
        width: 100%;
        height: 100%;
        display: flex;

    }
    .chart {
        width: 100%;
        height: 100%;
    }
    .menu-container {
        max-width: 100%;
        box-sizing: border-box;
        position: absolute;
        box-sizing: border-box;
        top: 0;
        right: 0;
        height: 100%;
        background-color: var(--c2);
        z-index: 1;
        overflow-x: hidden;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
    }
    .resize-handle {
        position: absolute;
        top: 0;
        width: 5px;
        height: 100%;
        cursor: ew-resize;
        background-color: var(--c2);
        z-index: 2;
    }
    .resize-handle:hover {
        background-color: var(--c4);
    }
    .button-container {
        align-items: center;
        width: 4vw;
        height: 100%;
        top: 0;
        right: 0;
        z-index: 3;
        background-color: var(--c1);
        flex-direction: column;
        justify-content: start;
        position: absolute;
    }
    .button.active {
        border-left-color: transparent;
        background-color: var(--c2);
    }
    .button {
        width: 4vw;
        height: 4vw;
        background-color: var(--c1);
        border: none; 
        padding: 0; 
        cursor: pointer;
        border-radius: 0; 
        font-size: 1.5vw;
        transition: background-color 0.1s;
        display: flex;
        align-items: center;
        justify-content: center;
    }
    .button:hover {
        background-color: var(--c2); 
    }
    .icon {
        width: 60%;
        height: 60%;
    }
</style>

