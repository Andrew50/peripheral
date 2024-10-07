<script lang="ts" context="module">
    import '$lib/core/global.css'
    import type {Instance} from '$lib/core/types' 
    import {queryInstanceInput } from '$lib/utils/popups/input.svelte'
    import {queryInstanceRightClick} from '$lib/utils/popups/rightClick.svelte'
    import {queryChart} from '$lib/features/chart/interface'
    import {menuWidth,entryOpen} from '$lib/core/stores'
    import type {RightClickResult} from '$lib/utils/popups/rightClick.svelte'
    import {writable} from 'svelte/store'
    import type {Writable} from 'svelte/store'
    let externalEmbed: Writable<Instance> = writable({})
    import {UTCTimestampToESTString} from '$lib/core/timestamp'
    export function embedInstance(instance:Instance):void{
        if (instance.ticker && instance.timestamp && instance.securityId && instance.timeframe){
            externalEmbed.set(instance)
        }
    }
</script>
<script lang="ts">
    import { onDestroy,onMount } from 'svelte';
    import { privateRequest } from '$lib/core/backend'
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, EmbedBlot } from 'quill'
    export let func: string;
    export let id: number;
    export let completed: boolean;
    let Quill;
    let editorContainer: HTMLElement | string;
    let controlsContainer: HTMLElement | string;
    let editor: Quill | undefined;
    let lastSaveTimeout: ReturnType<typeof setTimeout> | undefined;
    let quillWidth = writable(0);
    
    function debounceSave(): void {
        if (lastSaveTimeout) {
            clearTimeout(lastSaveTimeout);
        }
        lastSaveTimeout = setTimeout(() => {
            privateRequest<void>(`save${func}`, {
                id: id,
                entry: editor?.getContents(),
            });
        }, 1000);
    }

    function del():void{
        privateRequest<void>(`delete${func}`,{id:id})
    }
    function complete():void{
        completed = !completed
        privateRequest<void>(`complete${func}`,{id:id,completed:completed})
    }

    externalEmbed.subscribe((v:Instance)=>{
        if (Object.keys(v).length > 0){
            insertEmbeddedInstance(v)
        }
    })
    function insertEmbeddedInstance(instance:Instance):void{
        editor.focus()
        const range = editor.getSelection()
        let insertIndex;
        console.log(range)
        if (range === null){
            insertIndex = editor.getLength()
        }else{
            insertIndex = range.index
        }
        console.log(insertIndex)
        editor.insertEmbed(insertIndex, 'embeddedInstance',instance);
        editor.setSelection(insertIndex + 1, 0);
        debounceSave()
    }

    function inputAndEmbedInstance(): void {
        const blankInstance: Instance = {ticker:"",timestamp:0,timeframe:""}
        queryInstanceInput(["ticker","timeframe","timestamp"],blankInstance)
        .then((instance: Instance) => {
            insertEmbeddedInstance(instance)
        })
    }

    function embeddedInstanceLeftClick(instance: Instance): void {
        instance.securityId = parseInt(instance.securityId)
        instance.timestamp = parseInt(instance.timestamp)
        queryChart(instance, true)

    }

    function embeddedInstanceRightClick(instance: Instance, event:MouseEvent): void {
        event.preventDefault()
        instance.securityId = parseInt(instance.securityId)
        instance.timestamp = parseInt(instance.timestamp)
        queryInstanceRightClick(event,instance,"embedded")
        .then((res:RightClickResult)=>{
            if (res === "edit"){
                editEmbeddedInstance(instance)
            }

        })

    }
    function editEmbeddedInstance(instance:Instance): void{
        const ins = {...instance} //make a copy
        queryInstanceInput(["ticker", "timeframe", "timestamp"],ins)
        .then((updatedInstance: Instance) => {
            // Find the embedded instance in the editor content
            const delta = editor?.getContents();
            completed = false;
            delta?.ops?.forEach(op => {
                if (op.insert && op.insert.embeddedInstance) {
                    const embedded = op.insert.embeddedInstance;
                    if (embedded.ticker === instance.ticker && embedded.timestamp === instance.timestamp) {
                        embedded.ticker = updatedInstance.ticker;
                        embedded.timeframe = updatedInstance.timeframe;
                        embedded.timestamp = updatedInstance.timestamp;
                        embedded.securityId = updatedInstance.securityId;
                        completed = true;
                        
                    }
                }
            });
            if (!completed){
                console.error("failed edit")
            }
            editor?.setContents(delta);
            debounceSave()
        });
    }



    onMount(() => {
        entryOpen.set(true)
        import('quill').then(QuillModule => {
            Quill = QuillModule.default;
            const Block = Quill.import('blots/block');
            Block.tagName = 'div';
            Quill.register(Block);
            editor = new Quill(editorContainer, {
                theme: 'snow',
                placeholder: 'Entry ...',
                modules: {
                    toolbar: false
                }
            });
            editor.on('text-change',() => {
                debounceSave();
            })
            class ChartBlot extends (Quill.import('blots/embed') as typeof EmbedBlot) {
                static create(instance: Instance): HTMLElement {
                    let node = super.create();
                    node.setAttribute('type', 'button');
                    node.className = 'embedded-button';
                    node.dataset.securityId = instance.securityId.toString()
                    node.dataset.ticker = instance.ticker
                    node.dataset.timestamp = instance.timestamp.toString()
                    node.dataset.timeframe = instance.timeframe
                    node.textContent = `${instance.ticker} ${UTCTimestampToESTString(parseInt(instance.timestamp))}`; 
                    node.onclick = () => embeddedInstanceLeftClick(instance)                    
                    node.oncontextmenu = (event:MouseEvent) => embeddedInstanceRightClick(instance,event)                    
                    return node;
                }

                static value(node: HTMLElement) {
                    return {
                        ticker: node.dataset.ticker,
                        timeframe: node.dataset.timeframe,
                        timestamp: parseInt(node.dataset.timestamp),
                        securityId: parseInt(node.dataset.securityId)
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
    onDestroy(()=>{
        entryOpen.set(false)
    })
</script>
<div class="editor-container" style="width: {$menuWidth - 11}px">
    <div bind:this={editorContainer}></div>
</div>
<div class="controls-container" bind:this={controlsContainer}>
    <button on:click={inputAndEmbedInstance}> Insert </button>
    <button on:click={complete}> {completed ? "Complete" : "Uncomplete"} </button>
    <!--<button on:click={save} class="action-btn"> Save </button>-->
    <button on:click={del}> Delete </button>
</div>
<style>
  .editor-container {
      overflow: hidden; /* Prevent overflowing */
      box-sizing: border-box;
      border: none;
      align-items: center;
      justify-content: center;
      width:100px;
  }
    :global(.embedded-button) {
        padding:1px;
        margin: 1px;
    }
</style>
