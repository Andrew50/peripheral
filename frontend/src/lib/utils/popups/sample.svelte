<!-- sample.svelte -->
<script lang="ts" context="module">
    import { writable,get } from 'svelte/store';
    import type { Writable } from 'svelte/store';
    import type { Instance } from '$lib/core/types';
    import { privateRequest } from '$lib/core/backend';
    import {setups} from '$lib/core/stores'
    import {setSample} from '$lib/features/setups/interface'
    interface UserSetupMenu {
        x: number;
        y: number;
        instance: Instance
        status: "active" | "inactive";
    }

    const inactiveUserSetupMenu = { x: 0, y: 0, instance: {}, status: "inactive" } as UserSetupMenu;
    let userSetupMenu: Writable<UserSetupMenu> = writable(inactiveUserSetupMenu);
    export function querySetSample(event: MouseEvent, instance: Instance): void {
        const menuState: UserSetupMenu = {
            x: event.clientX,
            y: event.clientY,
            instance: instance,
            status: "active"
        };
        userSetupMenu.set(menuState);
    }
</script>

<script lang="ts">
    import { onMount } from 'svelte';
    import {browser} from '$app/environment'
    let menu: HTMLElement;
    let startX: number;
    let startY: number;
    let initialX: number;
    let initialY: number;
    let isDragging = false;
    onMount(() => {
        userSetupMenu.subscribe((menuState) => {
            if (browser){
            if (menuState.status === "active") {
                setTimeout(() => {
                        document.addEventListener('click', handleClickOutside);
                        document.addEventListener('keydown', handleKeyDown);
                        document.addEventListener('mousedown', handleMouseDown);
                    }, 0);
            } else {
                document.removeEventListener('click', handleClickOutside);
                document.removeEventListener('keydown', handleKeyDown);
                document.removeEventListener('mousedown', handleMouseDown);
            }
            }
        });
    });
    function handleClickOutside(event: MouseEvent): void {
        console.log(event.target)
        console.log(menu)
        if (menu && !menu.contains(event.target as Node)) {
            console.log("close")
            closeMenu();
        }
    }
    function handleKeyDown(event: KeyboardEvent): void {
        if (event.key === 'Escape') {
            closeMenu();
        }
    }

    function closeMenu(): void {
        userSetupMenu.set(inactiveUserSetupMenu);
    }

    function handleMouseDown(event: MouseEvent): void {
        if (event.target && (event.target as HTMLElement).classList.contains('context-menu')) {
            startX = event.clientX;
            startY = event.clientY;
            initialX = $userSetupMenu.x;
            initialY = $userSetupMenu.y;
            isDragging = true;
            document.addEventListener('mousemove', handleMouseMove);
            document.addEventListener('mouseup', handleMouseUp);
        }
    }

    function handleMouseMove(event: MouseEvent): void {
        if (isDragging) {
            const deltaX = event.clientX - startX;
            const deltaY = event.clientY - startY;
            userSetupMenu.update((menuState) => {
                return { ...menuState, x: initialX + deltaX, y: initialY + deltaY };
            });
        }
    }

    function handleMouseUp(): void {
        isDragging = false;
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
    }
</script>

{#if $userSetupMenu.status === "active"}
    <div class="popup-container" bind:this={menu} style="top: {$userSetupMenu.y}px; left: {$userSetupMenu.x}px;">
        <div class="content-container">
        <table>
            <thead>
                <tr>
                    <th>Setup</th>
                </tr>
            </thead>
            <tbody>
            {#if $setups && $setups.length > 0}
                {#each $setups as setup}
                <tr>
                    <td on:click={() => {setSample(setup.setupId,$userSetupMenu.instance);closeMenu()}}>
                        {setup.name}
                    </td>
                    </tr>
                {/each}
            {/if}
            <tr>
                <td on:click={() => {setSample("new",$userSetupMenu.instance);closeMenu()}}>
                new
                </td>
                </tr>
            </tbody>
        </table>
        </div>
    </div>
{/if}
