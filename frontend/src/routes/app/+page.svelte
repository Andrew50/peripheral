<script lang='ts'>
    import Chart from '$lib/features/chart/chart.svelte';
    import ChartContainer from "$lib/features/chart/chartContainer.svelte"
    import RightClick from '$lib/utils/rightClick.svelte';
    import Input from '$lib/utils/input.svelte';
    import Settings from "$lib/features/settings.svelte"
    import Journal from "$lib/features/journal.svelte"
    import Similar from '$lib/utils/similar.svelte';
    import Study from '$lib/features/study.svelte';
    import Setups from '$lib/features/setups.svelte';
    import Screen from '$lib/features/screen.svelte';
    import Test from '$lib/features/test.svelte';
    import Watchlist from '$lib/features/watchlist.svelte'
    import Screensaver from '$lib/features/screensaver.svelte'
    import Replay from '$lib/features/replay.svelte';
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/core/backend';
    import { goto } from '$app/navigation';
    import { get, writable } from 'svelte/store';
    import { browser } from '$app/environment';
    import {initStores} from '$lib/core/stores'
    import { currentTimestamp, formatTimestamp, updateTime } from '$lib/core/stores';
    type Menu = 'study' | 'screen' | 'setups' | 'test' | 'none' | 'watchlist' | "journal"|'screensaver' | "replay" | "settings";
    const menus: Menu[] = ['watchlist' ,'screen' ,'study' ,"journal", 'setups' ,'screensaver' , "replay", "settings"] //,'test'
    let active_menu: Menu = 'none';
    let minWidth: number;
    let maxWidth: number;
    let close: number;
    let pix: number;
    let menuWidth = writable(0);
    let buttonWidth: number;
    let interval;
    onMount(() => { 
        privateRequest<string>("verifyAuth", {}).catch(() => {
            goto('/login');
        });
        initStores()
        if (browser) {
            function handleResize() {
                pix = window.innerWidth;
            }

            window.addEventListener('resize', handleResize);

            // Set initial value
            handleResize();
            interval = setInterval(updateTime, 1000);
            return () => {
                window.removeEventListener('resize', handleResize);
            };
        }
    });

    $: if (browser) {
        pix = window.innerWidth;
        minWidth = pix * 0.2;
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
        event.preventDefault();  
        resizing = true;
        document.addEventListener('mousemove', resize);
        document.addEventListener('mouseup', stopResize);
    }

    function resize(event: MouseEvent) {
        if (resizing) {
            let width = window.innerWidth - event.clientX - buttonWidth;
            if (width > maxWidth) {
                width = maxWidth;
            } else if (width < minWidth) {
                width = minWidth;
            }
            menuWidth.set(width);
        }
    }

    function stopResize(event: MouseEvent) {
        if (resizing) {
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
        <!--<Chart width={pix - $menuWidth - buttonWidth}/>-->
       <ChartContainer width={pix - $menuWidth - buttonWidth}/>
        <div
            on:mousedown={startResize}
            class="resize-handle"
            style="right: {$menuWidth + buttonWidth}px"
        ></div>
        <div
            class="menu-container"
            style="width: {$menuWidth}px; right: {buttonWidth}px"
        >
            
            {#if active_menu === 'study'}
                <Study/>
            {:else if active_menu === "setups"}
                <Setups/>
            {:else if active_menu === 'screen'}
                <Screen/>
            <!--{:else if active_menu === 'test'}
                <Test/>-->
            {:else if active_menu === 'watchlist'}
                <Watchlist/>
            {:else if active_menu === "journal"}
                <Journal/>
            {:else if active_menu === "screensaver"}
                <Screensaver/>
            {:else if active_menu === 'replay'}
                <Replay/>
            {:else if active_menu === "settings"}
                <Settings/>
            {/if}

        </div>
    </div>
    <div class="button-container">
        {#each menus as menu}

        <button
            class="button {active_menu == menu ? 'active' : ''}"
            on:click={() => toggle_menu(menu)}
        >
            <img class="icon" src={`${menu}.png`} alt="" />
        </button>
        {/each}
        <h3>{$currentTimestamp ? formatTimestamp($currentTimestamp) : 'Loading Time...'}</h3>
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
        flex-direction: row;

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
        overflow-y:scroll;
        overflow-x:auto;
        display: flex;
        flex-direction: column;
        padding: 10px;
        padding-left: 0px;
        /*scrollbar-color: var(--c4) var(--c1);
        scrollbar-width: thin;*/
        scrollbar-width: none;

    }
    .resize-handle {
        position: absolute;
        top: 0;
        width: 10px;
        height: 100%;
        cursor: ew-resize;
        background-color: var(--c2);
        z-index: 2;
    }
    .resize-handle:hover {
        background-color: var(--c2);
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

