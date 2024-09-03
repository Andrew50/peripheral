<script lang="ts" context="module">
    import type {Instance} from '$lib/api/backend' 
    import {queryInstanceInput } from '$lib/utils/input.svelte'
    import {queryInstanceRightClick} from '$lib/utils/rightClick.svelte'
    import {chartQuery} from './chart.svelte'
</script>
<script lang="ts">
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/api/backend'
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, EmbedBlot } from 'quill'
    export let func: string;
    export let id: number;
    export let completed: boolean;
    let Quill;
    let editorContainer: HTMLElement | string;
    let editor: Quill | undefined;
    

    function save():void {
        privateRequest<void>(`save${func}`,
        {id:id,
        entry:editor?.getContents()})
   
    }
    function del():void{
        privateRequest<void>(`delete${func}`,{id:id})
    }
    function complete():void{
        completed = !completed
        privateRequest<void>(`complete${func}`,{id:id,completed:completed})
    }

    function insertInstance(): void {
        queryInstanceInput(["ticker","timeframe","datetime"])
        .then((instance: Instance) => {
            const range = editor.getSelection()
            let insertIndex;
            if (range === null){
                insertIndex = editor.getLength()
            }else{
                insertIndex = range.index
            }
            editor.insertEmbed(insertIndex, 'embeddedInstance',instance);
        })
    }

    function embeddedInstanceLeftClick(instance: Instance): void {

        console.log(instance)
        instance.securityId = parseInt(instance.securityId)
        chartQuery.set(instance)
        console.log(instance.ticker, instance.datetime)
    }

    function embeddedInstanceRightClick(instance: Instance, event:MouseEvent): void {
        event.preventDefault()
        instance.securityId = parseInt(instance.securityId)
        queryInstanceRightClick(event,instance,"embedded")
        .then((res:RightClickResult)=>{
            console.log(res)
            if (res.result == "edit"){
                editEmbeddedInstance(instance)
            }

        })

    }
    function editEmbeddedInstance(instance:Instance): void{
        queryInstanceInput(["ticker", "timeframe", "datetime"],instance)
        .then((updatedInstance: Instance) => {
            // Find the embedded instance in the editor content
            const delta = editor?.getContents();
            delta?.ops?.forEach(op => {
                if (op.insert && op.insert.embeddedInstance) {
                    const embedded = op.insert.embeddedInstance;
                    if (embedded.ticker === instance.ticker && embedded.datetime === instance.datetime) {
                        embedded.ticker = updatedInstance.ticker;
                        embedded.timeframe = updatedInstance.timeframe;
                        embedded.datetime = updatedInstance.datetime;
                    }
                }
            });
            editor?.setContents(delta);
        });
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
                    node.dataset.securityId = instance.securityId
                    node.dataset.ticker = instance.ticker
                    node.dataset.datetime = instance.datetime
                    node.dataset.timeframe = instance.timeframe
                    node.textContent = `${instance.ticker} ${instance.datetime}`; 
                    node.onclick = () => embeddedInstanceLeftClick(instance)                    
                    node.oncontextmenu = (event:MouseEvent) => embeddedInstanceRightClick(instance,event)                    
                    return node;
                }

                static value(node: HTMLElement) {
                    return {
                        ticker: node.dataset.ticker,
                        timeframe: node.dataset.timeframe,
                        datetime: node.dataset.datetime,
                        securityId: node.dataset.securityId
//                        pm: node.dataset.pm
                    };
                }
            }
            ChartBlot.blotName = 'embeddedInstance';
            ChartBlot.tagName = 'button';
            Quill.register('formats/embeddedInstance', ChartBlot);
            privateRequest<JSON>("getStudyEntry", { studyId: id })
            .then((entry: JSON) => {
                const delta: DeltaStatic = entry// as unknown as string;
                editor?.setContents(delta);
                console.log(editor.getContents())
            });
        })
    });
</script>
<div bind:this={editorContainer}></div>
<div>
    <button on:click={insertInstance}> Insert </button>
    <button on:click={complete}> {completed ? "Complete" : "Uncomplete"} </button>
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
