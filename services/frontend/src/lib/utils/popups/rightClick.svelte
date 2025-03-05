<!-- rightClick.svlete -->
<script lang="ts" context="module">
	import '$lib/core/global.css';
	import type { Writable } from 'svelte/store';
	import type { Instance, Setup } from '$lib/core/types';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	import { flagSecurity } from '$lib/utils/flag';
	import { embedInstance } from '$lib/utils/modules/entry.svelte';
	import { newStudy } from '$lib/features/study.svelte';
	import { get, writable } from 'svelte/store';
	import { setSample } from '$lib/features/setups/interface';
	import { querySimilarInstances } from '$lib/features/similar/interface';
	import { newPriceAlert } from '$lib/features/alerts/interface';
	import { querySetup } from '$lib/utils/popups/setup.svelte';
	import { startReplay } from '$lib/utils/stream/interface';
	import { addHorizontalLine } from '$lib/features/chart/drawingMenu.svelte';
	interface RightClickQuery {
		x?: number;
		y?: number;
		source?: Source;
		instance: Instance;
		status: 'inactive' | 'active' | 'initializing' | 'cancelled' | 'complete';
		result: RightClickResult;
	}
	export type RightClickResult = 'edit' | 'embed' | 'alert' | 'embedSimilar' | 'none' | 'flag';
	type Source = 'chart' | 'embedded' | 'similar' | 'list' | 'header';
	const inactiveRightClickQuery: RightClickQuery = {
		status: 'inactive',
		result: 'none',
		instance: {}
	};

	let rightClickQuery: Writable<RightClickQuery> = writable(inactiveRightClickQuery);

	export async function queryInstanceRightClick(
		event: MouseEvent,
		instance: Instance,
		source: Source
	): Promise<RightClickResult> {
		event.preventDefault();
		const rqQ: RightClickQuery = {
			x: event.clientX,
			y: event.clientY,
			source: source,
			status: 'initializing',
			instance: instance,
			result: 'none'
		};
		rightClickQuery.set(rqQ);
		return new Promise<RightClickResult>((resolve, reject) => {
			const unsubscribe = rightClickQuery.subscribe((r: RightClickQuery) => {
				if (r.status === 'cancelled') {
					deactivate();
					reject();
				} else if (r.status === 'complete') {
					const res = r.result;
					deactivate();
					resolve(res);
				}
			});
			function deactivate() {
				rightClickQuery.set(inactiveRightClickQuery);
				unsubscribe();
			}
		});
	}
</script>

<script lang="ts">
	import { entryOpen } from '$lib/core/stores';
	import { browser } from '$app/environment';
	import { onMount, tick } from 'svelte';
	let rightClickMenu: HTMLElement;
	onMount(() => {
		rightClickQuery.subscribe(async (v: RightClickQuery) => {
			if (browser) {
				if (v.status === 'initializing') {
					document.addEventListener('click', handleClick);
					document.addEventListener('keydown', handleKeyDown);
					await tick();
					if (!rightClickMenu) return;

					const menuRect = rightClickMenu.getBoundingClientRect();
					const windowWidth = window.innerWidth;
					const windowHeight = window.innerHeight;
					v.y;
					let newX = v.x;
					let newY = v.y;
					const halfMenuWidth = Math.floor(menuRect.width * 0.5);
					const halfMenuHeight = Math.floor(menuRect.height * 0.5);
					if (v.x && v.y) {
						if (v.x + halfMenuWidth > windowWidth) {
							newX = windowWidth - halfMenuWidth - 40; // Add a small margin from the edge
						} else if (v.x - halfMenuWidth < 0) {
							newX = halfMenuWidth + 40;
						}
						if (v.y + menuRect.height > windowHeight) {
							newY = windowHeight - halfMenuHeight - 40; // Add a small margin from the edge
						} else if (v.y - halfMenuWidth < 0) {
							newY = halfMenuWidth + 40;
						}
					}
					v.y;
					v.status = 'active';
					rightClickQuery.update((c: RightClickQuery) => {
						return {
							...c,
							x: newX,
							y: newY,
							status: 'active'
						};
					});
				} else if (v.status == 'inactive') {
					document.removeEventListener('click', handleClick);
					document.removeEventListener('keydown', handleKeyDown);
				}
			}
		});
	});
	function handleClick(event: MouseEvent): void {
		//if (rightClickMenu && !rightClickMenu.contains(event.target as Node)) {
		closeRightClickMenu();
		//}
	}
	function handleKeyDown(event: KeyboardEvent): void {
		if (event.key == 'Escape') {
			closeRightClickMenu();
		}
	}
	function closeRightClickMenu(): void {
		rightClickQuery.update((v: RightClickQuery) => {
			v.status = 'complete';
			return v;
		});
	}

	function getStats(): void {}
	function replay(): void {}
	function addAlert(): void {}
	function embed(): void {}
	function edit(): void {
		rightClickQuery.update((v: RightClickQuery) => {
			v.result = 'edit';
			return v;
		});
	}
	function cancelRequest() {
		rightClickQuery.update((v: RightClickQuery) => {
			v.status = 'cancelled';
			return v;
		});
	}
	function completeRequest(
		result: RightClickResult,
		func: ((instance: Instance) => void) | null = null
	) {
		rightClickQuery.update((v: RightClickQuery) => {
			v.status = 'complete';
			v.result = result;
			return v;
		});
		if (func !== null) {
			func(get(rightClickQuery).instance);
		}
	}
	function embedSimilar(): void {
		rightClickQuery.update((v: RightClickQuery) => {
			v.result = 'embedSimilar';
			return v;
		});
	}

	function sSample(event: MouseEvent) {
		querySetup(event).then((v: number) => {
			if (v == null) return;
			setSample(v, $rightClickQuery.instance);
		});
	}
</script>

{#if ['initializing', 'active'].includes($rightClickQuery.status)}
	<div
		bind:this={rightClickMenu}
		class="popup-container"
		style="top: {$rightClickQuery.y}px; left: {$rightClickQuery.x}px;"
	>
		<div class="header">
			<div class="ticker">{$rightClickQuery.instance.ticker || ''}</div>
			<div class="timestamp">
				{$rightClickQuery.instance.timestamp
					? UTCTimestampToESTString($rightClickQuery.instance.timestamp)
					: ''}
			</div>
		</div>

		<div class="section">
			<button class="wide-button" on:click={() => startReplay($rightClickQuery.instance)}
				>Begin Replay</button
			>
			{#if $rightClickQuery.source === 'chart'}
				<div class="separator"></div>
				<button class="wide-button" on:click={() => newPriceAlert($rightClickQuery.instance)}
					>Add Alert {$rightClickQuery.instance.price?.toFixed(2)}</button
				>
				<button
					class="wide-button"
					on:click={() =>
						addHorizontalLine(
							$rightClickQuery.instance.price || 0,
							$rightClickQuery.instance.securityId || 0
						)}>Add Horizontal Line {$rightClickQuery.instance.price?.toFixed(2)}</button
				>
			{/if}
		</div>

		<div class="section">
			<button class="wide-button" on:click={() => newStudy(get(rightClickQuery).instance)}>
				Add to Study
			</button>
			<button class="wide-button" on:click={(event) => sSample(event)}> Add to Sample </button>
			<button
				class="wide-button"
				on:click={(event) => querySimilarInstances($rightClickQuery.instance)}
			>
				Similar Instances
			</button>
		</div>

		{#if $entryOpen}
			<div class="section">
				<button class="wide-button" on:click={() => embedInstance(get(rightClickQuery).instance)}>
					Embed
				</button>
			</div>
		{/if}

		{#if $rightClickQuery.source === 'embedded'}
			<div class="section">
				<button class="wide-button" on:click={() => completeRequest('edit')}> Edit </button>
			</div>
		{:else if $rightClickQuery.source === 'list'}
			<div class="section">
				<button class="wide-button" on:click={() => flagSecurity($rightClickQuery.instance)}
					>{$rightClickQuery.instance.flagged ? 'Unflag' : 'Flag'}</button
				>
			</div>
		{/if}
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

	.header {
		padding: 8px 12px;
		border-bottom: 1px solid var(--ui-border);
		margin-bottom: 4px;
	}

	.ticker {
		font-size: 16px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.timestamp {
		font-size: 12px;
		color: var(--text-secondary);
		margin-top: 2px;
	}

	.section {
		padding: 4px 0;
		border-bottom: 1px solid var(--ui-border);
	}

	.section:last-child {
		border-bottom: none;
	}

	.wide-button {
		width: 100%;
		padding: 8px 12px;
		text-align: left;
		background: transparent;
		border: none;
		color: var(--text-primary);
		font-size: 14px;
		cursor: pointer;
		border-radius: 4px;
		transition: background-color 0.2s ease;
	}

	.wide-button:hover {
		background: var(--ui-bg-hover);
	}

	.separator {
		margin: 4px 8px;
		height: 1px;
		background: var(--ui-border);
		opacity: 0.6;
	}
</style>
