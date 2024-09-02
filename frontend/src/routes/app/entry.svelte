<script lang="ts" context="module">
    import type {Instance} from '../../store' 
    import {queryInstanceInput} from './instance.svelte'
    import {chartQuery} from './chart.svelte'
</script>
<script lang="ts">
    import { onMount } from 'svelte';
    import { privateRequest } from '../../store'
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, EmbedBlot } from 'quill'
    export let func: string;
    export let id: number;
    let Quill;
    let editorContainer: HTMLElement | string;
    let editor: Quill | undefined;

    function save():void {
        privateRequest<void>(`save${func}`,{id:id,entry:JSON.stringify(editor?.getContents())})
    }
    function del():void{
        privateRequest<void>(`delete${func}`,{id:id})
    }

    function insertInstance(): void {
        console.log('go')
        queryInstanceInput(["ticker","timeframe","datetime"])
        .then((instance: Instance) => {
            const range = editor.getSelection()
            let insertIndex;
            if (range === null){
                insertIndex = -1;
            }else{
                insertIndex = range.index
            }
            editor.insertEmbed(insertIndex, 'embeddedInstance',instance);
        })
    }

    function embeddedInstanceClick(instance: Instance): void {
        chartQuery.set(instance)
        console.log(instance.ticker, instance.datetime)
    }

    onMount(() => {
        import('quill').then(QuillModule => {
            Quill = QuillModule.default;
            editor = new Quill(editorContainer, {
                theme: 'snow',
                placeholder: 'Entry ...',
                modules: {
                    toolbar: false
                }
            });
            class ChartBlot extends (Quill.import('blots/embed') as typeof EmbedBlot) {
                static create(instance: Instance): HTMLElement {
                    let node = super.create();
                    node.setAttribute('type', 'button');
                    node.className = 'btn';
                    node.textContent = `${instance.ticker} ${instance.datetime}`; 
                    node.onclick = () => embeddedInstanceClick(instance)                    
                    return node;
                }

                static value(node: HTMLElement) {
                    return {
                        ticker: node.dataset.ticker,
                        timeframe: node.dataset.timeframe,
                        datetime: node.dataset.datetime,
//                        pm: node.dataset.pm
                    };
                }
            }
            ChartBlot.blotName = 'embeddedInstance';
            ChartBlot.tagName = 'button';
            Quill.register('formats/embeddedInstance', ChartBlot);
        })
        privateRequest<JSON>("getStudyEntry", { studyId: id })
        .then((entry: JSON) => {
            const delta: DeltaStatic = JSON.parse(entry as unknown as string);
            editor?.setContents(delta);
            console.log(editor.getContents())
        });
    });
</script>
<div bind:this={editorContainer}></div>
<div>
    <button on:click={insertInstance}> Insert Instance </button>
    <button on:click={save}> Save </button>
    <button on:click={del}> Delete </button>
</div>
<style>
  .ql-container {
    /*height: 200px;*/
    max-height: 75%;
    width: 100%;
    height: auto;
    overflow-y: auto;
    border: none;
  }
  :global(.btn) {
    background-color: #f1f1f1;
    border: 1px solid #ccc;
    border-radius: 4px;
    color: #333;
    cursor: pointer;
    display: inline-block;
    font-size: 14px;
    margin: 5px;
    padding: 5px 10px;
    text-align: center;
  }
</style>
