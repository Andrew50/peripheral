<script lang='ts'>
    import '$lib/core/global.css'
    import ChartContainer from "$lib/features/chart/chartContainer.svelte"
    import RightClick from '$lib/utils/popups/rightClick.svelte';
    import Setup from '$lib/utils/popups/setup.svelte';
    import Input from '$lib/utils/popups/input.svelte';
    import Settings from "$lib/features/settings.svelte"
    import Journal from "$lib/features/journal.svelte"
    import Similar from '$lib/utils/popups/similar.svelte';
    import Study from '$lib/features/study.svelte';
    import Setups from '$lib/features/setups/setups.svelte';
    import Screen from '$lib/features/screen.svelte';
    import Watchlist from '$lib/features/watchlist.svelte'
    import Screensaver from '$lib/features/screensaver.svelte'
    import Quotes from '$lib/features/quotes/quotes.svelte'
    import Replay from '$lib/features/replay.svelte';
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/core/backend';
    import { goto } from '$app/navigation';
    import { get } from 'svelte/store';
    import { browser } from '$app/environment';
    import {initStores} from '$lib/core/stores'
    import { dispatchMenuChange,streamInfo, formatTimestamp } from '$lib/core/stores';
    type Menu = 'study' | 'screen' |'quotes'| 'setups' | 'test' | 'none' | 'watchlist' | "journal"|'screensaver' | "replay" | "settings";
    const menus: Menu[] = ['quotes','watchlist' ,'screen' ,'study' ,"journal", 'setups' ,'screensaver' , "replay", "settings"] //,'test'
    let active_menu: Menu = 'none';
    let minWidth: number;
    let maxWidth: number;
    let close: number;
    let pix: number;
    import {menuWidth} from "$lib/core/stores"
    let buttonWidth: number;
    onMount(() => { 
        dispatchMenuChange.subscribe((v:Menu)=>{
            toggleMenu(v)
        })
        privateRequest<string>("verifyAuth", {}).catch(() => {
            goto('/login');
        });
        initStores()
        if (browser) {
            function handleResize() {
                pix = window.innerWidth;
            }
            window.addEventListener('resize', handleResize);
            handleResize();
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

    function toggleMenu(menuName: Menu) {
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
        document.addEventListener('touchmove', resize);
        document.addEventListener('touchend', stopResize);
    }

    function resize(event: MouseEvent | TouchEvent) {
        if (resizing) {
            let clientX = 0
            if (event instanceof MouseEvent) {
                clientX = event.clientX;
            } else if (event instanceof TouchEvent) {
                clientX = event.touches[0].clientX;
            }
            let width = window.innerWidth - clientX - buttonWidth;
            if (width > maxWidth) {
                width = maxWidth;
            } else if (width < minWidth) {
                width = minWidth;
            }
            menuWidth.set(width);
        }
    }

    function stopResize(event: MouseEvent|TouchEvent) {
        if (resizing) {
            document.removeEventListener('mousemove', resize);
            document.removeEventListener('mouseup', stopResize);
            document.removeEventListener('touchmove', resize);
            document.removeEventListener('touchend', stopResize);
            resizing = false;
        }
    }
</script>

<div class="page">
    <Input/>
    <RightClick/>
    <Similar/>
    <Setup/>
    <div class="dcontainer">
        <!--<Chart width={pix - $menuWidth - buttonWidth}/>-->
       <ChartContainer width={pix - $menuWidth - buttonWidth}/>
        <div
            on:mousedown={startResize}
            on:touchstart={startResize}
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
            {:else if active_menu === "quotes"}
                <Quotes/>
            {/if}

        </div>
        <div class="system-clock">
            <h3>{$streamInfo.timestamp ? formatTimestamp($streamInfo.timestamp) : 'Loading Time...'}</h3>
        </div>
    </div>
    <div class="button-container">
        {#each menus as menu}

        <button
            class="button {active_menu == menu ? 'active' : ''}"
            on:click={() => toggleMenu(menu)}
        >
            <img class="icon" src={`${menu}.png`} alt="" />
        </button>
        {/each}
    </div>
</div>

<style>
    .dcontainer {
        /*position: relative;*/
        width: 100%;
        height: 100%;
        display: flex;
        flex-direction: row;

    }
    .system-clock {
        position: absolute;
        bottom: 5px; /* Adjust this to move it up a bit from the bottom */
        left: 5px; /* Adjust to align with the menu's width */
        z-index: 4;
        background-color: var(--c2);
        padding: 0px 10px;
        font-size: 1rem;
        color: var(--f1);
        /*box-shadow: 0 2px 5px rgba(0, 0, 0, 0.3);*/
    }

    .menu-container {
        max-width: 100%;
        box-sizing: border-box;
        position: absolute;
        box-sizing: border-box;
        top: 0;
        right: 0;
        height: 100%;
        width: 100%;
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
        padding: 0px;
        margin: 0px;
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

