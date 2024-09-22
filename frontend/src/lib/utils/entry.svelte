<script lang="ts" context="module">
    import type {Instance} from '$lib/core/types' 
    import {queryInstanceInput } from '$lib/utils/input.svelte'
    import {queryInstanceRightClick} from '$lib/utils/rightClick.svelte'
    import {changeChart} from '$lib/features/chart/interface'
    import type {RightClickResult} from '$lib/utils/rightClick.svelte'
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
    import { onMount } from 'svelte';
    import { privateRequest } from '$lib/core/backend'
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, EmbedBlot } from 'quill'
    export let func: string;
    export let id: number;
    export let completed: boolean;
    let Quill;
    let editorContainer: HTMLElement | string;
    let editor: Quill | undefined;
    let lastSaveTimeout: ReturnType<typeof setTimeout> | undefined;
    
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

    /*function save():void {
        privateRequest<void>(`save${func}`,
        {id:id,
        entry:editor?.getContents()})
   
    }*/
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
        const range = editor.getSelection()
        let insertIndex;
        if (range === null){
            insertIndex = editor.getLength()
        }else{
            insertIndex = range.index
        }
        editor.insertEmbed(insertIndex, 'embeddedInstance',instance);
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
        changeChart(instance, true)

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
                console.log(op)
                if (op.insert && op.insert.embeddedInstance) {
                    const embedded = op.insert.embeddedInstance;
                    console.log(embedded)
                    console.log(instance)
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
        import('quill').then(QuillModule => {
            Quill = QuillModule.default;
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
                    node.className = 'btn';
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
</script>
<div bind:this={editorContainer}></div>
<div class="button-container">
    <button on:click={inputAndEmbedInstance} class="action-btn"> Insert </button>
    <button on:click={complete} class="action-btn"> {completed ? "Complete" : "Uncomplete"} </button>
    <!--<button on:click={save} class="action-btn"> Save </button>-->
    <button on:click={del} class="action-btn"> Delete </button>
</div>
<style>
    @import "$lib/core/colors.css";

    /* Quill container styling */
    .ql-container {
        max-height: 75%;
        width: 100%;
        overflow-y: auto;
        border: none;
    }

    /* Global styling for embedded buttons */
    :global(.btn) {
        background-color: var(--c1);
        border: 1px solid var(--c4);
        border-radius: 4px;
        color: var(--f1);
        cursor: pointer;
        display: inline-block;
        font-size: 14px;
        margin: 5px;
        padding: 5px 10px;
        text-align: center;
    }

    :global(.btn:hover) {
        background-color: var(--c3-hover);
    }

    /* Button styling for action buttons */
    .button-container {
        display: flex;
        justify-content: space-between;
        margin-top: 10px;
    }

    .action-btn {
        background-color: var(--c3);
        color: var(--f1);
        border: none;
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
    }

    .action-btn:hover {
        background-color: var(--c3-hover);
    }
</style>
