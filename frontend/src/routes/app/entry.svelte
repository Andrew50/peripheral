<script lang="ts" context="module">
    import type {Instance} from '../../store' 
    import {queryInstanceInput} from './instance.svelte'
</script>
<script lang="ts">
    import { onMount } from 'svelte';
    import { writable } from 'svelte/store'
    import { privateRequest } from '../../store'
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, EmbedBlot } from 'quill'

    //export let store: Writable<string>;

    export let func: string;
    export let id: number;
    let Quill;
    let editorContainer: HTMLElement | string;
    let editor: Quill | undefined;
    let errorMessage = writable("");

    function loadStudy(studyId: number): void {
        privateRequest<DeltaStatic>("getStudy",{studyId: studyId})
        .then((response: DeltaStatic) => {
            editor.setContents(response);
        }).catch((error) => {
             errorMessage.set(error);
        });
    }
    function save():void {
        privateRequest<void>(`save${func}`,{id:id,entry:JSON.stringify(editor?.getContents())})
    }
    function del():void{
        privateRequest<void>(`delete${func}`,{id:id})
    }

    /*interface EmbeddedInstance {
        ticker: string;
        timeframe: string;
        datetime: string;
        pm: boolean;
    }*/

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
