<script lang="ts">
  /*
    QueryBuilder.svelte — graphical builder for the new strategy “Spec” DSL defined in Go
    -------------------------------------------------------------------------------
    ‣ Author: ChatGPT (o3)
    ‣ Purpose: let users create/edit a Spec JSON interactively.
    ‣ Usage: <QueryBuilder bind:value={spec} /> — two‑way bind to the Spec object.
  */

  import { writable, get } from 'svelte/store';
  import { createEventDispatcher } from 'svelte';

  // ────────────────────────────────────────────────────────────────────────────
  //                TS types mirroring Go (partial / optional)
  // ────────────────────────────────────────────────────────────────────────────
  type NodeID = number;

  // --- value nodes ----------------------------------------------------------
  interface ConstNode    { kind: 'const';  number?: number; }
  interface ColumnNode   { kind: 'column'; name?: string;  }
  interface ExprNode     { kind: 'expr';   op?: ArithOp;  args: NodeID[]; }
  interface AggregateNode{ kind: 'agg';    fn?: AggFn; of?: NodeID; scope?: string; period?: number; }

  // --- boolean nodes --------------------------------------------------------
  interface ComparisonNode { op?: CompOp; lhs?: NodeID; rhs?: NodeID; kind: 'comparison'; }
  interface RankFilterNode { fn?: RankFn; expr?: NodeID; param?: number; kind: 'rank'; }
  interface LogicNode      { op?: LogicOp; args: NodeID[]; kind: 'logic'; }

  type ValueNode   = ConstNode | ColumnNode | ExprNode | AggregateNode;
  type BooleanNode = ComparisonNode | RankFilterNode | LogicNode;
  export type AnyNode    = ValueNode | BooleanNode;

  // --- enums mirrored -------------------------------------------------------
  const arithOps  = ['+', '-', '*', '/', 'offset'] as const;
  type ArithOp    = typeof arithOps[number];

  const compOps   = ['==','!=','<','<=','>','>='] as const;
  type CompOp     = typeof compOps[number];

  const aggFns    = ['avg','stdev','median'] as const;
  type AggFn      = typeof aggFns[number];

  const rankFns   = ['top_pct','bottom_pct','top_n','bottom_n'] as const;
  type RankFn     = typeof rankFns[number];

  const logicOps  = ['AND','OR','NOT'] as const;
  type LogicOp    = typeof logicOps[number];

  // --- exposed component API -----------------------------------------------
  export interface Spec {
    name: string;
    nodes: AnyNode[];
    root: NodeID | null;
  }

  export let value: Spec = { name: '', nodes: [], root: null };
  const dispatch = createEventDispatcher<{ change: Spec }>();

  // local store mirroring `value` to leverage Svelte reactivity on deep edits
  const nodes = writable<AnyNode[]>(value.nodes);
  const specName = writable(value.name);
  const root = writable<NodeID | null>(value.root ?? null);

  // UI state ---------------------------------------------------------------
  let showAddMenu = false;   // controls the “Add Node” pop‑over

  // keep parent in‑sync -----------------------------------------------------
  $: value = { name: get(specName), nodes: get(nodes), root: get(root) };
  $: dispatch('change', value);

  // helpers ------------------------------------------------------------------
  function addNode(kind: AnyNode['kind']) {
    const arr = get(nodes);
    let newNode: AnyNode;
    switch (kind) {
      case 'const':      newNode = { kind:'const',  number:0 }; break;
      case 'column':     newNode = { kind:'column', name:'' }; break;
      case 'expr':       newNode = { kind:'expr',   op:'+', args:[] }; break;
      case 'agg':        newNode = { kind:'agg',    fn:'avg', of:0, scope:'self', period:0 }; break;
      case 'comparison': newNode = { kind:'comparison', op:'>', lhs:0, rhs:0 }; break;
      case 'rank':       newNode = { kind:'rank', fn:'top_pct', expr:0, param:10 }; break;
      case 'logic':      newNode = { kind:'logic', op:'AND', args:[] }; break;
      default:           return;
    }
    nodes.set([...arr, newNode]);
  }

  function deleteNode(idx: number) {
    const arr = get(nodes);
    arr.splice(idx,1);
    // fix references ↓
    arr.forEach((n)=>updateRefs(n,idx));
    nodes.set(arr);
    // root update
    if (get(root) !== null) {
      const r = get(root)!;
      if (r === idx) root.set(null);
      else if (r > idx) root.set(r - 1);
    }
  }

  function updateRefs(n: AnyNode, removedIdx: number) {
    const dec = (id?: number)=> (id!==undefined && id>removedIdx)? id-1 : id;
    switch(n.kind){
      case 'expr': n.args = n.args.map(dec as (id:number)=>number); break;
      case 'agg':  n.of = dec(n.of); break;
      case 'comparison': n.lhs = dec(n.lhs); n.rhs = dec(n.rhs); break;
      case 'rank': n.expr = dec(n.expr); break;
      case 'logic': n.args = n.args.map(dec as (id:number)=>number); break;
    }
  }

  // Helper to ensure store updates are propagated when directly mutating node properties
  function bump() {
    nodes.update(arr => arr); // same array reference, but re-emits to trigger reactivity
  }

  // Helper for immutable node updates
  function mutate(idx: number, fn: (n: AnyNode) => AnyNode) {
    nodes.update(arr => {
      const next = [...arr];
      next[idx] = fn(structuredClone(arr[idx])); // safe copy
      return next;
    });
  }

  // Download JSON to file
  function downloadJSON() {
    const jsonStr = JSON.stringify(value, null, 2);
    const blob = new Blob([jsonStr], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    
    const a = document.createElement('a');
    a.href = url;
    a.download = `${value.name || 'strategy'}_spec.json`;
    document.body.appendChild(a);
    a.click();
    
    // Cleanup
    setTimeout(() => {
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    }, 100);
  }

  const availableNodeIDs = () => Array.from(get(nodes).keys());
  const boolNodeIDs = () => get(nodes).flatMap((n,i)=>(n.kind==='comparison'||n.kind==='rank'||n.kind==='logic')? [i] : []);
  const isRootNode = (idx: number) => get(root) === idx;
</script>

<!-- ────────────────────────────────── UI layout ────────────────────────────────── -->
<div class="qb-root">
  <label class="name-input">Spec Name
    <input bind:value={$specName} placeholder="My Strategy" />
  </label>

  <section>
    <h3>Nodes <small>({$nodes.length})</small></h3>
    <button class="add-btn" on:click={() => showAddMenu = !showAddMenu}>＋ Add Node</button>
    {#if showAddMenu}
      <div class="add-menu">
        {#each ['const','column','expr','agg','comparison','rank','logic'] as k}
          <button on:click={() => { addNode(k); showAddMenu=false; }}>{k}</button>
        {/each}
      </div>
    {/if}

    {#if $nodes.length === 0}
      <p class="dim">No nodes yet. Add one to start.</p>
    {:else}
      <div class="node-list">
        {#each $nodes as n, idx (idx)}
          <div class="node-card" class:root-node={isRootNode(idx)}>
            <header>
              <strong>#{idx}</strong> <span class="tag">{n.kind}</span>
              <button class="delete" on:click={() => deleteNode(idx)}>✕</button>
            </header>

            {#if n.kind === 'const'}
              <label>Number <input type="number" bind:value={(n).number} /></label>
            {:else if n.kind === 'column'}
              <label>Column Name <input bind:value={(n).name} placeholder="close" /></label>
            {:else if n.kind === 'expr'}
              <label>Operator
                <select bind:value={(n).op}>
                  {#each arithOps as op}<option value={op}>{op}</option>{/each}
                </select>
              </label>
              <div class="arg-list">
                <h4>Args</h4>
                {#each (n).args as id, i (i)}
                  <span class="pill" on:click={() => (n).args.splice(i,1)}>{id} ✕</span>
                {/each}
                <select on:change={(e)=>{ const v=parseInt((e.target).value); if(!isNaN(v)){ (n).args.push(v); (e.target).selectedIndex=0; }}}>
                  <option disabled selected>Add…</option>
                  {#each availableNodeIDs() as id}
                    {#if id !== idx}<option value={id}>{id}</option>{/if}
                  {/each}
                </select>
              </div>
            {:else if n.kind === 'agg'}
              <label>Function
                <select bind:value={(n).fn}>
                  {#each aggFns as fn}<option value={fn}>{fn}</option>{/each}
                </select>
              </label>
              <label>Of Node
                <select bind:value={(n).of}>{#each availableNodeIDs() as id}<option value={id}>{id}</option>{/each}</select>
              </label>
              <label>Scope <input bind:value={(n).scope} placeholder="self/sector/market" /></label>
              <label>Period (bars) <input type="number" min="0" bind:value={(n).period} /></label>
            {:else if n.kind === 'comparison'}
              <label>Op
                <select bind:value={(n).op}>{#each compOps as op}<option value={op}>{op}</option>{/each}</select>
              </label>
              <label>LHS
                <select bind:value={(n).lhs}>{#each availableNodeIDs() as id}<option value={id}>{id}</option>{/each}</select>
              </label>
              <label>RHS

                <select bind:value={(n).rhs}>{#each availableNodeIDs() as id}<option value={id}>{id}</option>{/each}</select>
              </label>
            {:else if n.kind === 'rank'}
              <label>Fn
                <select bind:value={(n).fn}>{#each rankFns as fn}<option value={fn}>{fn}</option>{/each}</select>
              </label>
              <label>Expr Node
                <select bind:value={(n).expr}>{#each availableNodeIDs() as id}<option value={id}>{id}</option>{/each}</select>
              </label>
              <label>Param <input type="number" min="1" bind:value={(n).param} /></label>
            {:else if n.kind === 'logic'}
              <label>Op
                <select bind:value={(n ).op}>{#each logicOps as op}<option value={op}>{op}</option>{/each}</select>
              </label>
              <div class="arg-list">
                <h4>Args</h4>
                {#each (n ).args as id, i (i)}
                  <span class="pill" on:click={() => (n ).args.splice(i,1)}>{id} ✕</span>
                {/each}
                <select on:change={(e)=>{ const v=parseInt(e.target.value); if(!isNaN(v)){ (n).args.push(v); e.target.selectedIndex=0; }}}>
                  <option disabled selected>Add…</option>
                  {#each boolNodeIDs() as id}
                    {#if id !== idx}<option value={id}>{id}</option>{/if}
                  {/each}
                </select>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <!-- Root selector -->
  <section>
    <h3>Root Node</h3>
    <select bind:value={$root}>
      <option value={null}>— choose —</option>
      {#each boolNodeIDs() as id}<option value={id}>{id}</option>{/each}
    </select>
  </section>

  <!-- JSON preview & export -->
  <section>
    <h3>Spec JSON</h3>
    <div class="json-actions">
      <button class="download-btn" on:click={downloadJSON}>Download JSON</button>
    </div>
    <textarea readonly rows="10">{JSON.stringify(value, null, 2)}</textarea>
  </section>
</div>

<style>
  /*
    All colors now come from the global design‑system CSS variables declared
    on :root.  This keeps the component consistent with the site‑wide theme and
    avoids the previous high‑contrast black/white look.
  */

  .qb-root{
    font-family:system-ui;
    display:flex;
    flex-direction:column;
    gap:1rem;
    color:var(--text-primary);
  }

  label{
    display:flex;
    flex-direction:column;
    font-size:0.85rem;
    margin-bottom:0.25rem;
    color:var(--text-secondary);
  }

  input,select,textarea{
    font:inherit;
    padding:4px;
    background:var(--ui-bg-element);
    border:1px solid var(--ui-border);
    border-radius:4px;
    color:var(--text-primary);
  }
  input::placeholder, textarea::placeholder{ color:var(--text-secondary); }

  .name-input{ max-width:18rem; }

  h3{
    margin:0 0 0.25rem 0;
    font-size:1rem;
    color:var(--text-primary);
  }

  .node-list{ display:flex; flex-wrap:wrap; gap:0.75rem; }

  .node-card{
    background:var(--ui-bg-secondary);
    border:1px solid var(--ui-border);
    border-radius:6px;
    padding:0.5rem;
    min-width:220px;
    position:relative;
  }

  .node-card header{ display:flex; align-items:center; gap:0.5rem; margin-bottom:0.5rem; }

  .tag{
    background:var(--ui-bg-hover);
    border-radius:4px;
    padding:0 4px;
    font-size:0.7rem;
    color:var(--text-secondary);
  }

  .delete{
    position:absolute;
    top:4px;
    right:4px;
    background:none;
    border:none;
    cursor:pointer;
    color:var(--color-down);
    font-size:0.9rem;
  }
  .delete:hover{ color:var(--color-down-strong); }

  .add-btn{
    align-self:flex-start;
    background:var(--ui-accent);
    color:var(--text-primary);
    border:none;
    padding:4px 8px;
    border-radius:4px;
    cursor:pointer;
  }
  .add-btn:hover{ background:var(--c3-hover); }

  .add-menu{ display:flex; gap:4px; margin:4px 0 8px 0; }

  .add-menu button{
    padding:2px 6px;
    font-size:0.75rem;
    background:var(--ui-bg-hover);
    border:1px solid var(--ui-border);
    border-radius:4px;
    color:var(--text-primary);
    cursor:pointer;
  }
  .add-menu button:hover{ background:var(--ui-bg-element); }

  .arg-list h4{ margin:0; font-size:0.75rem; color:var(--text-secondary); }

  .pill{
    background:var(--ui-bg-hover);
    border-radius:12px;
    padding:2px 8px;
    font-size:0.75rem;
    margin-right:4px;
    cursor:pointer;
  }
  .pill:hover{ background:var(--ui-accent); color:#fff; }

  textarea{
    width:100%;
    font-family:monospace;
    background:var(--ui-bg-element);
    border:1px solid var(--ui-border);
    color:var(--text-primary);
  }

  .dim{ opacity:0.6; color:var(--text-secondary); }

  .root-node {
    border: 2px solid var(--color-up);
    box-shadow: 0 0 4px rgba(var(--color-up-rgb), 0.3);
  }

  .json-actions {
    display: flex;
    justify-content: flex-end;
    margin-bottom: 0.5rem;
  }

  .download-btn {
    background: var(--ui-accent);
    color: var(--text-primary);
    border: none;
    padding: 4px 8px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  
  .download-btn:hover {
    background: var(--c3-hover);
  }
</style>

