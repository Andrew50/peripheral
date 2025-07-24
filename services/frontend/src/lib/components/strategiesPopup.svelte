<!-- strategiesPopup.svelte -->
<script lang="ts" context="module">
	/* ─────────────── Stores & Types ─────────────── */
	import { writable, type Writable } from 'svelte/store';
	import { strategies } from '$lib/utils/stores/stores';
	import type { Strategy as CoreStrategy } from '$lib/utils/types/types';
	import { eventDispatcher } from '$lib/features/strategies/interface'; // 🆕 dispatch "new"

	/* ─────────────── Menu State ─────────────── */
	interface StrategyMenuState {
		x: number;
		y: number;
		status: 'active' | 'inactive';
		strategy: CoreStrategy | null | 'new';
	}

	const inactiveState: StrategyMenuState = { x: 0, y: 0, status: 'inactive', strategy: null };
	const menuState: Writable<StrategyMenuState> = writable(inactiveState);

	const MARGIN = 20;
	function clamp(v: number, min: number, max: number) {
		return Math.min(Math.max(v, min), max);
	}

	/* ─────────────── Public helper ─────────────── */
	/** Opens the pop‑up and resolves with the chosen strategyId. */
	export function queryStrategy(e: MouseEvent): Promise<number> {
		const menuWidth = 220;
		const { clientX, clientY } = e;

		menuState.set({
			x: browser ? clamp(clientX, MARGIN, window.innerWidth - menuWidth - MARGIN) : clientX,
			y: clamp(clientY, MARGIN, 99999), // y is further clamped when dragging
			status: 'active',
			strategy: null
		});

		return new Promise<number>((resolve, reject) => {
			const unsub = menuState.subscribe((s) => {
				if (s.status === 'inactive') {
					unsub();

					if (s.strategy === 'new') {
						// 🆕 "create new" clicked
						eventDispatcher.set('new');
						reject('new');
					} else if (s.strategy) {
						resolve(s.strategy.strategyId);
					} else {
						reject('cancel');
					}
				}
			});
		});
	}
</script>

<script lang="ts">
	/* ─────────────── Local behaviour ─────────────── */
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';

	let menu: HTMLElement;
	let dragStart = { x: 0, y: 0 };
	let initialPos = { x: 0, y: 0 };
	let dragging = false;

	function close(strat: CoreStrategy | null | 'new' = null) {
		menuState.set({ ...inactiveState, strategy: strat });
	}

	/* event listeners are attached only while the menu is open */
	onMount(() => {
		menuState.subscribe((s) => {
			if (!browser) return;

			if (s.status === 'active') {
				setTimeout(() => {
					window.addEventListener('click', outside);
					window.addEventListener('keydown', esc);
				});
			} else {
				window.removeEventListener('click', outside);
				window.removeEventListener('keydown', esc);
				window.removeEventListener('mousemove', move);
				window.removeEventListener('mouseup', up);
			}
		});
	});

	function outside(e: MouseEvent) {
		if (menu && !menu.contains(e.target as Node)) close();
	}
	function esc(e: KeyboardEvent) {
		if (e.key === 'Escape') close();
	}

	function down(e: MouseEvent | KeyboardEvent) {
		if (!(e.target as HTMLElement).classList.contains('context-menu')) return;
		dragging = true;
		const clientX = 'clientX' in e ? e.clientX : 0;
		const clientY = 'clientY' in e ? e.clientY : 0;
		dragStart = { x: clientX, y: clientY };
		initialPos = { x: $menuState.x, y: $menuState.y };
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}
	function move(e: MouseEvent) {
		if (!dragging) return;
		const { innerWidth, innerHeight } = window;
		const menuWidth = 220;
		const menuHeight = menu?.offsetHeight ?? 0;

		const nx = clamp(
			initialPos.x + (e.clientX - dragStart.x),
			MARGIN,
			innerWidth - menuWidth - MARGIN
		);
		const ny = clamp(
			initialPos.y + (e.clientY - dragStart.y),
			MARGIN,
			innerHeight - menuHeight - MARGIN
		);
		menuState.update((s) => ({ ...s, x: nx, y: ny }));
	}
	function up() {
		dragging = false;
		window.removeEventListener('mousemove', move);
		window.removeEventListener('mouseup', up);
	}
</script>

{#if $menuState.status === 'active'}
	<!-- svelte-ignore a11y-no-noninteractive-tabindex -->
	<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
	<div
		class="context-menu popup-container responsive-shadow responsive-border"
		bind:this={menu}
		style="top: {$menuState.y}px; left: {$menuState.x}px;"
		on:mousedown|preventDefault={down}
		on:keydown={(e) => {
			if (e.key === 'Enter' || e.key === ' ') down(e);
		}}
		role="dialog"
		aria-label="Strategy Menu"
		tabindex="0"
	>
		<div class="content-container content-padding">
			<table>
				<thead>
					<tr class="header-row">
						<th>Strategy</th>
					</tr>
				</thead>
				<tbody>
					<!-- 🆕 "Create new" entry -->
					<tr class="item-row new-row" on:click={() => close('new')}>
						<td>＋ New strategy</td>
					</tr>

					{#each $strategies as strat}
						<tr class="item-row" on:click={() => close(strat)}>
							<td>{strat.name}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{/if}

<style>
	.popup-container {
		width: clamp(180px, 30vw, 220px);
		position: fixed;
		z-index: 1000;
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: clamp(6px, 0.8vw, 8px);
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0 0 0 / 0.2);
	}
	.content-container {
		padding: clamp(4px, 1vw, 8px);
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}
	.header-row th {
		text-align: left;
		font-weight: 600;
		text-transform: uppercase;
		padding: 0.5rem 0.75rem;
		color: var(--text-secondary);
		border-bottom: 1px solid var(--ui-border);
	}

	.item-row td {
		padding: 0.5rem 0.75rem;
		cursor: pointer;
		border-radius: 4px;
		transition: background 0.2s;
	}
	.item-row:hover td {
		background: var(--ui-bg-hover);
	}

	.new-row td {
		font-style: italic;
	} /* 🆕 styling */
</style>
