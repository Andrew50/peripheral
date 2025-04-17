<!-- sample.svelte -->
<script lang="ts" context="module">
	import { writable } from 'svelte/store';
	import type { Writable } from 'svelte/store';
	import type { Setup } from '$lib/core/types';
	import { newSetup } from '$lib/features/strategies/interface';
	import { strategies } from '$lib/core/stores';
	interface UserSetupMenu {
		x: number;
		y: number;
		status: 'active' | 'inactive';
		strategy: Setup | null | 'new';
	}

	const inactiveUserSetupMenu = { x: 0, y: 0, status: 'inactive' } as UserSetupMenu;
	let userSetupMenu: Writable<UserSetupMenu> = writable(inactiveUserSetupMenu);

	const MARGIN = 20; // Margin from window edges in pixels

	// Initial position without height constraint
	function getInitialPosition(x: number, y: number): { x: number; y: number } {
		if (browser) {
			const windowWidth = window.innerWidth;
			const menuWidth = 220; // Match our CSS width
			return {
				x: Math.min(Math.max(MARGIN, x), windowWidth - menuWidth - MARGIN),
				y: Math.max(MARGIN, y)
			};
		}
		return { x, y };
	}

	export async function queryStrategy(event: MouseEvent): Promise<number> {
		const { x, y } = getInitialPosition(event.clientX, event.clientY);
		const menuState: UserSetupMenu = {
			x,
			y,
			status: 'active',
			strategy: null
		};
		userSetupMenu.set(menuState);
		('open');
		return new Promise<number>((resolve, reject) => {
			const unsubscribe = userSetupMenu.subscribe(async (menuState: UserSetupMenu) => {
				if (menuState.status === 'inactive') {
					menuState;
					if (menuState.strategy === 'new') {
						('TODO: implement new strategy functionality');
						unsubscribe();
						reject();
					} else if (menuState.strategy === null) {
						unsubscribe();
						reject();
					} else {
						unsubscribe();
						resolve(menuState.strategy.strategyId);
					} // Return the selected strategy
				}
			});
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

	function closeMenu(strategy: Setup | null | 'new' = null): void {
		console.trace();
		userSetupMenu.set({ ...inactiveUserSetupMenu, strategy: strategy });
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

	function constrainPosition(x: number, y: number): { x: number; y: number } {
		if (browser) {
			const windowWidth = window.innerWidth;
			const windowHeight = window.innerHeight;
			const menuWidth = 220;
			const menuHeight = menu?.offsetHeight || 0;

			return {
				x: Math.min(Math.max(MARGIN, x), windowWidth - menuWidth - MARGIN),
				y: Math.min(Math.max(MARGIN, y), windowHeight - menuHeight - MARGIN)
			};
		}
		return { x, y };
	}

	function handleMouseMove(event: MouseEvent): void {
		if (isDragging) {
			const deltaX = event.clientX - startX;
			const deltaY = event.clientY - startY;
			const { x, y } = constrainPosition(initialX + deltaX, initialY + deltaY);
			userSetupMenu.update((menuState) => {
				return { ...menuState, x, y };
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
		class="popup-container responsive-shadow responsive-border"
		bind:this={menu}
		style="top: {$userSetupMenu.y}px; left: {$userSetupMenu.x}px;"
	>
		<div class="content-container content-padding">
			<table>
				<thead>
					<tr class="defalt-tr">
						<th class="defalt-th">Setup</th>
					</tr>
				</thead>
				<tbody>
					{#if $strategies && $strategies.length > 0}
						{#each $strategies as strategy}
							<tr class="defalt-tr">
								<td
									on:click={() => {
										closeMenu(strategy);
									}}
								>
									{strategy.name}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
{/if}

<style>
	.popup-container {
		width: clamp(180px, 30vw, 220px);
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: clamp(6px, 0.8vw, 8px);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
		position: fixed;
		z-index: 1000;
		padding: clamp(2px, 0.5vw, 4px);
	}

	.content-container {
		padding: clamp(4px, 1vw, 8px);
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
		padding: clamp(6px, 1vw, 8px) clamp(8px, 1.5vw, 12px);
		color: var(--text-secondary);
		font-weight: 600;
		text-transform: uppercase;
	}

	td {
		padding: clamp(6px, 1vw, 8px) clamp(8px, 1.5vw, 12px);
		color: var(--text-primary);
		cursor: pointer;
		border-radius: clamp(3px, 0.5vw, 4px);
		transition: background-color 0.2s ease;
	}

	td:hover {
		background: var(--ui-bg-hover);
	}
</style>
