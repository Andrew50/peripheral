<script lang="ts">
    import { onMount } from 'svelte';
    import { writable } from 'svelte/store'
    import { browser } from '$app/environment';
    import { privateRequest } from '../../store'
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, EmbedBlot } from 'quill'

    let Quill;
    let editorContainer: HTMLElement | string;
    let editor: Quill | undefined;
    let ticker = "";
    let timeframe = "";
    let datetime = "";
    let pm = false;
    let insertInstanceVisible = false;
    let errorMessage = writable("");
    function loadStudy(studyId: number): void {
        privateRequest<DeltaStatic>("getStudy",{studyId: studyId})
        .then((response: DeltaStatic) => {
            editor.setContents(response);
        }).catch((error) => {
             errorMessage.set(error);
        });
    }

    interface EmbeddedInstance {
        ticker: string;
        timeframe: string;
        datetime: string;
        pm: boolean;
    }

    function insertInstance(): void {
        const range = editor.getSelection()
        let insertIndex;
        if (range === null){
            insertIndex = -1;
        }else{
            insertIndex = range.index
        }
        editor.insertEmbed(insertIndex, 'embeddedInstance', {ticker, timeframe, datetime, pm});
    }

    function embeddedInstanceClick(instance: EmbeddedInstance): void {
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
                static create(instance: EmbeddedInstance): HTMLElement {
                    let node = super.create();
                    node.setAttribute('type', 'button');
                    node.className = 'btn';
                    node.textContent = `${instance.ticker}`; 
                    node.onclick = () => embeddedInstanceClick(instance)                    
                    return node;
                }

                static value(node: HTMLElement) {
                    return {
                        ticker: node.dataset.ticker,
                        timeframe: node.dataset.timeframe,
                        datetime: node.dataset.datetime,
                        pm: node.dataset.pm
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
<button on:click={() => {insertInstanceVisible = true} }> Insert Instance </button>
{#if insertInstanceVisible}
    <div class="form" >
        <div>
            <input bind:value={ticker} placeholder="ticker"/>
        </div>
        <div>
            <input type="date" bind:value={datetime} placeholder="datetime"/>
        </div>
        <div>
            <input  bind:value={timeframe} placeholder="timeframe"/>
        </div>
        <div>
            <button on:click={insertInstance}> enter </button>
        </div>
    </div>
{/if}
<div>
    <button on:click={() => {console.log(editor.getContents())}}> save </button>
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
