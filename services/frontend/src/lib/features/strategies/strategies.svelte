<script lang="ts">
import { onMount } from 'svelte';
import { writable, get, derived } from 'svelte/store';
import { strategies } from '$lib/core/stores';
import { privateRequest } from '$lib/core/backend';
import '$lib/core/global.css';

// --- Interfaces ---
// ... (Keep all Spec interfaces: UniverseFilterSpec, UniverseSpec, etc.) ...
type StrategyId = number | 'new' | null;
interface UniverseFilterSpec { securityFeature: "SecurityId" | "Ticker" | "Locale" | "Market" | "PrimaryExchange" | "Active" | "Sector" | "Industry"; include: string[]; exclude: string[]; }
interface UniverseSpec { filters: UniverseFilterSpec[]; timeframe: "1" | "1h" | "1d" | "1w"; extendedHours: boolean; startTime: string | null; endTime: string | null; }
interface SourceSpec { field: "SecurityId" | "Ticker" | "Locale" | "Market" | "PrimaryExchange" | "Active" | "Sector" | "Industry"; value: string; }
interface ExprElement { type: "column" | "operator"; value: string; }
interface FeatureSpec { name: string; featureId: number; source: SourceSpec; output: "raw" | "rankn" | "rankp"; expr: ExprElement[]; window: number; }
interface RhsSpec { featureId: number; const: number; scale: number; }
interface FilterSpec { name: string; lhs: number; operator: "<" | "<=" | ">=" | ">" | "!=" | "=="; rhs: RhsSpec; }
interface SortBySpec { feature: number; direction: "asc" | "desc"; }
interface NewStrategySpec { universe: UniverseSpec; features: FeatureSpec[]; filters: FilterSpec[]; sortBy: SortBySpec; }

interface Strategy {
  strategyId: number;
  name: string;
  spec: NewStrategySpec;
  score?: number;
  version?: string | number;
  createdAt?: string;
  isAlertActive?: boolean;
}
interface EditableStrategy extends Strategy { strategyId: StrategyId; spec: NewStrategySpec; }

// --- Stores ---
const loading = writable(false);
const selectedStrategyId = writable<StrategyId>(null);
const editedStrategy = writable<EditableStrategy | null>(null);
const viewedStrategyId = writable<number | null>(null);

const viewedStrategy = derived(
    [viewedStrategyId, strategies],
    ([$viewedStrategyId, $strategies]) => {
        if ($viewedStrategyId === null) return null;
        return $strategies.find(s => s.strategyId === $viewedStrategyId) || null;
    }
);

// --- Constants & Options ---
// ... (Keep timeframeOptions, securityFeatureOptions, etc.) ...
const timeframeOptions = [ { value: '1', label: '1 Minute' }, { value: '1h', label: '1 Hour' }, { value: '1d', label: '1 Day' }, { value: '1w', label: '1 Week' } ];
const securityFeatureOptions = ["SecurityId", "Ticker", "Locale", "Market", "PrimaryExchange", "Active", "Sector", "Industry"];
const operatorOptions = ["<", "<=", ">=", ">", "!=", "=="];
const outputOptions = ["raw", "rankn", "rankp"];
const sortDirectionOptions = ["asc", "desc"];
const baseColumnOptions = ["open", "high", "low", "close", "volume", "market_cap", "shares_outstanding", "eps", "revenue", "dividend", "social_sentiment", "fear_greed", "short_interest", "borrow_fee"];
const operatorChars = ["+", "-", "*", "/", "^"];
const operatorPrecedence: { [key: string]: number } = { '+': 1, '-': 1, '*': 2, '/': 2, '^': 3 };


// --- Help Text ---
// ... (Keep helpText object) ...
const helpText = { universeTimeframe: "Select the primary timeframe resolution for the strategy (e.g., 1d for daily).", universeExtendedHours: "Include pre-market and after-hours data? (Only applicable for 1-minute timeframe).", universeFilters: "Define the initial pool of securities using filters (e.g., Sector = Technology, Ticker includes AAPL).", universeTickerInclude: "Enter specific tickers to include (e.g. AAPL, MSFT). Press Enter after each ticker.", universeTickerExclude: "Enter specific tickers to exclude (e.g. TSLA, META). Press Enter after each ticker.", features: "Define calculated values (Features) used in your strategy's filters or sorting. Features use Reverse Polish Notation (RPN) for expressions.", featureExpr: "Expression in RPN (postfix). Use valid columns (e.g., 'close', 'volume') and operators (+, -, *, /, ^). Example: 'high low -' calculates High minus Low.", featureWindow: "Smoothing window for the expression (e.g., 14 for a 14-period average). 1 means no smoothing.", filters: "Define the conditions your strategy uses to select securities, comparing Features against constants or other Features.", sortBy: "Select a Feature to sort the final results by, and the direction (ascending/descending)." };

// --- Formatting/Helper Functions ---

// Capitalizes first letter of each word and replaces underscores with spaces
function formatName(name: string | undefined): string {
    if (!name) return 'Unnamed';
    return name.replace(/_/g, ' ')
               .toLowerCase()
               .split(' ')
               .map(word => word.charAt(0).toUpperCase() + word.slice(1))
               .join(' ');
}

// Converts RPN expression array to infix string
function formatExprInfix(expr: ExprElement[]): string {
    const stack: { value: string, precedence: number }[] = [];

    for (const element of expr) {
        const formattedValue = formatName(element.value); // Format column names nicely
        if (element.type === 'column') {
            stack.push({ value: formattedValue, precedence: Infinity }); // Operands have highest precedence
        } else if (element.type === 'operator') {
            const opPrecedence = operatorPrecedence[element.value] ?? 0;
            // Basic implementation assumes binary operators for simplicity
            if (stack.length < 2) return `(Error: Invalid RPN near '${element.value}')`; // Error handling

            const operand2 = stack.pop()!;
            const operand1 = stack.pop()!;

            // Add parentheses if the inner operation has lower precedence
            const val1 = operand1.precedence < opPrecedence ? `(${operand1.value})` : operand1.value;
            const val2 = operand2.precedence <= opPrecedence ? `(${operand2.value})` : operand2.value; // Right-associativity check for same precedence (e.g. a - (b - c))

            stack.push({ value: `${val1} ${element.value} ${val2}`, precedence: opPrecedence });
        }
    }

    if (stack.length !== 1) return '(Error: Invalid RPN expression)'; // Should end with one result
    return stack[0].value;
}


function formatTimeframe(tf: string | undefined): string {
    return timeframeOptions.find(opt => opt.value === tf)?.label ?? tf ?? 'N/A';
}

function formatFilterCondition(filter: FilterSpec, features: FeatureSpec[]): string {
    const lhsFeature = features.find(f => f.featureId === filter.lhs);
    const lhsName = formatName(lhsFeature?.name); // Use formatted name

    let rhsDesc: string;
    if (filter.rhs.featureId !== 0) {
        const rhsFeature = features.find(f => f.featureId === filter.rhs.featureId);
        rhsDesc = formatName(rhsFeature?.name); // Use formatted name
        if (filter.rhs.scale !== 1.0) {
            rhsDesc += ` * ${filter.rhs.scale}`;
        }
    } else {
        rhsDesc = `${filter.rhs.const}`;
        if (filter.rhs.scale !== 1.0) {
            // Decide how to show scaling on constant, e.g., multiply it out or show explicitly
            rhsDesc = `${filter.rhs.const * filter.rhs.scale} (${filter.rhs.const} * ${filter.rhs.scale})`; // Example: Show result and calculation
        }
    }
    return `${lhsName} ${filter.operator} ${rhsDesc}`;
}

function formatUniverseFilter(uFilter: UniverseFilterSpec): string {
    let desc = `${uFilter.securityFeature}`;
    if (uFilter.include.length > 0) desc += ` includes [${uFilter.include.join(', ')}]`;
    if (uFilter.exclude.length > 0) desc += `${uFilter.include.length > 0 ? ' and' : ''} excludes [${uFilter.exclude.join(', ')}]`;
    return desc;
}


// ... (Keep: getNextFeatureId, blankSpec, ensureValidSpec) ...
function getNextFeatureId(features: FeatureSpec[]): number { if (!features || features.length === 0) return 0; return Math.max(...features.map(f => f.featureId)) + 1; }
function blankSpec(): NewStrategySpec { const defaultFeatureId = 0; return { universe: { filters: [], timeframe: '1d', extendedHours: false, startTime: null, endTime: null }, features: [ { name: "close_price", featureId: defaultFeatureId, source: { field: "SecurityId", value: "relative" }, output: "raw", expr: [{ type: "column", value: "close" }], window: 1 } ], filters: [], sortBy: { feature: defaultFeatureId, direction: 'desc' } }; }
function ensureValidSpec(spec: any): NewStrategySpec { const validSpec = blankSpec(); if (!spec) return validSpec; if (spec.universe) { if (Array.isArray(spec.universe.filters)) validSpec.universe.filters = spec.universe.filters; if (typeof spec.universe.timeframe === 'string' && ['1', '1h', '1d', '1w'].includes(spec.universe.timeframe)) { validSpec.universe.timeframe = spec.universe.timeframe as "1" | "1h" | "1d" | "1w"; } if (typeof spec.universe.extendedHours === 'boolean') validSpec.universe.extendedHours = spec.universe.extendedHours; validSpec.universe.startTime = typeof spec.universe.startTime === 'string' && spec.universe.startTime !== "" ? spec.universe.startTime : null; validSpec.universe.endTime = typeof spec.universe.endTime === 'string' && spec.universe.endTime !== "" ? spec.universe.endTime : null; } const tempFeatures: FeatureSpec[] = []; if (Array.isArray(spec.features) && spec.features.length > 0) { spec.features.forEach((f: any) => { const nextId = getNextFeatureId(tempFeatures); tempFeatures.push({ name: typeof f.name === 'string' ? f.name : `feature_${f.featureId ?? nextId}`, featureId: typeof f.featureId === 'number' ? f.featureId : nextId, source: f.source && typeof f.source.field === 'string' ? f.source : { field: "SecurityId", value: "relative" }, output: f.output && ["raw", "rankn", "rankp"].includes(f.output) ? f.output : "raw", expr: Array.isArray(f.expr) ? f.expr : [{ type: "column", value: "close" }], window: typeof f.window === 'number' && f.window >= 1 ? f.window : 1 }); }); validSpec.features = tempFeatures; } else { validSpec.features = blankSpec().features; } const validFeatureIds = new Set(validSpec.features.map(f => f.featureId)); if (Array.isArray(spec.filters)) { validSpec.filters = spec.filters.filter((f: any) => f && typeof f.lhs === 'number' && validFeatureIds.has(f.lhs)) .map((f: any) => ({ name: typeof f.name === 'string' ? f.name : `filter_${f.lhs}`, lhs: f.lhs, operator: f.operator && operatorOptions.includes(f.operator) ? f.operator : ">", rhs: { featureId: typeof f.rhs?.featureId === 'number' && (f.rhs.featureId === 0 || validFeatureIds.has(f.rhs.featureId)) ? f.rhs.featureId : 0, const: typeof f.rhs?.const === 'number' ? f.rhs.const : 0.0, scale: typeof f.rhs?.scale === 'number' ? f.rhs.scale : 1.0 } })); } if (spec.sortBy && typeof spec.sortBy.feature === 'number' && validFeatureIds.has(spec.sortBy.feature)) { validSpec.sortBy.feature = spec.sortBy.feature; if (typeof spec.sortBy.direction === 'string' && sortDirectionOptions.includes(spec.sortBy.direction)) { validSpec.sortBy.direction = spec.sortBy.direction as "asc" | "desc"; } } else { validSpec.sortBy = { feature: validSpec.features[0]?.featureId ?? 0, direction: 'desc' }; } return validSpec; }

// --- Data Loading & CRUD Actions ---
// ... (Keep loadStrategies, startCreate, startEdit, cancelEdit, deleteStrategyConfirm, saveStrategy) ...
async function loadStrategies() { loading.set(true); viewedStrategyId.set(null); selectedStrategyId.set(null); try { const data = await privateRequest<Strategy[]>('getStrategies', {}); if (Array.isArray(data)) { data.forEach((d: any) => { d.spec = ensureValidSpec(d.spec); d.version = d.version ?? '1.0'; d.createdAt = d.createdAt ?? new Date(Date.now() - Math.random() * 1e10).toISOString(); d.isAlertActive = d.isAlertActive ?? (Math.random() > 0.7); }); } strategies.set(data || []); } catch (error) { console.error("Error loading strategies:", error); strategies.set([]); } finally { loading.set(false); } }
onMount(loadStrategies);
function viewStrategy(id: number) { selectedStrategyId.set(null); editedStrategy.set(null); viewedStrategyId.set(id); }
function startCreate() { const newStrategy: EditableStrategy = { strategyId: 'new', name: '', spec: blankSpec(), version: '1.0', createdAt: new Date().toISOString(), isAlertActive: false, }; viewedStrategyId.set(null); editedStrategy.set(newStrategy); selectedStrategyId.set('new'); }
function startEdit(strategyToEdit: Strategy | null) { if (!strategyToEdit) return; const validSpec = ensureValidSpec(strategyToEdit.spec); const clonedStrategy: EditableStrategy = JSON.parse(JSON.stringify({ ...strategyToEdit, spec: validSpec })); editedStrategy.set(clonedStrategy); selectedStrategyId.set(strategyToEdit.strategyId); }
function cancelEdit() { const editState = get(editedStrategy); editedStrategy.set(null); selectedStrategyId.set(null); if (editState?.strategyId === 'new') { viewedStrategyId.set(null); } }
async function deleteStrategyConfirm(id: number | null) { if (id === null || typeof id !== 'number') return; if (!confirm(`Are you sure you want to delete this strategy (ID: ${id})?`)) return; try { await privateRequest('deleteStrategy', { strategyId: id }); strategies.update(arr => arr.filter(s => s.strategyId !== id)); viewedStrategyId.set(null); selectedStrategyId.set(null); editedStrategy.set(null); } catch (error) { console.error("Error deleting strategy:", error); alert("Failed to delete strategy."); } }
async function saveStrategy() { const currentStrategy = get(editedStrategy); if (!currentStrategy) return; if (!currentStrategy.name.trim()) { alert("Strategy Name cannot be empty."); return; } if (!currentStrategy.spec?.features || currentStrategy.spec.features.length === 0) { alert("Strategy must have at least one Feature."); return; } const featureIds = currentStrategy.spec.features.map(f => f.featureId); if (new Set(featureIds).size !== featureIds.length) { alert("Feature IDs must be unique."); return; } const existingFeatureIds = new Set(featureIds); let invalidFilterFound = false; currentStrategy.spec.filters.forEach((filter, index) => { if (!existingFeatureIds.has(filter.lhs)) { alert(`Filter ${index + 1} uses non-existent LHS Feature.`); invalidFilterFound = true; } if (filter.rhs.featureId !== 0 && !existingFeatureIds.has(filter.rhs.featureId)) { alert(`Filter ${index + 1} uses non-existent RHS Feature.`); invalidFilterFound = true; } }); if (!existingFeatureIds.has(currentStrategy.spec.sortBy.feature)) { alert(`Sort By uses non-existent Feature.`); invalidFilterFound = true; } if (invalidFilterFound) return; const cleanSpec = ensureValidSpec(currentStrategy.spec); const payload = { name: currentStrategy.name, spec: cleanSpec }; console.log("Saving strategy with payload:", JSON.stringify(payload, null, 2)); try { let savedStrategyId: number | null = null; if (currentStrategy.strategyId === 'new') { const created = await privateRequest<Strategy>('newStrategy', payload); created.spec = ensureValidSpec(created.spec); created.version = created.version ?? currentStrategy.version ?? '1.0'; created.createdAt = created.createdAt ?? currentStrategy.createdAt ?? new Date().toISOString(); created.isAlertActive = created.isAlertActive ?? currentStrategy.isAlertActive ?? false; strategies.update(arr => [...arr, created]); savedStrategyId = created.strategyId; viewedStrategyId.set(null); selectedStrategyId.set(null); editedStrategy.set(null); } else if (typeof currentStrategy.strategyId === 'number') { await privateRequest('setStrategy', { strategyId: currentStrategy.strategyId, ...payload }); savedStrategyId = currentStrategy.strategyId; strategies.update(arr => arr.map(s => (s.strategyId === savedStrategyId ? { ...currentStrategy as Strategy, spec: cleanSpec } : s ))); selectedStrategyId.set(null); editedStrategy.set(null); } } catch (error: any) { console.error("Error saving strategy:", error); const errorMsg = error?.response?.data?.error || error.message || "An unknown error occurred."; alert(`Failed to save strategy: ${errorMsg}`); } }


// --- UI Update Helpers (Edit View) ---
// ... (Keep: updateEditedStrategy, addUniverseFilter, etc... addFilter, removeFilter) ...
function updateEditedStrategy(updater: (strategy: EditableStrategy) => void) { editedStrategy.update(strategy => { if (!strategy) return null; const clone = JSON.parse(JSON.stringify(strategy)); updater(clone); return clone; }); }
function addUniverseFilter() { updateEditedStrategy(s => { s.spec.universe.filters.push({ securityFeature: 'Ticker', include: [], exclude: [] }); }); }
function removeUniverseFilter(index: number) { updateEditedStrategy(s => { s.spec.universe.filters.splice(index, 1); }); }
function addUniverseInclude(filterIndex: number, value: string) { if (!value.trim()) return; updateEditedStrategy(s => { const filter = s.spec.universe.filters[filterIndex]; const upperVal = value.trim().toUpperCase(); if (filter && !filter.include.includes(upperVal)) { filter.include.push(upperVal); filter.exclude = filter.exclude.filter(ex => ex !== upperVal); } }); }
function removeUniverseInclude(filterIndex: number, valueIndex: number) { updateEditedStrategy(s => { s.spec.universe.filters[filterIndex]?.include.splice(valueIndex, 1); }); }
function addUniverseExclude(filterIndex: number, value: string) { if (!value.trim()) return; updateEditedStrategy(s => { const filter = s.spec.universe.filters[filterIndex]; const upperVal = value.trim().toUpperCase(); if (filter && !filter.exclude.includes(upperVal)) { filter.exclude.push(upperVal); filter.include = filter.include.filter(inc => inc !== upperVal); } }); }
function removeUniverseExclude(filterIndex: number, valueIndex: number) { updateEditedStrategy(s => { s.spec.universe.filters[filterIndex]?.exclude.splice(valueIndex, 1); }); }
function addFeature() { updateEditedStrategy(s => { const newId = getNextFeatureId(s.spec.features); s.spec.features.push({ name: `new_feature_${newId}`, featureId: newId, source: { field: "SecurityId", value: "relative" }, output: "raw", expr: [], window: 1 }); if (s.spec.features.length === 1 || s.spec.sortBy.feature === (blankSpec().features[0]?.featureId ?? 0)) { s.spec.sortBy.feature = newId; } }); }
function removeFeature(index: number) { updateEditedStrategy(s => { const removedFeatureId = s.spec.features[index]?.featureId; s.spec.features.splice(index, 1); s.spec.filters = s.spec.filters.filter(f => f.lhs !== removedFeatureId && (f.rhs.featureId === 0 || f.rhs.featureId !== removedFeatureId)); if (s.spec.sortBy.feature === removedFeatureId) { s.spec.sortBy.feature = s.spec.features[0]?.featureId ?? 0; } }); }
function addExprElement(featureIndex: number, type: 'column' | 'operator', value: string) { if (!value.trim()) return; updateEditedStrategy(s => { s.spec.features[featureIndex].expr.push({ type, value: value.trim() }); }); }
function removeExprElement(featureIndex: number, elementIndex: number) { updateEditedStrategy(s => { s.spec.features[featureIndex].expr.splice(elementIndex, 1); }); }
function addFilter() { updateEditedStrategy(s => { const defaultLhsFeatureId = s.spec.features[0]?.featureId ?? 0; s.spec.filters.push({ name: `new_filter_${s.spec.filters.length}`, lhs: defaultLhsFeatureId, operator: '>', rhs: { featureId: 0, const: 0, scale: 1.0 } }); }); }
function removeFilter(index: number) { updateEditedStrategy(s => { s.spec.filters.splice(index, 1); }); }

// Derived store for Edit View dropdowns
const availableFeatures = derived(editedStrategy, ($editedStrategy) => {
    if (!$editedStrategy || !$editedStrategy.spec?.features) return [];
    return $editedStrategy.spec.features.map(f => ({ id: f.featureId, name: f.name || `Feature ${f.featureId}` }));
});


</script>

{#if $viewedStrategyId === null && $selectedStrategyId === null}
  <div class="toolbar">
    <button on:click={startCreate}>＋ New Strategy</button>
  </div>

  {#if $loading}
    <p>Loading strategies...</p>
  {:else if !$strategies || $strategies.length === 0}
     <p>No strategies found. Click "＋ New Strategy" to create one.</p>
  {:else}
    <div class="table-container">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Timeframe</th>
            <th>Version</th>
            <th>Created</th>
            <th>Alert Active</th>
            <th>Score</th>
          </tr>
        </thead>
        <tbody>
          {#each $strategies as s (s.strategyId)}
            <tr class="clickable-row" on:click={() => viewStrategy(s.strategyId)} title="Click to view details">
              <td>{s.name}</td>
              <td>{formatTimeframe(s.spec?.universe?.timeframe)}</td>
              <td>{s.version ?? 'N/A'}</td>
              <td>{s.createdAt ? new Date(s.createdAt).toLocaleDateString() : 'N/A'}</td>
              <td class="alert-status">{s.isAlertActive ? '✔️ Active' : '❌ Inactive'}</td>
              <td>{s.score ?? '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

{:else if $viewedStrategyId !== null && $selectedStrategyId === null}
  {#if $viewedStrategy}
   {@const strat = $viewedStrategy}
   <div class="detail-view-container">
        <div class="detail-view-header">
            <div class="detail-view-title">
                 <h2>{formatName(strat.name)}</h2>
                 <div class="detail-view-meta">
                    <span>Version: {strat.version ?? 'N/A'}</span> |
                    <span>Created: {strat.createdAt ? new Date(strat.createdAt).toLocaleString() : 'N/A'}</span> |
                    <span class="alert-status">Alert: {strat.isAlertActive ? '✔️ Active' : '❌ Inactive'}</span>
                </div>
            </div>
             <div class="detail-view-actions">
                <button on:click={() => startEdit(strat)}>✏️ Edit</button>
                <button class="danger" on:click={() => deleteStrategyConfirm(strat.strategyId)}>🗑️ Delete</button>
                <button class="secondary" on:click={() => viewedStrategyId.set(null)}>← Back to List</button>
            </div>
        </div>

        <div class="detail-section-card">
            <h3>Universe</h3>
            <dl class="definition-list">
                <dt>Timeframe</dt><dd>{formatTimeframe(strat.spec.universe.timeframe)}</dd>
                <dt>Extended Hours</dt><dd>{strat.spec.universe.extendedHours ? 'Yes' : 'No'}</dd>
                {#if strat.spec.universe.startTime || strat.spec.universe.endTime}
                    <dt>Intraday Time</dt>
                    <dd>{strat.spec.universe.startTime ?? 'Market Open'} - {strat.spec.universe.endTime ?? 'Market Close'}</dd>
                {/if}
            </dl>
            {#if strat.spec.universe.filters.length > 0}
                <h4 class="subsection-title">Filters:</h4>
                <ul class="detail-list">
                    {#each strat.spec.universe.filters as uFilter}
                        <li>{formatUniverseFilter(uFilter)}</li>
                    {/each}
                </ul>
            {:else}
                 <p class="no-items-note"><em>No specific universe filters applied.</em></p>
            {/if}
        </div>

         <div class="detail-section-card">
             <h3>Features <span class="count-badge">{strat.spec.features.length}</span></h3>
             {#if strat.spec.features.length > 0}
                <ul class="detail-list feature-list">
                {#each strat.spec.features as feature}
                    <li class="detail-list-item">
                        <strong class="feature-name">{formatName(feature.name)}</strong>
                        <div class="feature-details">
                             <span>Output: {feature.output}</span>
                             <span>Window: {feature.window}</span>
                        </div>
                        <div class="feature-expr">Expr: <code>{formatExprInfix(feature.expr)}</code></div>
                    </li>
                {/each}
                </ul>
             {:else}
                <p class="warning-text">No features defined (strategy may not function).</p>
             {/if}
         </div>

         <div class="detail-section-card">
             <h3>Filters <span class="count-badge">{strat.spec.filters.length}</span></h3>
              {#if strat.spec.filters.length > 0}
                <ul class="detail-list filter-list">
                {#each strat.spec.filters as filter}
                    <li class="detail-list-item">
                        <strong class="filter-name">{formatName(filter.name) || 'Unnamed Filter'}</strong>
                        <div class="filter-condition">
                            <code>{formatFilterCondition(filter, strat.spec.features)}</code>
                        </div>
                    </li>
                {/each}
                </ul>
             {:else}
                 <p class="no-items-note"><em>No filters defined - strategy will likely return all securities from the universe.</em></p>
             {/if}
         </div>

         <div class="detail-section-card">
             <h3>Sort By</h3>
             {#if strat.spec.sortBy && strat.spec.features.some(f => f.featureId === strat.spec.sortBy.feature)}
                {@const sortFeature = strat.spec.features.find(f => f.featureId === strat.spec.sortBy.feature)}
                 <p>Feature: <strong>{formatName(sortFeature?.name)}</strong> <br/> Direction: <strong>{strat.spec.sortBy.direction.toUpperCase()}</strong></p>
             {:else}
                  <p class="warning-text"><em>Sort feature (ID: {strat.spec.sortBy?.feature}) not found or invalid.</em></p>
             {/if}
         </div>

   </div>
   {:else}
       <div class="loading-container">
            <p>Loading strategy details...</p>
            <button class="secondary" on:click={() => viewedStrategyId.set(null)}>← Back to List</button>
       </div>
   {/if}


{:else if $editedStrategy}
  <div class="form-block"> <label for="strategy-name">Strategy Name</label> <input id="strategy-name" type="text" placeholder="e.g., Daily Momentum Breakout" bind:value={$editedStrategy.name} /> </div>
  <fieldset class="section"> <legend>Universe Definition</legend> <div class="layout-grid cols-3 items-end"> <label> Timeframe <span class="help-icon" title={helpText.universeTimeframe}>?</span> <select bind:value={$editedStrategy.spec.universe.timeframe}> {#each timeframeOptions as tf} <option value={tf.value}>{tf.label}</option> {/each} </select> </label> <label class="inline-label"> <input type="checkbox" bind:checked={$editedStrategy.spec.universe.extendedHours} disabled={$editedStrategy.spec.universe.timeframe !== '1'} /> Extended Hours? <span class="help-icon" title={helpText.universeExtendedHours}>?</span> </label> {#if $editedStrategy.spec.universe.timeframe === '1'} <p class="hint">(Only available for 1-min timeframe)</p> {/if} </div> <div class="layout-grid cols-2"> <label> Intraday Start Time (Optional) <input type="time" bind:value={$editedStrategy.spec.universe.startTime} /> </label> <label> Intraday End Time (Optional) <input type="time" bind:value={$editedStrategy.spec.universe.endTime} /> </label> </div> <div class="subsection"> <h4>Security Filters <span class="help-icon" title={helpText.universeFilters}>?</span></h4> {#each $editedStrategy.spec.universe.filters as uFilter, uIndex (uIndex)} <div class="universe-filter-row"> <div class="universe-filter-header"> <select bind:value={uFilter.securityFeature}> {#each securityFeatureOptions as sf} <option value={sf}>{sf}</option> {/each} </select> <button class="danger-text" on:click={() => removeUniverseFilter(uIndex)}>✕ Remove</button> </div> {#if uFilter.securityFeature === 'Ticker'} <div class="layout-grid cols-2"> <div class="pill-group"> <h5>Include Tickers <span class="help-icon" title={helpText.universeTickerInclude}>?</span></h5> {#each uFilter.include as ticker, i (ticker)} <span class="pill" on:click={() => removeUniverseInclude(uIndex, i)}>{ticker} ✕</span> {/each} <input class="small" placeholder="Add Ticker (Enter)" on:keydown={(e) => { if (e.key === 'Enter' && e.currentTarget.value.trim()) { addUniverseInclude(uIndex, e.currentTarget.value); e.currentTarget.value = ''; e.preventDefault(); } }} /> </div> <div class="pill-group"> <h5>Exclude Tickers <span class="help-icon" title={helpText.universeTickerExclude}>?</span></h5> {#each uFilter.exclude as ticker, i (ticker)} <span class="pill" on:click={() => removeUniverseExclude(uIndex, i)}>{ticker} ✕</span> {/each} <input class="small" placeholder="Add Ticker (Enter)" on:keydown={(e) => { if (e.key === 'Enter' && e.currentTarget.value.trim()) { addUniverseExclude(uIndex, e.currentTarget.value); e.currentTarget.value = ''; e.preventDefault(); } }} /> </div> </div> {:else} <div class="layout-grid cols-2"> <label>Include Values <input type="text" bind:value={uFilter.include[0]} placeholder="Value" title="Enter single value for non-ticker include" /></label> <label>Exclude Values <input type="text" bind:value={uFilter.exclude[0]} placeholder="Value" title="Enter single value for non-ticker exclude" /></label> </div> <p class="hint">Enter a single value for Include/Exclude for {uFilter.securityFeature}.</p> {/if} </div> {/each} <button type="button" on:click={addUniverseFilter}>＋ Add Universe Filter</button> </div> </fieldset>
  <fieldset class="section"> <legend>Features <span class="help-icon" title={helpText.features}>?</span></legend> <p class="help-text">Define calculations used in Filters or Sorting. Use RPN for expressions.</p> {#each $editedStrategy.spec.features as feature, fIndex (feature.featureId)} <div class="feature-row"> <div class="layout-grid cols-3 items-center"> <label>Name <input type="text" bind:value={feature.name} placeholder="e.g., daily_range" /></label> <label>Output Type <select bind:value={feature.output}> {#each outputOptions as o} <option value={o}>{o}</option> {/each} </select> </label> <div class="feature-id-remove"> <span class="feature-id">ID: {feature.featureId}</span> {#if $editedStrategy.spec.features.length > 1} <button type="button" class="danger-text" on:click={() => removeFeature(fIndex)}>✕ Remove</button> {/if} </div> </div> <div class="layout-grid cols-3"> <label>Window <span class="help-icon" title={helpText.featureWindow}>?</span> <input type="number" min="1" step="1" bind:value={feature.window} /> </label> <label>Source Field <select bind:value={feature.source.field}> {#each securityFeatureOptions as sf} <option value={sf}>{sf}</option> {/each} </select> </label> <label>Source Value <input type="text" bind:value={feature.source.value} placeholder='"relative" or specific value' /> </label> </div> <div class="expr-builder"> <label class="expr-label">Expression (RPN) <span class="help-icon" title={helpText.featureExpr}>?</span></label> <div class="expression-display"> {#each feature.expr as element, eIndex (eIndex)} <span class="expr-element" class:operator={element.type === 'operator'} on:click={() => removeExprElement(fIndex, eIndex)} title="Click to remove"> {element.value} </span> {/each} {#if feature.expr.length === 0} <span class="hint">Empty Expression</span> {/if} </div> <div class="expression-input-area"> <input type="text" placeholder="Add Column/Operator (Enter)" list="rpn-suggestions" on:keydown={(e) => { if (e.key === 'Enter' && e.currentTarget.value.trim()) { const val = e.currentTarget.value.trim(); const type = operatorChars.includes(val) ? 'operator' : 'column'; addExprElement(fIndex, type, val); e.currentTarget.value = ''; e.preventDefault(); } }} /> <datalist id="rpn-suggestions"> {#each baseColumnOptions as col}<option value={col}>{col}</option>{/each} {#each operatorChars as op}<option value={op}>{op}</option>{/each} </datalist> <div class="quick-add"> {#each baseColumnOptions as col} <button type="button" class="quick-add-btn" on:click={() => addExprElement(fIndex, 'column', col)}>{col}</button> {/each} {#each operatorChars as op} <button type="button" class="quick-add-btn operator" on:click={() => addExprElement(fIndex, 'operator', op)}>{op}</button> {/each} </div> </div> </div> </div> {/each} <button type="button" on:click={addFeature}>＋ Add Feature</button> </fieldset>
  <fieldset class="section"> <legend>Filters (Conditions) <span class="help-icon" title={helpText.filters}>?</span></legend> <p class="help-text">Define conditions comparing Features to constants or other Features.</p> {#if $availableFeatures.length === 0} <p class="warning-text">You need to define at least one Feature before adding Filters.</p> {:else} {#each $editedStrategy.spec.filters as filter, fIndex (fIndex)} <div class="filter-row"> <label class="filter-label">IF</label> <select bind:value={filter.lhs} title="Left Hand Side Feature"> {#each $availableFeatures as feat} <option value={feat.id}>{feat.name} (ID: {feat.id})</option> {/each} </select> <select bind:value={filter.operator} title="Comparison Operator"> {#each operatorOptions as op} <option value={op}>{op}</option> {/each} </select> <div class="rhs-group"> <select bind:value={filter.rhs.featureId} title="Right Hand Side Feature (0 for Constant)"> <option value={0}>Constant Value</option> {#each $availableFeatures as feat} <option value={feat.id}>{feat.name} (ID: {feat.id})</option> {/each} </select> {#if filter.rhs.featureId === 0} <input class="small" type="number" step="any" bind:value={filter.rhs.const} title="Constant Value"/> {/if} <span class="scale-label">Scale:</span> <input class="tiny" type="number" step="any" bind:value={filter.rhs.scale} title="Scale Factor (applied to RHS)" /> </div> <button type="button" class="danger-text" on:click={() => removeFilter(fIndex)}>✕</button> <label class="filter-name-label">Name (Optional): <input type="text" class="small" bind:value={filter.name} /></label> </div> {/each} <button type="button" on:click={addFilter} disabled={$availableFeatures.length === 0}>＋ Add Filter</button> {/if} </fieldset>
  <fieldset class="section"> <legend>Sort Results By <span class="help-icon" title={helpText.sortBy}>?</span></legend> {#if $availableFeatures.length === 0} <p class="warning-text">You need to define at least one Feature before setting Sort criteria.</p> {:else} <div class="layout-grid cols-2"> <label>Feature to Sort By <select bind:value={$editedStrategy.spec.sortBy.feature}> {#each $availableFeatures as feat} <option value={feat.id}>{feat.name} (ID: {feat.id})</option> {/each} </select> </label> <label>Direction <select bind:value={$editedStrategy.spec.sortBy.direction}> {#each sortDirectionOptions as dir} <option value={dir}>{dir.toUpperCase()}</option> {/each} </select> </label> </div> {/if} </fieldset>
  <div class="actions"> <button class="primary" on:click={saveStrategy}>💾 Save Strategy</button> <button type="button" on:click={cancelEdit}>Cancel</button> {#if typeof $editedStrategy.strategyId === 'number'} <button type="button" class="danger" on:click={() => deleteStrategyConfirm($editedStrategy.strategyId)}>Delete Strategy</button> {/if} </div>
{/if}


<style>
  /* --- Base & General Styles --- */
  :global(body) {
    background-color: var(--ui-bg-primary, #f4f7f9); /* Lighter background */
    color: var(--text-primary, #333);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji"; /* System fonts */
    font-size: 15px; /* Slightly smaller base */
    line-height: 1.6;
  }

  input, select, textarea {
    background: var(--ui-bg-element, #fff);
    color: var(--text-primary, #333);
    border: 1px solid var(--ui-border, #d1d9e0); /* Softer border */
    padding: 0.5rem 0.75rem;
    border-radius: 6px; /* Slightly larger radius */
    width: 100%;
    box-sizing: border-box;
    margin-bottom: 0.5rem;
    font-size: 0.9rem; /* Smaller input text */
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
  }
  input:focus, select:focus, textarea:focus {
      border-color: var(--accent-blue, #007bff);
      box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.2);
      outline: none;
  }
  input[type="checkbox"] { width: auto; margin-right: 0.5rem; vertical-align: middle; }
  input:disabled { background-color: var(--ui-bg-disabled, #e9ecef); cursor: not-allowed; }
  input.small { font-size: 0.8rem; padding: 0.25rem 0.5rem; }
  input.tiny { font-size: 0.75rem; padding: 0.1rem 0.3rem; width: 55px; }

  label { display: block; font-weight: 500; margin-bottom: 0.25rem; font-size: 0.85rem; color: var(--text-secondary, #555); }
  label.inline-label { display: inline-flex; align-items: center; margin-bottom: 0.5rem; }

  button { background: var(--ui-bg-element, #fff); color: var(--text-primary, #333); border: 1px solid var(--ui-border, #d1d9e0); padding: 0.5rem 1.1rem; border-radius: 6px; cursor: pointer; transition: all 0.2s ease; font-size: 0.9rem; vertical-align: middle; }
  button:hover { background: var(--ui-bg-hover, #f0f3f6); border-color: var(--ui-border-hover, #b8c4cf); }
  button:active { transform: translateY(1px); }
  button:disabled { opacity: 0.6; cursor: not-allowed; }
  button.primary { background-color: var(--accent-blue, #007bff); color: #fff; border-color: var(--accent-blue, #007bff); font-weight: 500; }
  button.primary:hover { background-color: var(--accent-blue-dark, #0056b3); border-color: var(--accent-blue-dark, #0056b3); }
  button.secondary { background-color: var(--ui-bg-secondary, #6c757d); color: #fff; border-color: var(--ui-bg-secondary, #6c757d); }
  button.secondary:hover { background-color: #5a6268; border-color: #545b62; }
  button.danger { color: var(--color-down, #dc3545); border-color: var(--color-down, #dc3545); background-color: transparent; }
  button.danger:hover { background: rgba(220, 53, 69, 0.05); color: var(--color-down-dark, #a71d2a); border-color: var(--color-down-dark, #a71d2a); }
  button.danger-text { background: none; border: none; color: var(--color-down, #dc3545); padding: 0.25rem; margin-left: 0.5rem; font-size: 0.85rem; cursor: pointer; vertical-align: middle; }
  button.danger-text:hover { color: var(--color-down-dark, #a71d2a); text-decoration: underline; }

  code {
      font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace;
      background-color: var(--ui-bg-code, #e3eaf0);
      padding: 0.1em 0.4em;
      border-radius: 4px;
      font-size: 0.9em;
  }

  /* --- Layout & Sections --- */
  .layout-grid { display: grid; gap: 1rem; margin-bottom: 1rem; }
  .layout-grid.cols-2 { grid-template-columns: repeat(2, 1fr); }
  .layout-grid.cols-3 { grid-template-columns: repeat(3, 1fr); }
  .layout-grid.items-center { align-items: center; }
  .layout-grid.items-end { align-items: flex-end; }

  fieldset.section { border: 1px solid var(--ui-border, #d1d9e0); background: var(--ui-bg-element, #fff); border-radius: 8px; padding: 1.25rem 1.5rem; margin-bottom: 1.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
  legend { font-weight: 600; font-size: 1.1rem; color: var(--text-primary, #333); padding: 0 0.5rem; margin-bottom: 1rem; border-bottom: 1px solid var(--ui-border-light, #e9ecef); display: inline-block; }
  .subsection { margin-top: 1.25rem; padding-top: 1rem; border-top: 1px solid var(--ui-border-light, #e9ecef); }
  .subsection h4 { font-weight: 600; margin-bottom: 0.75rem; font-size: 1rem; }
  .subsection h5 { font-weight: 500; font-size: 0.8rem; margin-bottom: 0.25rem; color: var(--text-secondary); }

  .form-block { margin-bottom: 1rem; }
  .actions { display: flex; gap: 1rem; margin-top: 2rem; padding-top: 1rem; border-top: 1px solid var(--ui-border-light, #e9ecef); }
  .actions button { padding: 0.7rem 1.4rem; font-weight: 500; }

  .help-icon { display: inline-flex; align-items: center; justify-content: center; width: 15px; height: 15px; border-radius: 50%; background: var(--text-secondary, #aaa); color: #fff; font-size: 10px; font-weight: bold; cursor: help; margin-left: 5px; vertical-align: middle; }
  .help-text { font-size: 0.85rem; color: var(--text-secondary, #666); margin-bottom: 1rem; margin-top: -0.75rem; }
  .hint { font-size: 0.75rem; color: var(--text-secondary, #777); font-style: italic; margin-top: 0.25rem; }
  .warning-text { color: var(--accent-orange, #fd7e14); font-size: 0.85rem; font-weight: 500; }
  .no-items-note { font-style: italic; color: var(--text-secondary); font-size: 0.9rem; margin-top: 0.5rem; }
  .loading-container { padding: 2rem; text-align: center; }

  /* --- List View --- */
  .toolbar { margin-bottom: 1rem; }
  .table-container { overflow-x: auto; border: 1px solid var(--ui-border, #d1d9e0); border-radius: 8px; background-color: var(--ui-bg-element, #fff); }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: 0.8rem 1rem; border-bottom: 1px solid var(--ui-border, #d1d9e0); text-align: left; vertical-align: middle; font-size: 0.85rem; }
  th { background-color: var(--ui-bg-secondary, #f8f9fa); font-weight: 600; color: var(--text-secondary, #555); border-top: none; border-bottom-width: 2px; }
  tbody tr { transition: background-color 0.15s ease-in-out; }
  tbody tr:last-child td { border-bottom: none; }
  tbody tr.clickable-row { cursor: pointer; }
  tbody tr.clickable-row:hover { background-color: var(--ui-bg-hover, #eef2f5); }
  .alert-status { font-weight: 500; white-space: nowrap; }
  .alert-status > span { vertical-align: middle; margin-right: 0.25rem; } /* If using icons */

  /* --- Detail View --- */
  .detail-view-container { padding: 1rem; }
  .detail-view-header { display: flex; justify-content: space-between; align-items: flex-start; /* Align items top */ margin-bottom: 1.5rem; padding-bottom: 1rem; border-bottom: 1px solid var(--ui-border-light, #e9ecef); flex-wrap: wrap; gap: 0.5rem; }
  .detail-view-title h2 { margin: 0 0 0.25rem 0; font-size: 1.8rem; font-weight: 600; line-height: 1.2; color: var(--text-primary); }
  .detail-view-meta { font-size: 0.8rem; color: var(--text-secondary, #666); display: flex; flex-wrap: wrap; gap: 0 0.75rem; /* Horizontal gap only */ }
  .detail-view-meta span { white-space: nowrap; }
  .detail-view-actions { display: flex; gap: 0.5rem; flex-shrink: 0; /* Prevent shrinking */ align-self: flex-start; /* Align with title */ }
  .detail-section-card { background-color: var(--ui-bg-element, #fff); border: 1px solid var(--ui-border-light, #e3eaf0); border-radius: 8px; padding: 1rem 1.25rem; margin-bottom: 1.25rem; box-shadow: 0 1px 2px rgba(0,0,0,0.03); }
  .detail-section-card h3 { font-size: 1.15rem; font-weight: 600; margin: 0 0 0.75rem 0; color: var(--text-primary); display: flex; align-items: center; justify-content: space-between; }
  .count-badge { font-size: 0.8rem; font-weight: normal; background-color: var(--ui-bg-secondary, #e9ecef); color: var(--text-secondary, #555); padding: 0.15rem 0.5rem; border-radius: 10px; }
  .definition-list { display: grid; grid-template-columns: auto 1fr; gap: 0.3rem 1rem; font-size: 0.9rem; }
  .definition-list dt { font-weight: 500; color: var(--text-secondary, #555); text-align: right; }
  .definition-list dd { margin: 0; color: var(--text-primary); }
  h4.subsection-title { font-size: 0.9rem; font-weight: 600; color: var(--text-secondary, #555); margin: 1rem 0 0.5rem 0; padding-bottom: 0.25rem; border-bottom: 1px dashed var(--ui-border-light, #e9ecef); }
  .detail-list { list-style: none; padding: 0; margin: 0.5rem 0 0 0; }
  .detail-list-item { padding: 0.75rem 0; border-bottom: 1px solid var(--ui-border-light, #e9ecef); font-size: 0.9rem; }
  .detail-list-item:last-child { border-bottom: none; padding-bottom: 0; }
  .detail-list-item strong { color: var(--text-primary); font-weight: 500; }
  .feature-list .detail-list-item { padding: 0.6rem 0; }
  .feature-name { display: block; margin-bottom: 0.25rem; font-size: 1rem; }
  .feature-details { display: flex; gap: 1rem; font-size: 0.8rem; color: var(--text-secondary); margin-bottom: 0.25rem; }
  .feature-expr { font-size: 0.85rem; margin-top: 0.25rem; }
  .feature-expr code { background-color: transparent; padding: 0; }
  .filter-name { display: block; margin-bottom: 0.25rem; }
  .filter-condition code { display: block; background-color: var(--ui-bg-code, #e3eaf0); padding: 0.5rem; border-radius: 4px; white-space: normal; }

  /* --- Edit View Specific --- */
  /* (Styles for elements primarily used in Edit View, kept from previous version) */
  .pill-group { margin-bottom: 0.5rem; }
  .pill-group input.small { margin-top: 0.5rem; }
  .pill { background: var(--ui-bg-hover, #e9ecef); color: var(--text-primary, #333); display: inline-block; padding: 0.25rem 0.75rem; border-radius: 16px; margin: 0.25rem 0.25rem 0.25rem 0; cursor: pointer; font-size: 0.8rem; border: 1px solid var(--ui-border-light, #dee2e6); transition: background-color 0.15s ease-in-out; }
  .pill:hover { background: var(--accent-red-light, #f8d7da); border-color: var(--accent-red, #dc3545); color: var(--accent-red-dark, #721c24); }
  .universe-filter-row { border: 1px solid var(--ui-border-light, #e9ecef); padding: 1rem; margin-bottom: 1rem; border-radius: 6px; background-color: var(--ui-bg-element, #fff); }
  .universe-filter-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
  .universe-filter-header select { flex-grow: 1; margin-right: 1rem; margin-bottom: 0; }
  .feature-row { border: 1px solid var(--ui-border-light, #e9ecef); padding: 1rem; margin-bottom: 1rem; border-radius: 6px; background-color: var(--ui-bg-element, #fff); }
  .feature-id-remove { text-align: right; }
  .feature-id { display: none; /* Hide in Edit view too */ }
  .expr-builder { margin-top: 1rem; }
  .expr-label { font-weight: 500; display: block; margin-bottom: 0.25rem; }
  .expression-display { border: 1px solid var(--ui-border, #ced4da); background-color: var(--ui-bg-secondary, #f8f9fa); padding: 0.5rem; border-radius: 4px; min-height: 40px; margin-bottom: 0.5rem; display: flex; flex-wrap: wrap; gap: 0.35rem; align-items: center; }
  .expr-element { display: inline-block; border: 1px solid var(--ui-border-hover, #adb5bd); padding: 0.15rem 0.4rem; border-radius: 3px; font-size: 0.8rem; background-color: var(--ui-bg-element, #fff); cursor: pointer; transition: background-color 0.15s ease-in-out; white-space: nowrap; }
  .expr-element:hover { background-color: var(--accent-red-light, #f8d7da); border-color: var(--accent-red, #dc3545); }
  .expr-element.operator { background-color: var(--accent-blue-light, #cfe2ff); border-color: var(--accent-blue, #0d6efd); font-weight: bold; }
  .expr-element.operator:hover { background-color: var(--accent-red-light, #f8d7da); border-color: var(--accent-red, #dc3545); }
  .expression-input-area { display: grid; grid-template-columns: 1fr; gap: 0.5rem; /* Stack input and quick add */ }
  .expression-input-area .quick-add { grid-row: 2; /* Ensure quick add is below */ }
  .quick-add { display: flex; flex-wrap: wrap; gap: 0.35rem; align-items: center; }
  .quick-add-btn { padding: 0.25rem 0.5rem; font-size: 0.75rem; background-color: var(--ui-bg-element, #e9ecef); border: 1px solid var(--ui-border, #ced4da); }
  .quick-add-btn:hover { background-color: var(--ui-bg-hover, #dee2e6); }
  .quick-add-btn.operator { background-color: var(--accent-blue-light, #cfe2ff); border-color: var(--accent-blue, #0d6efd); font-weight: bold; }
  .quick-add-btn.operator:hover { background-color: var(--accent-blue, #0d6efd); color: #fff; }
  .filter-row { display: grid; grid-template-columns: auto minmax(120px, 1fr) auto minmax(180px, 1.5fr) auto; gap: 0.75rem; align-items: center; margin-bottom: 1rem; padding: 0.75rem; border: 1px solid var(--ui-border-light, #e9ecef); border-radius: 6px; background-color: var(--ui-bg-element, #fff); }
  .filter-label { font-size: 0.85rem; font-weight: bold; margin-bottom: 0; }
  .filter-row select, .filter-row .rhs-group { margin-bottom: 0; }
  .rhs-group { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
  .rhs-group select { flex-grow: 1; min-width: 100px; }
  .rhs-group input { flex-shrink: 0; }
  .scale-label { font-size: 0.75rem; color: var(--text-secondary, #666); white-space: nowrap; }
  .filter-name-label { grid-column: 1 / -1; margin-top: 0.5rem; font-size: 0.75rem; font-weight: normal; display: flex; align-items: center; gap: 0.5rem; }
  .filter-name-label input.small { flex-grow: 1; margin-bottom: 0; }

</style>
