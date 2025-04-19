<script lang="ts">
  import { onMount } from 'svelte';
  import { writable, get } from 'svelte/store';
  import { strategies } from '$lib/core/stores';
  import { privateRequest } from '$lib/core/backend';
  import '$lib/core/global.css';

  /***********************
   *     ─ Types ─       *
   ***********************/
  type StrategyId = number | 'new' | null;

  /**  ───────── High‑level record saved to DB ───────── */
  interface Strategy {
    strategyId: number;
    name: string;
    spec: StrategySpec;
    score?: number;
  }

  /**  ───────── Local editable copy ───────── */
  interface EditableStrategy extends Strategy {
    strategyId: StrategyId;
  }

  /**  ───────── PARSED STRATEGY SPEC ───────── */
  interface StrategySpec {
    timeframes: string[];
    stocks: {
      universe: string;
      include: string[];
      exclude: string[];
      filters: StockFilter[];
    };
    indicators: Indicator[];
    derived_columns: DerivedColumn[];
    future_performance: FuturePerf[];
    conditions: Condition[];
    logic: string;
    date_range: { start: string; end: string };
    time_of_day: { constraint: string; start_time: string; end_time: string };
    output_columns: string[];
  }

  interface StockFilter {
    metric: string;
    operator: string;
    value: number;
    timeframe: string;
  }

  interface Indicator {
    id: string;
    type: string;
    parameters: Record<string, any>;
    input_field: string;
    timeframe: string;
  }

  interface DerivedColumn {
    id: string;
    expression: string;
    comment?: string;
  }

  interface FuturePerf {
    id: string;
    expression: string;
    timeframe: string;
    comment?: string;
  }

  interface ConditionLHS {
    field: string;
    offset: number;
    timeframe: string;
  }
  interface ConditionRHS {
    field?: string;
    offset?: number;
    timeframe?: string;
    indicator_id?: string;
    value?: number;
    multiplier?: number;
  }
  interface Condition {
    id?: string;
    lhs: ConditionLHS;
    operation: string;
    rhs: ConditionRHS;
  }

  /***********************
   *   ─ Component State ─
   ***********************/
  const loading = writable(false);
  let selectedStrategyId: StrategyId = null;
  let editedStrategy: EditableStrategy | null = null;

  /***********************
   *   ─ Helpers ─
   ***********************/
  function blankSpec(): StrategySpec {
    return {
      timeframes: ['daily'],
      stocks: {
        universe: 'all',
        include: [],
        exclude: [],
        filters: []
      },
      indicators: [],
      derived_columns: [],
      future_performance: [],
      conditions: [],
      logic: 'AND',
      date_range: { start: '', end: '' },
      time_of_day: { constraint: '', start_time: '', end_time: '' },
      output_columns: []
    };
  }

  async function loadStrategies() {
    loading.set(true);
    try {
      const data = await privateRequest<Strategy[]>('getStrategies', {});
      // if old backend returns criteria, map → spec placeholder
      if (Array.isArray(data) && data.length){
      data.forEach((d: any) => {
        if (!('spec' in d)) {
          d.spec = blankSpec();
        }
      });
      }
      strategies.set(data);
    } finally {
      loading.set(false);
    }
  }
  onMount(loadStrategies);

  /***********************
   *       ─ CRUD ─
   ***********************/
  function startCreate() {
    editedStrategy = {
      strategyId: 'new',
      name: '',
      spec: blankSpec()
    } as EditableStrategy;
    selectedStrategyId = 'new';
  }

  function editStrategy(strat: Strategy) {
    selectedStrategyId = strat.strategyId;
    editedStrategy = JSON.parse(JSON.stringify(strat)); // deep clone
  }

  function cancelEdit() {
    selectedStrategyId = null;
    editedStrategy = null;
  }

  async function deleteStrategy(id: number) {
    if (!confirm('Delete this strategy?')) return;
    await privateRequest('deleteStrategy', { strategyId: id });
    strategies.update(arr => arr.filter(s => s.strategyId !== id));
    cancelEdit();
  }

  async function saveStrategy() {
    if (!editedStrategy) return;

    const payload = {
      name: editedStrategy.name,
      spec: editedStrategy.spec
    };

    if (editedStrategy.strategyId === 'new') {
      const created = await privateRequest<Strategy>('newStrategy', payload);
      strategies.update(arr => [...arr, created]);
    } else if (typeof editedStrategy.strategyId === 'number') {
      await privateRequest('setStrategy', {
        strategyId: editedStrategy.strategyId,
        ...payload
      });
      strategies.update(arr =>
        arr.map(s => (s.strategyId === editedStrategy!.strategyId ? { ...(editedStrategy as Strategy) } : s))
      );
    }

    cancelEdit();
  }

  /***********************
   *   ─ Utility funcs ─
   ***********************/
  const timeframeOptions = ['daily', 'hourly', 'minute'];
  const stockMetricOptions = ['dolvol', 'adr', 'mcap'];
  const operatorOptions = ['>', '<', '>=', '<=', '==', '!='];
</script>

{#if selectedStrategyId === null}
  <!-- List View -->
  <div class="toolbar">
    <button on:click={startCreate}>＋ New Strategy</button>
  </div>

  {#if $loading}
    <p>Loading…</p>
  {:else}
    <div class="table-container">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Timeframes</th>
            <th>Score</th>
            <th style="width:8rem">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#if Array.isArray($strategies) && $strategies.length}
            {#each $strategies as s}
              <tr>
                <td>{s.name}</td>
                <!--<td>{s.spec.timeframes?.join(', ')}</td>-->
                <td>{s.score ?? '—'}</td>
                <td>
                  <button on:click={() => editStrategy(s)}>Edit</button>
                  <button class="danger" on:click={() => deleteStrategy(s.strategyId)}>Delete</button>
                </td>
              </tr>
            {/each}
          {:else}
            <tr><td colspan="4">No strategies yet.</td></tr>
          {/if}
        </tbody>
      </table>
    </div>
  {/if}
{:else if editedStrategy}
  <!-- Edit / Create View -->
  <div class="form-block">
    <label>
      Name
      <input type="text" bind:value={editedStrategy.name} placeholder="Strategy name" />
    </label>
  </div>

  <!-- Timeframes section -->
  <fieldset class="section">
    <legend>Timeframes</legend>
    {#each timeframeOptions as tf}
      <label class="inline">
      <input type="checkbox" value={tf} 
      checked={editedStrategy.spec.timeframes.includes(tf)} 
      on:change={(e) => {
          const checked = e.currentTarget.checked;
        const arr = editedStrategy?.spec.timeframes;
        if (checked) arr.push(tf); 
        else {
            editedStrategy.spec.timeframes = arr.filter(v => v !== tf);
        }
      }}> {tf}</label>
    {/each}
  </fieldset>

  <!-- Stocks section -->
  <fieldset class="section">
    <legend>Stocks Universe</legend>

    <label>
      Universe
      <select bind:value={editedStrategy.spec.stocks.universe}>
        <option value="all">All</option>
        <option value="list">Custom List</option>
        <option value="sp500">S&P 500</option>
      </select>
    </label>

    <div class="flex flex-space">
      <div class="pill-group">
        <h4>Include</h4>
        {#each editedStrategy.spec.stocks.include as ticker, i (ticker)}
          <span class="pill" on:click={() => editedStrategy?.spec.stocks.include.splice(i, 1)}>{ticker} ✕</span>
        {/each}
        <input class="small" placeholder="Ticker" on:keydown={(e) => {
          if (e.key === 'Enter' && e.target.value.trim()) {
            editedStrategy.spec.stocks.include.push(e.target.value.trim().toUpperCase());
            e.target.value = '';
          }
        }} />
      </div>

      <div class="pill-group">
        <h4>Exclude</h4>
        {#each editedStrategy.spec.stocks.exclude as ticker, i (ticker)}
          <span class="pill" on:click={() => editedStrategy?.spec.stocks.exclude.splice(i, 1)}>{ticker} ✕</span>
        {/each}
        <input class="small" placeholder="Ticker" on:keydown={(e) => {
          if (e.key === 'Enter' && e.target.value.trim()) {
            editedStrategy?.spec.stocks.exclude.push(e.target.value.trim().toUpperCase());
            e.target.value = '';
          }
        }} />
      </div>
    </div>

    <details>
      <summary>Filters</summary>
      <table class="mini">
        <thead>
          <tr><th>Metric</th><th>Operator</th><th>Value</th><th>Timeframe</th><th></th></tr>
        </thead>
        <tbody>
          {#each editedStrategy.spec.stocks.filters as f, i (i)}
            <tr>
              <td>
                <select bind:value={f.metric}>{#each stockMetricOptions as m}<option value={m}>{m}</option>{/each}</select>
              </td>
              <td>
                <select bind:value={f.operator}>{#each operatorOptions as op}<option value={op}>{op}</option>{/each}</select>
              </td>
              <td><input type="number" step="any" bind:value={f.value} /></td>
              <td><input type="text" class="tiny" bind:value={f.timeframe} /></td>
              <td><button on:click={() => editedStrategy?.spec.stocks.filters.splice(i, 1)}>✕</button></td>
            </tr>
          {/each}
        </tbody>
      </table>
      <button on:click={() => editedStrategy?.spec.stocks.filters.push({metric:'dolvol', operator:'>', value:0, timeframe:'daily'})}>＋ Add Filter</button>
    </details>
  </fieldset>

  <!-- Conditions section -->
  <fieldset class="section">
    <legend>Conditions</legend>
    {#each editedStrategy.spec.conditions as c, idx (idx)}
      <div class="condition-row">
        <input class="tiny" placeholder="field" bind:value={c.lhs.field} />
        <input class="tiny" type="number" bind:value={c.lhs.offset} placeholder="offset" />
        <input class="tiny" placeholder="tf" bind:value={c.lhs.timeframe} />

        <select bind:value={c.operation}>{#each operatorOptions as op}<option value={op}>{op}</option>{/each}</select>

        <input class="tiny" placeholder="value / field / id" 
        bind:value={c.rhs.value} 
        on:input={(e)=>{
            const v=parseFloat(e.target.value); 
        if (!isNaN(v)) c.rhs.value=v}} />

        <button on:click={() => editedStrategy?.spec.conditions.splice(idx,1)}>✕</button>
      </div>
    {/each}
    <button on:click={() => editedStrategy?.spec.conditions.push({lhs:{field:'',offset:0,timeframe:'daily'},operation:'>',rhs:{value:0}})}>＋ Add Condition</button>
  </fieldset>

  <!-- Output columns section -->
  <fieldset class="section">
    <legend>Output Columns</legend>
    <div class="pill-group">
      {#each editedStrategy.spec.output_columns as col, i (col)}
        <span class="pill" on:click={() => editedStrategy?.spec.output_columns.splice(i,1)}>{col} ✕</span>
      {/each}
      <input class="small" placeholder="column" on:keydown={(e)=>{if(e.key==='Enter' && e.target.value.trim()){editedStrategy?.spec.output_columns.push(e.target.value.trim()); e.target.value='';}}} />
    </div>
  </fieldset>

  <!-- Save / Cancel -->
  <div class="actions">
    <button on:click={saveStrategy}>💾 Save</button>
    <button on:click={cancelEdit}>Cancel</button>
    {#if editedStrategy.strategyId !== 'new'}
      <button class="danger" on:click={() => deleteStrategy(editedStrategy?.strategyId)}>Delete</button>
    {/if}
  </div>
{/if}

<style>
  /*  ─── Layout & Typography ─── */
  .toolbar { margin-bottom: 0.75rem; }
  .table-container { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: 0.5rem 0.75rem; border-bottom: 1px solid #e2e8f0; text-align: left; }
  th { font-weight: 600; }

  /*  ─── Form Sections ─── */
  .form-block { margin-bottom: 1rem; }
  fieldset.section { border: 1px solid #cbd5e1; border-radius: 4px; padding: 0.75rem; margin-bottom: 1rem; }
  legend { padding: 0 8px; font-weight: 600; }

  label { display: flex; flex-direction: column; font-size: 0.9rem; margin-bottom: 0.5rem; }
  input, select { padding: 0.35rem 0.5rem; border: 1px solid #cbd5e1; border-radius: 4px; font-size: 0.9rem; }
  input.small { width: 6rem; }
  input.tiny { width: 4.5rem; }

  details > summary { cursor: pointer; margin-bottom: 0.5rem; font-weight: 500; }


  .mini { width: 100%; margin-bottom: 0.5rem; }
  .mini td, .mini th { padding: 0.3rem 0.4rem; }
  .mini input, .mini select { width: 100%; font-size: 0.8rem; }

  /*  ─── Pills & Flex helpers ─── */
  .pill-group { display: flex; flex-wrap: wrap; gap: 4px; align-items: center; }
  .pill { background: #e2e8f0; border-radius: 12px; padding: 2px 8px; font-size: 0.8rem; cursor: pointer; user-select: none; }
  .pill:hover { background: #cbd5e1; }

  .flex { display: flex; gap: 1rem; }
  .flex-space { justify-content: space-between; }

  .condition-row { display: flex; gap: 4px; align-items: center; margin-bottom: 4px; }

  /*  ─── Buttons ─── */
  button { padding: 0.35rem 0.75rem; border: 1px solid #94a3b8; background: #f8fafc; border-radius: 4px; font-size: 0.9rem; cursor: pointer; transition: background 120ms; }
  button:hover { background: #e2e8f0; }
  button.danger { color: #b91c1c; border-color: #b91c1c; }
  button.danger:hover { background: #fecaca; }
  button:disabled { opacity: 0.6; cursor: not-allowed; }

  .actions { margin-top: 0.75rem; }
</style>

