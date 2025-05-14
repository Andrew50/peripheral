<script lang="ts">
  import { onMount } from 'svelte';
  import { writable } from 'svelte/store';
  import { privateRequest } from '$lib/utils/helpers/backend';
  import '$lib/styles/global.css';

  interface Strategy {
    strategyId: number;
    name: string;
    spec: {
      timeframes: string[];
    };
    score?: number;
  }

  // Create stores
  const loading = writable(false);
  const strategies = writable<Strategy[]>([]);
  const showAlert = writable(false);

  async function loadStrategies() {
    loading.set(true);
    try {
      const data = await privateRequest<Strategy[]>('getStrategies', {});
      strategies.set(data);
    } catch (error) {
      console.error("Error loading strategies:", error);
    } finally {
      loading.set(false);
    }
  }
  
  onMount(loadStrategies);

  function toggleAlert() {
    showAlert.set(true);
    setTimeout(() => showAlert.set(false), 3000); // Hide after 3 seconds
  }
</script>

<!-- list view -->
<div class="toolbar">
  <h2>Strategies</h2>
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
        {#if $strategies && $strategies.length}
          {#each $strategies as s}
            <tr>
              <td>{s.name}</td>
              <td>{s.spec?.timeframes?.join(', ') || '—'}</td>
              <td>{s.score ?? '—'}</td>
              <td>
                <button on:click={toggleAlert}>Set Alert</button>
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

{#if $showAlert}
  <div class="alert-overlay">
    <div class="alert-box">
      <p>Coming Soon!</p>
      <button on:click={() => showAlert.set(false)}>Close</button>
    </div>
  </div>
{/if}

<style>
  :global(body) {
    background-color: var(--ui-bg-primary);
    color: var(--text-primary);
  }
  
  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  
  table {
    width: 100%;
    border-collapse: collapse;
  }
  
  th, td {
    text-align: left;
    padding: 0.75rem;
    border-bottom: 1px solid var(--ui-border);
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
  
  .alert-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
  }
  
  .alert-box {
    background-color: var(--ui-bg-primary);
    border: 1px solid var(--ui-border);
    border-radius: 8px;
    padding: 1.5rem;
    max-width: 400px;
    text-align: center;
  }
  
  .alert-box p {
    font-size: 1.2rem;
    margin-bottom: 1rem;
  }
</style>
