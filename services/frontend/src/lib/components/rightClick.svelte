<!-- rightClick.svlete -->
<script lang="ts" context="module">
	import '$lib/styles/global.css';
	import type { Writable } from 'svelte/store';
	import type { Instance, Setup } from '$lib/utils/types/types';
	import { UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	import { flagSecurity } from '$lib/utils/stores/flag';
	//import { embedInstance } from '$lib/components/entry.svelte';
	//import { newStudy } from '$lib/features/study.svelte';
	import { get, writable } from 'svelte/store';
	import { setSample } from '$lib/features/strategies/interface';
//	import { querySimilarInstances } from '$lib/features/similar/interface';
	import { newPriceAlert } from '$lib/features/alerts/interface';
	import { queryStrategy } from '$lib/components/strategiesPopup.svelte';
	import { startReplay } from '$lib/utils/stream/interface';
	import { addHorizontalLine } from '$lib/features/chart/drawingMenu.svelte';
	//import { getLLMSummary } from '$lib/features/summary.svelte';
    import {addInstanceToChat} from '$lib/features/chat/interface';
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
	import { entryOpen } from '$lib/utils/stores/stores';
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
		console.log(
			'Right-click action:',
			get(rightClickQuery).result,
			'Instance:',
			get(rightClickQuery).instance
		);
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
		queryStrategy(event).then((v: number) => {
			if (v == null) return;
			setSample(v, $rightClickQuery.instance);
		});
	}
</script>

{#if ['initializing', 'active'].includes($rightClickQuery.status)}
	<div
		bind:this={rightClickMenu}
		class="popup-container glass glass--rounded glass--responsive"
		style="top: {$rightClickQuery.y}px; left: {$rightClickQuery.x}px;"
	>
		<div class="header content-padding">
			<div class="ticker">{$rightClickQuery.instance.ticker || ''}</div>
			<div class="timestamp fluid-text">
				{$rightClickQuery.instance.timestamp
					? UTCTimestampToESTString($rightClickQuery.instance.timestamp)
					: ''}
			</div>
		</div>

		<div class="section content-padding">
			<button class="wide-button" on:click={() => startReplay($rightClickQuery.instance)}
				>Begin Replay</button
			>
			{#if $rightClickQuery.source === 'chart'}
				<div class="separator"></div>
				<button class="wide-button" on:click={() => newPriceAlert($rightClickQuery.instance)}
					>Set Alert on {$rightClickQuery.instance.ticker} at {$rightClickQuery.instance.price?.toFixed(2)}</button
				>
				<button
					class="wide-button"
					on:click={() =>
						addHorizontalLine(
							Number($rightClickQuery.instance.price || 0),
							Number($rightClickQuery.instance.securityId || 0)
						)}>Add Horizontal Line at {$rightClickQuery.instance.price?.toFixed(2)}</button
				>
			{/if}
		</div>

		<div class="section content-padding">
			<!--<button class="wide-button" on:click={() => newStudy(get(rightClickQuery).instance)}>
				Add to Study
			</button>-->
			<!--<button class="wide-button" on:click={(event) => sSample(event)}> Add to Sample </button>-->
			<!--<button
				class="wide-button"
				on:click={(event) => querySimilarInstances($rightClickQuery.instance)}
			>
				Similar Instances
			</button>-->
			<!--<button class="wide-button" on:click={() => getLLMSummary($rightClickQuery.instance)}>
				Get LLM Summary for {$rightClickQuery.instance.ticker}
			</button>-->
            <button class="wide-button" on:click={() => addInstanceToChat($rightClickQuery.instance)}>
                Add to Chat
            </button>
		</div>

		<!--{#if $entryOpen}
			<div class="section content-padding">
				<button class="wide-button" on:click={() => embedInstance(get(rightClickQuery).instance)}>
					Embed
				</button>
			</div>
		{/if}-->

		{#if $rightClickQuery.source === 'embedded'}
			<div class="section content-padding">
				<button class="wide-button" on:click={() => completeRequest('edit')}> Edit </button>
			</div>
		{:else if $rightClickQuery.source === 'list'}
			<div class="section content-padding">
				<button class="wide-button" on:click={() => flagSecurity($rightClickQuery.instance)}
					>{$rightClickQuery.instance.flagged ? 'Unflag' : 'Flag'}</button
				>
			</div>
		{/if}
	</div>
{/if}

<style>
	.popup-container {
		width: clamp(180px, 30vw, 220px);
		/* Glass effect now provided by global .glass classes */
		display: flex;
		flex-direction: column;
		position: fixed;
		z-index: 1000;
		padding: clamp(2px, 0.5vw, 4px);
	}

	.header {
		padding: clamp(6px, 1vw, 8px) clamp(8px, 1.5vw, 12px);
		border-bottom: 1px solid var(--ui-border);
		margin-bottom: clamp(2px, 0.5vw, 4px);
	}

	.ticker {
		font-size: clamp(14px, 1.5vw, 16px);
		font-weight: 600;
		color: var(--text-primary);
	}

	.timestamp {
		color: var(--text-secondary);
		margin-top: clamp(1px, 0.3vw, 2px);
	}

	.section {
		padding: clamp(2px, 0.5vw, 4px) 0;
		border-bottom: 1px solid var(--ui-border);
	}

	.section:last-child {
		border-bottom: none;
	}

	.wide-button {
		width: 100%;
		padding: clamp(6px, 1vw, 8px) clamp(8px, 1.5vw, 12px);
		text-align: left;
		background: transparent;
		border: none;
		color: var(--text-primary);
		cursor: pointer;
		border-radius: clamp(3px, 0.5vw, 4px);
		transition: background-color 0.2s ease;
	}

	.wide-button:hover {
		background: var(--ui-bg-hover);
	}

	.separator {
		margin: clamp(2px, 0.5vw, 4px) clamp(6px, 1vw, 8px);
		height: 1px;
		background: var(--ui-border);
		opacity: 0.6;
	}
</style>
