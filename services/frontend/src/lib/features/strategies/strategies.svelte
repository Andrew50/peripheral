<script lang="ts">
  import { onMount } from 'svelte';
  import { writable, get, derived } from 'svelte/store';
  import { strategies } from '$lib/core/stores';
  import { privateRequest } from '$lib/core/backend';
  import '$lib/core/global.css';

  type StrategyId = number | 'new' | null;

  interface Strategy {
    strategyId: number;
    name: string;
    spec: StrategySpec;
    score?: number;
  }

  interface EditableStrategy extends Strategy {
    strategyId: StrategyId;
  }

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

  // Create stores
  const loading = writable(false);
  const selectedStrategyId = writable<StrategyId>(null);
  const editedStrategy = writable<EditableStrategy | null>(null);

  // Help text for various fields
  const helpText = {
    timeframes: "Select one or more timeframes needed for your strategy (e.g. daily, hourly, minute)",
    stockUniverse: "Initial pool of stocks to consider (all stocks, custom list, or S&P 500)",
    stockInclude: "Enter specific tickers to include (e.g. AAPL, MSFT, GOOG). Press Enter after each ticker.",
    stockExclude: "Enter specific tickers to exclude (e.g. TSLA, META). Press Enter after each ticker.",
    stockFilters: "Add filters to narrow down stocks before applying conditions (e.g. volume > 1,000,000)",
    indicators: "Technical indicators used in your strategy (e.g. SMA, EMA, VWAP)",
    conditions: "Rules that must be met for a stock to trigger your strategy",
    outputColumns: "Select data columns to include in your results. Press Enter after each column."
  };

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
      logic: 'and',
      date_range: { start: '', end: '' },
      time_of_day: { constraint: '', start_time: '', end_time: '' },
      output_columns: []
    };
  }

  function ensureValidSpec(spec: any): StrategySpec {
    // Start with a blank spec to ensure all required fields are present
    const validSpec = blankSpec();
    
    if (!spec) return validSpec;
    
    // Copy over all valid fields from the provided spec
    if (Array.isArray(spec.timeframes)) validSpec.timeframes = spec.timeframes;
    
    // Handle stocks object
    if (spec.stocks) {
      if (typeof spec.stocks.universe === 'string') validSpec.stocks.universe = spec.stocks.universe;
      if (Array.isArray(spec.stocks.include)) validSpec.stocks.include = spec.stocks.include;
      if (Array.isArray(spec.stocks.exclude)) validSpec.stocks.exclude = spec.stocks.exclude;
      if (Array.isArray(spec.stocks.filters)) validSpec.stocks.filters = spec.stocks.filters;
    }
    
    // Handle other arrays and objects
    if (Array.isArray(spec.indicators)) validSpec.indicators = spec.indicators;
    if (Array.isArray(spec.derived_columns)) validSpec.derived_columns = spec.derived_columns;
    if (Array.isArray(spec.future_performance)) validSpec.future_performance = spec.future_performance;
    if (Array.isArray(spec.conditions)) validSpec.conditions = spec.conditions;
    if (Array.isArray(spec.output_columns)) validSpec.output_columns = spec.output_columns;
    
    // Handle string fields
    if (typeof spec.logic === 'string') validSpec.logic = spec.logic;
    
    // Handle nested objects
    if (spec.date_range) {
      if (typeof spec.date_range.start === 'string') validSpec.date_range.start = spec.date_range.start;
      if (typeof spec.date_range.end === 'string') validSpec.date_range.end = spec.date_range.end;
    }
    
    if (spec.time_of_day) {
      if (typeof spec.time_of_day.constraint === 'string') validSpec.time_of_day.constraint = spec.time_of_day.constraint;
      if (typeof spec.time_of_day.start_time === 'string') validSpec.time_of_day.start_time = spec.time_of_day.start_time;
      if (typeof spec.time_of_day.end_time === 'string') validSpec.time_of_day.end_time = spec.time_of_day.end_time;
    }
    
    return validSpec;
  }

  async function loadStrategies() {
    loading.set(true);
    try {
      const data = await privateRequest<Strategy[]>('getStrategies', {});
      if (Array.isArray(data) && data.length) {
        data.forEach((d: any) => {
          if (!d.spec) {
            d.spec = blankSpec();
          } else {
            // Ensure the spec is valid
            d.spec = ensureValidSpec(d.spec);
          }
        });
      }
      strategies.set(data);
    } catch (error) {
      console.error("Error loading strategies:", error);
    } finally {
      loading.set(false);
    }
  }
  
  onMount(loadStrategies);

  function startCreate() {
    const newStrategy = {
      strategyId: 'new',
      name: '',
      spec: blankSpec()
    } as EditableStrategy;
    
    editedStrategy.set(newStrategy);
    selectedStrategyId.set('new');
  }

  function editStrategy(strat: Strategy) {
    selectedStrategyId.set(strat.strategyId);
    console.log(strat.spec)
    
    // Make sure the spec is valid and complete
    const validSpec = ensureValidSpec(strat.spec);
    
    // Deep clone to avoid modifying original data
    const strategyToEdit = {
      ...JSON.parse(JSON.stringify(strat)),
      spec: validSpec
    };
    
    editedStrategy.set(strategyToEdit);
  }

  function cancelEdit() {
    selectedStrategyId.set(null);
    editedStrategy.set(null);
  }

  async function deleteStrategy(id: number) {
    if (!confirm('Delete this strategy?')) return;
    try {
      await privateRequest('deleteStrategy', { strategyId: id });
      strategies.update(arr => arr.filter(s => s.strategyId !== id));
      cancelEdit();
    } catch (error) {
      console.error("Error deleting strategy:", error);
      alert("Failed to delete strategy");
    }
  }

  async function saveStrategy() {
    const currentStrategy = get(editedStrategy);
    if (!currentStrategy) return;

    const payload = {
      name: currentStrategy.name,
      spec: currentStrategy.spec
    };

    try {
      if (currentStrategy.strategyId === 'new') {
        const created = await privateRequest<Strategy>('newStrategy', payload);
        strategies.update(arr => [...arr, created]);
      } else if (typeof currentStrategy.strategyId === 'number') {
        await privateRequest('setStrategy', {
          strategyId: currentStrategy.strategyId,
          ...payload
        });
        strategies.update(arr =>
          arr.map(s => (s.strategyId === currentStrategy.strategyId ? { ...currentStrategy as Strategy } : s))
        );
      }
      cancelEdit();
    } catch (error) {
      console.error("Error saving strategy:", error);
      alert("Failed to save strategy");
    }
  }

  // Helper functions for updating the editedStrategy store
  function updateEditedStrategy(updater: (strategy: EditableStrategy) => void) {
    editedStrategy.update(strategy => {
      if (!strategy) return null;
      const clone = JSON.parse(JSON.stringify(strategy));
      updater(clone);
      return clone;
    });
  }

  function addTimeframe(tf: string) {
    updateEditedStrategy(strategy => {
      if (!strategy.spec.timeframes.includes(tf)) {
        strategy.spec.timeframes.push(tf);
      }
    });
  }

  function removeTimeframe(tf: string) {
    updateEditedStrategy(strategy => {
      strategy.spec.timeframes = strategy.spec.timeframes.filter(v => v !== tf);
    });
  }

  function addStockInclude(ticker: string) {
    if (!ticker.trim()) return;
    updateEditedStrategy(strategy => {
      strategy.spec.stocks.include.push(ticker.trim().toUpperCase());
    });
  }

  function removeStockInclude(index: number) {
    updateEditedStrategy(strategy => {
      strategy.spec.stocks.include.splice(index, 1);
    });
  }

  function addStockExclude(ticker: string) {
    if (!ticker.trim()) return;
    updateEditedStrategy(strategy => {
      strategy.spec.stocks.exclude.push(ticker.trim().toUpperCase());
    });
  }

  function removeStockExclude(index: number) {
    updateEditedStrategy(strategy => {
      strategy.spec.stocks.exclude.splice(index, 1);
    });
  }

  function addStockFilter() {
    updateEditedStrategy(strategy => {
      strategy.spec.stocks.filters.push({
        metric: 'dolvol',
        operator: '>',
        value: 0,
        timeframe: 'daily'
      });
    });
  }

  function removeStockFilter(index: number) {
    updateEditedStrategy(strategy => {
      strategy.spec.stocks.filters.splice(index, 1);
    });
  }

  function addCondition() {
    updateEditedStrategy(strategy => {
      strategy.spec.conditions.push({
        lhs: { field: '', offset: 0, timeframe: 'daily' },
        operation: '>',
        rhs: { value: 0 }
      });
    });
  }

  function removeCondition(index: number) {
    updateEditedStrategy(strategy => {
      strategy.spec.conditions.splice(index, 1);
    });
  }

  function addDerivedColumn() {
    updateEditedStrategy(strategy => {
      strategy.spec.derived_columns.push({
        id: '',
        expression: '',
        comment: ''
      });
    });
  }

  function removeDerivedColumn(index: number) {
    updateEditedStrategy(strategy => {
      strategy.spec.derived_columns.splice(index, 1);
    });
  }

  function addOutputColumn(column: string) {
    if (!column.trim()) return;
    updateEditedStrategy(strategy => {
      strategy.spec.output_columns.push(column.trim());
    });
  }

  function removeOutputColumn(index: number) {
    updateEditedStrategy(strategy => {
      strategy.spec.output_columns.splice(index, 1);
    });
  }

  const timeframeOptions = ['daily', 'hourly', 'minute'];
  const stockMetricOptions = ['dolvol', 'adr', 'mcap'];
  const operatorOptions = ['>', '<', '>=', '<=', '==', '!='];
  const availableFieldOptions = ['open', 'high', 'low', 'close', 'volume'];
</script>

<!-- The UI part remains largely unchanged, but we update bindings to use store values and add help text -->

{#if $selectedStrategyId === null}
  <!-- list view -->
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
                <td>{s.spec?.timeframes?.join(', ') || '—'}</td>
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
{:else if $editedStrategy}
  <!-- edit / create view -->
  <div class="form-block">
    <label>
      Name
      <input 
        type="text" 
        value={$editedStrategy.name} 
        placeholder="Strategy name" 
        on:input={(e) => updateEditedStrategy(s => s.name = e.currentTarget.value)}
      />
    </label>
  </div>

  <!-- timeframes section -->
  <fieldset class="section">
    <legend>Timeframes <span class="help-icon" title={helpText.timeframes}>?</span></legend>
    <p class="help-text">Select one or more timeframes needed for your strategy:</p>
    {#each timeframeOptions as tf}
      <label class="inline">
        <input 
          type="checkbox" 
          value={tf} 
          checked={$editedStrategy.spec.timeframes.includes(tf)} 
          on:change={(e) => {
            if (e.currentTarget.checked) {
              addTimeframe(tf);
            } else {
              removeTimeframe(tf);
            }
          }}
        /> 
        {tf}
      </label>
    {/each}
  </fieldset>

  <!-- stocks section -->
  <fieldset class="section">
    <legend>Stocks Universe <span class="help-icon" title={helpText.stockUniverse}>?</span></legend>

    <label>
      Universe
      <select 
        value={$editedStrategy.spec.stocks.universe} 
        on:change={(e) => updateEditedStrategy(s => s.spec.stocks.universe = e.currentTarget.value)}
      >
        <option value="all">All</option>
        <option value="list">Custom List</option>
        <option value="sp500">S&P 500</option>
      </select>
    </label>

    <div class="flex flex-space">
      <div class="pill-group">
        <h4>Include <span class="help-icon" title={helpText.stockInclude}>?</span></h4>
        <p class="help-text">Enter stock tickers (e.g., AAPL, MSFT). Press Enter after each.</p>
        {#each $editedStrategy.spec.stocks.include as ticker, i (ticker)}
          <span class="pill" on:click={() => removeStockInclude(i)}>{ticker} ✕</span>
        {/each}
        <input 
          class="small" 
          placeholder="Ticker (press Enter to add)" 
          on:keydown={(e) => {
            if (e.key === 'Enter' && e.currentTarget.value.trim()) {
              addStockInclude(e.currentTarget.value);
              e.currentTarget.value = '';
            }
          }} 
        />
      </div>

      <div class="pill-group">
        <h4>Exclude <span class="help-icon" title={helpText.stockExclude}>?</span></h4>
        <p class="help-text">Enter stock tickers to exclude. Press Enter after each.</p>
        {#each $editedStrategy.spec.stocks.exclude as ticker, i (ticker)}
          <span class="pill" on:click={() => removeStockExclude(i)}>{ticker} ✕</span>
        {/each}
        <input 
          class="small" 
          placeholder="Ticker (press Enter to add)" 
          on:keydown={(e) => {
            if (e.key === 'Enter' && e.currentTarget.value.trim()) {
              addStockExclude(e.currentTarget.value);
              e.currentTarget.value = '';
            }
          }} 
        />
      </div>
    </div>

    <details>
      <summary>Filters <span class="help-icon" title={helpText.stockFilters}>?</span></summary>
      <p class="help-text">Add filters to narrow down stocks before applying conditions (e.g., volume > 1,000,000)</p>
      <table class="mini">
        <thead>
          <tr><th>Metric</th><th>Operator</th><th>Value</th><th>Timeframe</th><th></th></tr>
        </thead>
        <tbody>
          {#each $editedStrategy.spec.stocks.filters as f, i (i)}
            <tr>
              <td>
                <select 
                  value={f.metric}
                  on:change={(e) => updateEditedStrategy(s => s.spec.stocks.filters[i].metric = e.currentTarget.value)}
                >
                  {#each stockMetricOptions as m}
                    <option value={m}>{m}</option>
                  {/each}
                </select>
              </td>
              <td>
                <select 
                  value={f.operator}
                  on:change={(e) => updateEditedStrategy(s => s.spec.stocks.filters[i].operator = e.currentTarget.value)}
                >
                  {#each operatorOptions as op}
                    <option value={op}>{op}</option>
                  {/each}
                </select>
              </td>
              <td>
                <input 
                  type="number" 
                  step="any" 
                  value={f.value}
                  on:input={(e) => updateEditedStrategy(s => s.spec.stocks.filters[i].value = parseFloat(e.currentTarget.value) || 0)}
                />
              </td>
              <td>
                <select
                  value={f.timeframe}
                  on:change={(e) => updateEditedStrategy(s => s.spec.stocks.filters[i].timeframe = e.currentTarget.value)}
                >
                  {#each timeframeOptions as tf}
                    <option value={tf}>{tf}</option>
                  {/each}
                </select>
              </td>
              <td>
                <button on:click={() => removeStockFilter(i)}>✕</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      <button on:click={addStockFilter}>＋ Add Filter</button>
    </details>
  </fieldset>

  <!-- derived columns section -->
  <fieldset class="section">
    <legend>Derived Columns</legend>
    <p class="help-text">Create custom columns based on existing data</p>
    {#each $editedStrategy.spec.derived_columns as dc, idx (idx)}
      <div class="derived-column-row">
        <label>
          Column ID
          <input 
            class="small" 
            type="text" 
            value={dc.id} 
            placeholder="unique_id" 
            on:input={(e) => updateEditedStrategy(s => s.spec.derived_columns[idx].id = e.currentTarget.value)}
          />
        </label>
        
        <label>
          Expression
          <input 
            type="text" 
            value={dc.expression} 
            placeholder="SQL expression" 
            on:input={(e) => updateEditedStrategy(s => s.spec.derived_columns[idx].expression = e.currentTarget.value)}
          />
        </label>
        
        <label>
          Comment
          <input 
            type="text" 
            value={dc.comment || ''} 
            placeholder="Description (optional)" 
            on:input={(e) => updateEditedStrategy(s => s.spec.derived_columns[idx].comment = e.currentTarget.value)}
          />
        </label>
        
        <button on:click={() => removeDerivedColumn(idx)}>✕</button>
      </div>
    {/each}
    <button on:click={addDerivedColumn}>＋ Add Derived Column</button>
  </fieldset>

  <!-- conditions section -->
  <fieldset class="section">
    <legend>Conditions <span class="help-icon" title={helpText.conditions}>?</span></legend>
    <p class="help-text">Define the rules that must be met for a stock to trigger your strategy (e.g., close > SMA50)</p>
    {#each $editedStrategy.spec.conditions as c, idx (idx)}
      <div class="condition-row">
        <select
          class="condition-field"
          value={c.lhs.field}
          on:change={(e) => updateEditedStrategy(s => s.spec.conditions[idx].lhs.field = e.currentTarget.value)}
        >
          <option value="" disabled>Select field</option>
          {#each availableFieldOptions as field}
            <option value={field}>{field}</option>
          {/each}
          <!-- Check if we have derived columns and add them as options -->
          {#if $editedStrategy.spec.derived_columns && $editedStrategy.spec.derived_columns.length > 0}
            <optgroup label="Derived Columns">
              {#each $editedStrategy.spec.derived_columns as dc}
                {#if dc.id}
                  <option value={dc.id}>{dc.id}</option>
                {/if}
              {/each}
            </optgroup>
          {/if}
        </select>
        
        <input 
          class="tiny" 
          type="number" 
          value={c.lhs.offset}
          placeholder="offset (0=current, -1=previous)" 
          title="Offset: 0 for current bar, -1 for previous bar, etc."
          on:input={(e) => updateEditedStrategy(s => s.spec.conditions[idx].lhs.offset = parseInt(e.currentTarget.value) || 0)}
        />
        
        <select 
          value={c.lhs.timeframe}
          on:change={(e) => updateEditedStrategy(s => s.spec.conditions[idx].lhs.timeframe = e.currentTarget.value)}
        >
          {#each timeframeOptions as tf}
            <option value={tf}>{tf}</option>
          {/each}
        </select>

        <select 
          value={c.operation}
          on:change={(e) => updateEditedStrategy(s => s.spec.conditions[idx].operation = e.currentTarget.value)}
        >
          {#each operatorOptions as op}
            <option value={op}>{op}</option>
          {/each}
          <option value="crosses_above">crosses above</option>
          <option value="crosses_below">crosses below</option>
        </select>

        <input 
          class="tiny" 
          placeholder="value / field / indicator id" 
          value={c.rhs.value !== undefined ? c.rhs.value : (c.rhs.field || c.rhs.indicator_id || '')}
          on:input={(e) => {
            const inputVal = e.currentTarget.value;
            const numVal = parseFloat(inputVal);
            
            updateEditedStrategy(s => {
              // If it's a number, set as value
              if (!isNaN(numVal)) {
                s.spec.conditions[idx].rhs = { value: numVal };
              } 
              // Otherwise, assume it's a field or indicator reference
              else if (availableFieldOptions.includes(inputVal)) {
                s.spec.conditions[idx].rhs = { 
                  field: inputVal,
                  offset: 0,
                  timeframe: s.spec.conditions[idx].lhs.timeframe 
                };
              } 
              else {
                s.spec.conditions[idx].rhs = { indicator_id: inputVal };
              }
            });
          }}
        />

        <button on:click={() => removeCondition(idx)}>✕</button>
      </div>
    {/each}
    <button on:click={addCondition}>＋ Add Condition</button>
    <p class="hint">Need more complex conditions? You can add custom indicators or derived columns in the advanced section.</p>
  </fieldset>

  <!-- output columns section -->
  <fieldset class="section">
    <legend>Output Columns <span class="help-icon" title={helpText.outputColumns}>?</span></legend>
    <p class="help-text">Select data columns to include in your results (e.g., open, close, volume, sma50d)</p>
    <div class="pill-group">
      {#each $editedStrategy.spec.output_columns as col, i (col)}
        <span class="pill" on:click={() => removeOutputColumn(i)}>{col} ✕</span>
      {/each}
      <input 
        class="small" 
        placeholder="Column name (press Enter to add)" 
        on:keydown={(e) => {
          if (e.key === 'Enter' && e.currentTarget.value.trim()) {
            addOutputColumn(e.currentTarget.value);
            e.currentTarget.value = '';
          }
        }}
      />
    </div>
    <div class="quick-add">
      <p>Quick add:</p>
      {#each availableFieldOptions as field}
        <button 
          class="quick-add-btn" 
          on:click={() => {
            if (!$editedStrategy.spec.output_columns.includes(field)) {
              addOutputColumn(field);
            }
          }}
        >{field}</button>
      {/each}
      <!-- Add derived columns as quick add options -->
      {#if $editedStrategy.spec.derived_columns && $editedStrategy.spec.derived_columns.length > 0}
        {#each $editedStrategy.spec.derived_columns as dc}
          {#if dc.id}
            <button 
              class="quick-add-btn" 
              on:click={() => {
                if (!$editedStrategy.spec.output_columns.includes(dc.id)) {
                  addOutputColumn(dc.id);
                }
              }}
            >{dc.id}</button>
          {/if}
        {/each}
      {/if}
    </div>
  </fieldset>

  <!-- Advanced sections (collapsible) -->
  <details class="advanced-section">
    <summary>Advanced Settings</summary>
    
    <!-- date range section -->
    <fieldset class="section">
      <legend>Date Range</legend>
      <div class="flex">
        <label>
          Start Date
          <input 
            type="date" 
            value={$editedStrategy.spec.date_range.start} 
            on:change={(e) => updateEditedStrategy(s => s.spec.date_range.start = e.currentTarget.value)}
          />
        </label>
        <label>
          End Date
          <input 
            type="date" 
            value={$editedStrategy.spec.date_range.end} 
            on:change={(e) => updateEditedStrategy(s => s.spec.date_range.end = e.currentTarget.value)}
          />
        </label>
      </div>
      <p class="hint">Leave empty to use the default range (past year to present)</p>
    </fieldset>
    
    <!-- logic section -->
    <fieldset class="section">
      <legend>Logic</legend>
      <label>
        <select
          value={$editedStrategy.spec.logic}
          on:change={(e) => updateEditedStrategy(s => s.spec.logic = e.currentTarget.value)}
        >
          <option value="and">AND - All conditions must be true</option>
          <option value="or">OR - Any condition can be true</option>
        </select>
      </label>
    </fieldset>
    
    <!-- time of day section -->
    <fieldset class="section">
      <legend>Time of Day</legend>
      <label>
        Constraint
        <select
          value={$editedStrategy.spec.time_of_day.constraint}
          on:change={(e) => updateEditedStrategy(s => s.spec.time_of_day.constraint = e.currentTarget.value)}
        >
          <option value="">None</option>
          <option value="specific_time">Specific Time</option>
          <option value="range">Time Range</option>
          <option value="pre_market">Pre-Market</option>
          <option value="after_hours">After Hours</option>
        </select>
      </label>
      
      {#if $editedStrategy.spec.time_of_day.constraint === 'specific_time' || $editedStrategy.spec.time_of_day.constraint === 'range'}
        <div class="flex">
          <label>
            Start Time
            <input 
              type="time" 
              value={$editedStrategy.spec.time_of_day.start_time} 
              on:change={(e) => updateEditedStrategy(s => s.spec.time_of_day.start_time = e.currentTarget.value)}
            />
          </label>
          
          {#if $editedStrategy.spec.time_of_day.constraint === 'range'}
            <label>
              End Time
              <input 
                type="time" 
                value={$editedStrategy.spec.time_of_day.end_time} 
                on:change={(e) => updateEditedStrategy(s => s.spec.time_of_day.end_time = e.currentTarget.value)}
              />
            </label>
          {/if}
        </div>
      {/if}
    </fieldset>
  </details>

  <!-- save / cancel -->
  <div class="actions">
    <button on:click={saveStrategy}>💾 Save</button>
    <button on:click={cancelEdit}>Cancel</button>
    {#if $editedStrategy.strategyId !== 'new'}
      <button class="danger" on:click={() => deleteStrategy($editedStrategy.strategyId )}>Delete</button>
    {/if}
  </div>
{/if}

<style>
  :global(body) {
    background-color: var(--ui-bg-primary);
    color: var(--text-primary);
  }
  input, select {
    background: var(--ui-bg-element);
    color: var(--text-primary);
    border: 1px solid var(--ui-border);
    padding: 0.5rem;
    border-radius: 4px;
  }
  fieldset.section {
    border: 1px solid var(--ui-border);
    background: var(--ui-bg-primary);
    border-radius: 8px;
    padding: 0.75rem;
    margin-bottom: 1rem;
  }
  legend {
    font-weight: 600;
    color: var(--text-secondary);
    padding: 0 0.5rem;
  }
  button {
    background: var(--ui-bg-element);
    color: var(--text-primary);
    border: 1px solid var(--ui-border);
    padding: 0.5rem 1rem;
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
  }
  button:hover {
    background: var(--ui-bg-hover);
  }
  button.danger {
    color: var(--color-down);
    border-color: var(--color-down);
  }
  button.danger:hover {
    background: rgba(239, 68, 68, 0.1);
  }
  .pill {
    background: var(--ui-bg-hover);
    color: var(--text-primary);
    display: inline-block;
    padding: 0.25rem 0.5rem;
    border-radius: 16px;
    margin: 0.25rem;
    cursor: pointer;
  }
  .pill:hover {
    background: var(--accent-indigo);
  }
  .help-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 16px;
    height: 16px;
    border-radius: 50%;
    background: var(--text-secondary);
    color: var(--ui-bg-primary);
    font-size: 12px;
    cursor: help;
  }
  .help-text {
    font-size: 0.8rem;
    color: var(--text-secondary);
    margin-bottom: 0.75rem;
  }
  .hint {
    font-size: 0.8rem;
    color: var(--text-secondary);
    font-style: italic;
    margin-top: 0.5rem;
  }
  .condition-row {
    display: grid;
    grid-template-columns: 1fr 0.8fr 0.8fr 0.8fr 1.2fr 0.4fr;
    gap: 0.5rem;
    align-items: center;
    margin-bottom: 0.5rem;
  }
  .derived-column-row {
    display: grid;
    grid-template-columns: 1fr 2fr 2fr 0.2fr;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }
  .quick-add {
    margin-top: 1rem;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }
  .quick-add p {
    margin: 0;
    font-size: 0.9rem;
  }
  .quick-add-btn {
    padding: 0.25rem 0.5rem;
    font-size: 0.8rem;
  }
  .flex {
    display: flex;
    gap: 1rem;
  }
  .flex-space {
    justify-content: space-between;
  }
  .advanced-section {
    margin-bottom: 1rem;
  }
  .advanced-section summary {
    cursor: pointer;
    padding: 0.5rem;
    background: var(--ui-bg-element);
    border-radius: 4px;
    font-weight: 600;
  }
  details[open] > summary {
    margin-bottom: 1rem;
  }
  .actions {
    display: flex;
    gap: 1rem;
    margin-top: 2rem;
  }
  .actions button {
    padding: 0.75rem 1.5rem;
    font-weight: 600;
  }
</style>
