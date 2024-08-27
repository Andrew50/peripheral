<script lang="ts">
    import { onMount } from 'svelte';
    import { writable } from 'svelte/store'
    import { browser } from '$app/environment';
    import 'quill/dist/quill.snow.css';
    import type Quill from 'quill'
    import type { DeltaStatic, Sources, DeltaOperation, EmbedBlot } from 'quill'

    let Quill;
    let editorContainer: HTMLElement | string;
    let editor: Quill | undefined;
    let ticker = "";
    let timeframe = "";
    let datetime = "";
    let pm = false;
    let insertInstanceVisible = false;

    function insertInstance(): void {
        editor.insertEmbed(-1, 'embeddedInstance', {ticker, timeframe, datetime, pm});
    }

    function embeddedInstanceClick(): void {
        console.log("clicked embedded instance")
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
            editor.on('text-change', (delta: DeltaStatic, _, source: Sources) => {
                if (source === 'user' || source === 'api') {
                    let text = editor.getText();
                    /*if (source === 'user') {
                        currentEntry.set(text); 
                    }*/
                    delta.ops.forEach((op: DeltaOperation) => {
                        if (typeof op.insert === 'string') {
                            const regex = /\[([^\|]+)\|([^\|]+)\|([^\|]+)\|([^\]]+)\]/g;
                            let match;
                            while ((match = regex.exec(op.insert)) !== null) {
                                const [_, ticker, interval, t, pm] = match;
                                const index = match.index;
                                editor.deleteText(index, match[0].length);
                                editor.insertEmbed(index, 'embeddedInstance', { ticker, interval, t, pm });
                            }
                        }
                    });
                }
            });
            class ChartBlot extends (Quill.import('blots/embed') as typeof EmbedBlot) {
                static create(value: any): HTMLElement {
                    let node = super.create();
                    node.setAttribute('type', 'button');
                    node.className = 'btn';
                    node.textContent = `${value.ticker}`; 
                    node.onclick = embeddedInstanceClick//() => chartQuery.set([value.ticker, value.interval, value.t, value.pm]);
                    return node;
                }

                static value(node: HTMLElement) {
                    return {
                        ticker: node.dataset.ticker,
                        interval: node.dataset.interval,
                        t: node.dataset.t,
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
