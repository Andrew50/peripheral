<script lang="ts" context="module">
	import '$lib/core/global.css';
	import type { Instance, Strategy } from '$lib/core/types';
	import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
	import { queryInstanceRightClick } from '$lib/utils/popups/rightClick.svelte';
	import { queryChart } from '$lib/features/chart/interface';
	import { queryStrategy } from '$lib/utils/popups/strategiesPopup.svelte';
	import { menuWidth, entryOpen, dispatchMenuChange } from '$lib/core/stores';
	import type { RightClickResult } from '$lib/utils/popups/rightClick.svelte';
	import { writable } from 'svelte/store';
	import type { Writable } from 'svelte/store';
	let externalEmbed: Writable<Instance> = writable({});
	import { UTCTimestampToESTString } from '$lib/core/timestamp';
	export function embedInstance(instance: Instance): void {
		if (instance.ticker && instance.timestamp && instance.securityId && instance.timeframe) {
			externalEmbed.set(instance);
		}
	}
</script>

<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { privateRequest } from '$lib/core/backend';
	import 'quill/dist/quill.snow.css';
	import type Quill from 'quill';
	import type { QuillOptions } from 'quill';
	export let func: string;
	export let id: number;
	export let completed: boolean;
	export let strategyId: number | null | undefined = undefined;
	let Quill: any;
	let editorContainer: HTMLElement;
	let controlsContainer: HTMLElement;
	let editor: Quill | undefined;
	let lastSaveTimeout: ReturnType<typeof setTimeout> | undefined;
	let quillWidth = writable(0);
	let isCompleted = completed;

	function debounceSave(): void {
		if (lastSaveTimeout) {
			clearTimeout(lastSaveTimeout);
		}
		lastSaveTimeout = setTimeout(() => {
			privateRequest<void>(`save${func}`, {
				id: id,
				entry: editor?.getContents()
			});
		}, 1000);
	}

	function del(): void {
		privateRequest<void>(`delete${func}`, { id: id });
	}
	function changeStrategy(event: MouseEvent) {
		queryStrategy(event).then((i: number | null) => {
			if (i !== null) {
				privateRequest<null>('setStudyStrategy', { id: id, strategyId: i });
			}
		});
	}
	function complete(): void {
		completed = !completed;
		privateRequest<void>(`complete${func}`, { id: id, completed: completed });
	}

	externalEmbed.subscribe((v: Instance) => {
		if (Object.keys(v).length > 0) {
			insertEmbeddedInstance(v);
		}
	});
	function insertEmbeddedInstance(instance: Instance): void {
		if (!editor) return;
		editor.focus();
		const range = editor.getSelection();
		let insertIndex: number;

		if (range === null) {
			insertIndex = editor.getLength();
		} else {
			insertIndex = range.index;
		}

		editor.insertEmbed(insertIndex, 'embeddedInstance', instance);
		editor.setSelection(insertIndex + 1, 0);
		debounceSave();
	}

	function inputAndEmbedInstance(): void {
		const blankInstance: Instance = { ticker: '', timestamp: 0, timeframe: '' };
		queryInstanceInput(
			['ticker', 'timeframe', 'timestamp'],
			['ticker', 'timeframe', 'timestamp'],
			blankInstance
		).then((instance: Instance) => {
			insertEmbeddedInstance(instance);
		});
	}

	function embeddedInstanceLeftClick(instance: Instance): void {
		if (!instance.securityId || !instance.timestamp) return;
		const securityId = parseInt(instance.securityId.toString());
		const timestamp = parseInt(instance.timestamp.toString());
		if (isNaN(securityId) || isNaN(timestamp)) return;

		const updatedInstance = {
			...instance,
			securityId,
			timestamp
		};
		queryChart(updatedInstance, true);
	}

	function embeddedInstanceRightClick(instance: Instance, event: MouseEvent): void {
		event.preventDefault();
		if (!instance.securityId || !instance.timestamp) return;
		const securityId = parseInt(instance.securityId.toString());
		const timestamp = parseInt(instance.timestamp.toString());
		if (isNaN(securityId) || isNaN(timestamp)) return;

		const updatedInstance = {
			...instance,
			securityId,
			timestamp
		};
		queryInstanceRightClick(event, updatedInstance, 'embedded').then((res: RightClickResult) => {
			if (res === 'edit') {
				editEmbeddedInstance(updatedInstance);
			}
		});
	}
	function editEmbeddedInstance(instance: Instance): void {
		const ins = { ...instance }; //make a copy
		queryInstanceInput(
			['ticker', 'timeframe', 'timestamp'],
			['ticker', 'timeframe', 'timestamp', 'extendedHours'],
			ins
		).then((updatedInstance: Instance) => {
			// Find the embedded instance in the editor content
			if (!editor) return;
			const delta = editor.getContents();
			isCompleted = false;

			if (delta && delta.ops) {
				delta.ops.forEach((op: Record<string, any>) => {
					if (op.insert && typeof op.insert === 'object' && 'embeddedInstance' in op.insert) {
						const embedded = op.insert.embeddedInstance;
						if (embedded.ticker === instance.ticker && embedded.timestamp === instance.timestamp) {
							embedded.ticker = updatedInstance.ticker;
							embedded.timeframe = updatedInstance.timeframe;
							embedded.timestamp = updatedInstance.timestamp;
							embedded.securityId = updatedInstance.securityId;
							isCompleted = true;
						}
					}
				});
			}

			if (!isCompleted) {
				console.error('failed edit');
			}

			if (editor && delta) {
				editor.setContents(delta);
				editor.getContents();
			}
			debounceSave();
		});
	}

	onMount(() => {
		entryOpen.set(true);
		import('quill').then((QuillModule) => {
			Quill = QuillModule.default;
			const Block = Quill.import('blots/block');
			Block.tagName = 'div';
			Quill.register(Block, true);

			class ChartBlot extends Quill.import('blots/embed') {
				static blotName = 'embeddedInstance';
				static tagName = 'button';

				static create(instance: Instance): HTMLElement {
					const node = super.create() as HTMLElement;
					node.setAttribute('type', 'button');
					node.className = 'embedded-button';
					if (instance.securityId) node.dataset.securityId = instance.securityId.toString();
					if (instance.ticker) node.dataset.ticker = instance.ticker;
					if (instance.timestamp) node.dataset.timestamp = instance.timestamp.toString();
					if (instance.timeframe) node.dataset.timeframe = instance.timeframe;
					node.textContent = `${instance.ticker || ''} ${instance.timestamp ? UTCTimestampToESTString(instance.timestamp) : ''}`;
					node.onclick = () => embeddedInstanceLeftClick(instance);
					node.oncontextmenu = (event: MouseEvent) => embeddedInstanceRightClick(instance, event);
					return node;
				}

				static value(node: HTMLElement) {
					return {
						ticker: node.dataset.ticker,
						timeframe: node.dataset.timeframe,
						timestamp: node.dataset.timestamp ? parseInt(node.dataset.timestamp) : undefined,
						securityId: node.dataset.securityId ? parseInt(node.dataset.securityId) : undefined
					};
				}
			}

			Quill.register('formats/embeddedInstance', ChartBlot);

			editor = new Quill(editorContainer, {
				theme: 'snow',
				placeholder: 'Entry ...',
				modules: {
					toolbar: false
				}
			});

			if (editor) {
				editor.on('text-change', () => {
					debounceSave();
				});
			}

			privateRequest<any>('getStudyEntry', { studyId: id }).then((entry: any) => {
				if (editor) {
					editor.setContents(entry);
					editor.getContents();
				}
			});
		});
	});
	onDestroy(() => {
		if (editor) {
			editor.getContents();
		}
		entryOpen.set(false);
	});
</script>

<div class="editor-container" style="width: {$menuWidth - 11}px">
	<div bind:this={editorContainer}></div>
</div>
<div class="controls-container" bind:this={controlsContainer}>
	<button on:click={inputAndEmbedInstance}> Insert </button>
	<button on:click={complete}> {completed ? 'Complete' : 'Uncomplete'} </button>
	<!--<button on:click={save} class="action-btn"> Save </button>-->
	{#if strategyId !== undefined}
		<button
			on:click={(event) => {
				changeStrategy(event);
			}}
		>
			Change Strategy
		</button>
	{/if}
	<button on:click={del}> Delete </button>
</div>

<style>
	.editor-container {
		overflow: hidden; /* Prevent overflowing */
		box-sizing: border-box;
		border: none;
		align-items: center;
		justify-content: center;
		width: 100px;
	}
	:global(.embedded-button) {
		padding: 1px;
		margin: 1px;
	}
</style>
