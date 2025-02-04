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

	export async function queryAlgo(event: MouseEvent): Promise<number> {
		const menuState: UserAlgoMenu = {
			x: event.clientX,
			y: event.clientY,
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

	function handleMouseMove(event: MouseEvent): void {
		if (isDragging) {
			const deltaX = event.clientX - startX;
			const deltaY = event.clientY - startY;
			userAlgoMenu.update((menuState) => {
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

{#if $userAlgoMenu.status === 'active'}
	<div
		class="popup-container"
		bind:this={menu}
		style="top: {$userAlgoMenu.y}px; left: {$userAlgoMenu.x}px;"
	>
		<div class="content-container">
			<table>
				<thead>
					<tr>
						<th>Algorithm</th>
					</tr>
				</thead>
				<tbody>
					{#each algos as algo}
						<tr>
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
