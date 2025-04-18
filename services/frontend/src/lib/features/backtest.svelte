<script lang="ts">
  import { writable, get } from 'svelte/store';
  import StrategyDropdown from '$lib/utils/modules/strategyDropdown.svelte';
  import List from '$lib/utils/modules/list.svelte';
  import { privateRequest } from '$lib/core/backend';

  /***********************
   *     ─ Types ─       *
   ***********************/
  type StrategyId = number | 'new' | null;

  interface Instance {
    [key: string]: any;
  }

  /***********************
   *   ─ Component State ─
   ***********************/
  let selectedId: StrategyId = null;
  const list = writable<Instance[]>([]);
  let columns: string[] = [];
  const running = writable(false);
  let errorMsg: string | null = null;

  /***********************
   *     ─ Helpers ─     *
   ***********************/
  function prettify(col: string): string {
    if (col === 'timestamp') return 'Timestamp';
    return col.charAt(0).toUpperCase() + col.slice(1).replace(/_/g, ' ');
  }

  async function runBacktest() {
    if (selectedId === null || selectedId === 'new') return;

    running.set(true);
    errorMsg = null;
    list.set([]);

    try {
      const res = await privateRequest<any>('run_backtest', { strategyId: selectedId },true);
      console.log(res)
      const instances: Instance[] = res?.instances ?? [];

      list.set(instances);
      if (instances.length) {
        columns = Object.keys(instances[0]).map(prettify);
      } else {
        columns = [];
      }
    } catch (err: any) {
      errorMsg = err?.message || 'Failed to run backtest.';
    } finally {
      running.set(false);
    }
  }
</script>

<div class="panel">
  <StrategyDropdown bind:selectedId placeholder="Select strategy…" />
  <button class="run-btn" on:click={runBacktest} disabled={selectedId === null || selectedId === 'new' || $running}>
    {#if $running}
      Running…
    {:else}
      ▶ Run Backtest
    {/if}
  </button>
</div>

{#if errorMsg}
  <p class="error">{errorMsg}</p>
{/if}

<!-- Results table -->
{#if $list.length}
  <List {list} {columns} expandable={false} />
{:else if !$running && selectedId !== null}
  <p class="hint">No results yet. Click “Run Backtest” to fetch data.</p>
{/if}

<style>
  .panel {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
  }

  .run-btn {
    padding: 0.35rem 0.9rem;
    font-size: 0.9rem;
    border: 1px solid #94a3b8;
    border-radius: 4px;
    background: #f8fafc;
    transition: background 120ms;
    cursor: pointer;
  }

  .run-btn:hover:not(:disabled) {
    background: #e2e8f0;
  }

  .run-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .error {
    color: #b91c1c;
    margin-bottom: 0.5rem;
  }

  .hint {
    color: #64748b;
    font-size: 0.9rem;
    margin-top: 0.5rem;
  }
</style>

