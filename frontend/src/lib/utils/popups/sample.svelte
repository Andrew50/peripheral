<!-- sample.svelte -->
<script lang="ts" context="module">
	import { writable, get } from 'svelte/store';
	import type { Writable } from 'svelte/store';
	import type { Setup, Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
	import { setups } from '$lib/core/stores';
	import { setSample } from '$lib/features/setups/interface';
	interface UserSetupMenu {
		x: number;
		y: number;
		instance: Instance;
		status: 'active' | 'inactive';
		setup: Setup | null;
	}

	const inactiveUserSetupMenu = { x: 0, y: 0, instance: {}, status: 'inactive' } as UserSetupMenu;
	let userSetupMenu: Writable<UserSetupMenu> = writable(inactiveUserSetupMenu);
	export async function querySetup(event: MouseEvent, instance: Instance): Promise<Setup | 'new'> {
		const menuState: UserSetupMenu = {
			x: event.clientX,
			y: event.clientY,
			instance: instance,
			status: 'active',
			setup: null
		};
		userSetupMenu.set(menuState);
		return new Promise<Setup | 'new'>((resolve, reject) => {
			const unsubscribe = userSetupMenu.subscribe((iQ: User) => {
				iQ;
				if (iQ.status === 'cancelled') {
					deactivate();
					tick();
					reject();
				} else if (iQ.status === 'complete') {
					const re = iQ.instance;
					deactivate();
					resolve(re);
				}
			});
			function deactivate() {
				unsubscribe();
				inputQuery.update((v: InputQuery) => {
					v.status = 'shutdown';
					return v;
				});
			}
		});
	}
</script>

<script lang="ts">
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';
	let menu: HTMLElement;
	let startX: number;
	let startY: number;
	let initialX: number;
	let initialY: number;
	let isDragging = false;
	onMount(() => {
		userSetupMenu.subscribe((menuState) => {
			if (browser) {
				if (menuState.status === 'active') {
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
		event.target;
		menu;
		if (menu && !menu.contains(event.target as Node)) {
			('close');
			closeMenu();
		}
	}
	function handleKeyDown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			closeMenu();
		}
	}

	function closeMenu(setup: Setup | null = null): void {
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

{#if $userSetupMenu.status === 'active'}
	<div
		class="popup-container"
		bind:this={menu}
		style="top: {$userSetupMenu.y}px; left: {$userSetupMenu.x}px;"
	>
		<div class="content-container">
			<table>
				<thead>
					<tr class="defalt-tr">
						<th class="defalt-th">Setup</th>
					</tr>
				</thead>
				<tbody>
					{#if $setups && $setups.length > 0}
						{#each $setups as setup}
							<tr class="defalt-tr">
								<td
									on:click={() => {
										closeMenu(setup);
									}}
								>
									{setup.name}
								</td>
							</tr>
						{/each}
					{/if}
					<tr class="defalt-tr">
						<td
							on:click={() => {
								closeMenu('new');
							}}
						>
							new
						</td>
					</tr>
				</tbody>
			</table>
		</div>
	</div>
{/if}

<style>
	.popup-container {
		width: 220px;
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
		position: fixed;
		z-index: 1000;
		padding: 4px;
	}

	.content-container {
		padding: 8px;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	.defalt-tr {
		border-bottom: 1px solid var(--ui-border);
	}

	.defalt-tr:last-child {
		border-bottom: none;
	}

	.defalt-th {
		text-align: left;
		padding: 8px 12px;
		color: var(--text-secondary);
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
	}

	td {
		padding: 8px 12px;
		color: var(--text-primary);
		font-size: 14px;
		cursor: pointer;
		border-radius: 4px;
		transition: background-color 0.2s ease;
	}

	td:hover {
		background: var(--ui-bg-hover);
	}
</style>
