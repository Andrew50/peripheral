<script lang="ts" context="module">
	import { writable } from 'svelte/store';
	import type { Writable } from 'svelte/store';

	interface UserAlgoMenu {
		x: number;
		y: number;
		status: 'active' | 'inactive';
		algoId: number | null;
	}

	const inactiveUserAlgoMenu = { x: 0, y: 0, status: 'inactive', algoId: null } as UserAlgoMenu;
	let userAlgoMenu: Writable<UserAlgoMenu> = writable(inactiveUserAlgoMenu);
	const MARGIN = 20; // Margin from window edges in pixels

	// Initial position without height constraint
	function getInitialPosition(x: number, y: number): { x: number; y: number } {
		if (browser) {
			const windowWidth = window.innerWidth;
			const menuWidth = 200; // Same as our CSS width
			return {
				x: Math.min(Math.max(MARGIN, x), windowWidth - menuWidth - MARGIN),
				y: Math.max(MARGIN, y)
			};
		}
		return { x, y };
	}

	export async function queryAlgo(event: MouseEvent): Promise<number> {
		const { x, y } = getInitialPosition(event.clientX, event.clientY);
		const menuState: UserAlgoMenu = {
			x,
			y,
			status: 'active',
			algoId: null
		};
		userAlgoMenu.set(menuState);
		return new Promise<number>((resolve, reject) => {
			const unsubscribe = userAlgoMenu.subscribe((menuState: UserAlgoMenu) => {
				if (menuState.status === 'inactive') {
					if (menuState.algoId !== null) {
						unsubscribe();
						resolve(menuState.algoId);
					} else {
						unsubscribe();
						reject();
					}
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

	// Hardcoded algo list
	const algos = [{ algoId: 0, name: 'Vol Burst' }];
	//TODO: get algos from backend
	onMount(() => {
		userAlgoMenu.subscribe((menuState) => {
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
		if (menu && !menu.contains(event.target as Node)) {
			closeMenu();
		}
	}

	function handleKeyDown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			closeMenu();
		}
	}

	function closeMenu(algoId: number | null = null): void {
		userAlgoMenu.set({ ...inactiveUserAlgoMenu, algoId });
	}

	// Move constrainPosition here where it has access to menu
	function constrainPosition(x: number, y: number): { x: number; y: number } {
		if (browser) {
			const windowWidth = window.innerWidth;
			const windowHeight = window.innerHeight;
			const menuWidth = 200;
			const menuHeight = menu?.offsetHeight || 0;

			return {
				x: Math.min(Math.max(MARGIN, x), windowWidth - menuWidth - MARGIN),
				y: Math.min(Math.max(MARGIN, y), windowHeight - menuHeight - MARGIN)
			};
		}
		return { x, y };
	}

	// Update handleMouseMove to use constrainPosition
	function handleMouseMove(event: MouseEvent): void {
		if (isDragging) {
			const deltaX = event.clientX - startX;
			const deltaY = event.clientY - startY;
			const { x, y } = constrainPosition(initialX + deltaX, initialY + deltaY);
			userAlgoMenu.update((menuState) => {
				return { ...menuState, x, y };
			});
		}
	}

	// ... existing mouse handling functions ...
	function handleMouseDown(event: MouseEvent): void {
		if (event.target && (event.target as HTMLElement).classList.contains('context-menu')) {
			startX = event.clientX;
			startY = event.clientY;
			initialX = $userAlgoMenu.x;
			initialY = $userAlgoMenu.y;
			isDragging = true;
			document.addEventListener('mousemove', handleMouseMove);
			document.addEventListener('mouseup', handleMouseUp);
		}
	}

	function handleMouseUp(): void {
		isDragging = false;
		document.removeEventListener('mousemove', handleMouseMove);
		document.removeEventListener('mouseup', handleMouseUp);
	}
</script>

{#if $userAlgoMenu.status === 'active'}
	<div
		class="popup-container"
		bind:this={menu}
		style="top: {$userAlgoMenu.y}px; left: {$userAlgoMenu.x}px;"
	>
		<div class="content-container">
			<table>
				<thead>
					<tr class="defalt-tr">
						<th class="defalt-th">Algorithm</th>
					</tr>
				</thead>
				<tbody>
					{#each algos as algo}
						<tr class="defalt-tr">
							<td on:click={() => closeMenu(algo.algoId)}>
								{algo.name}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}

<style>
	.popup-container {
		width: 200px;
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
		position: fixed;
		z-index: 1000;
	}

	.content-container {
		padding: 12px;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	.defalt-tr {
		border-bottom: 1px solid var(--ui-border);
	}

	.defalt-th {
		text-align: left;
		padding: 8px;
		color: var(--text-secondary);
		font-size: 14px;
		font-weight: 500;
	}

	td {
		padding: 8px;
		color: var(--text-primary);
		font-size: 14px;
		cursor: pointer;
	}

	td:hover {
		background: var(--ui-bg-hover);
	}
</style>
