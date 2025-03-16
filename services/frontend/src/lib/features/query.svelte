<script lang="ts">
  import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
  import { onMount } from 'svelte';
  import type { Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
  
  let inputValue = '';
  let queryInput: HTMLInputElement;
  
  onMount(() => {
    if (queryInput) {
      setTimeout(() => queryInput.focus(), 100);
    }
  });
  
  function handleSubmit() {
    privateRequest('getLLMParsedQuery', { query: inputValue }).then((response) => {
      console.log(response);
    });
  }
  
  function handleKeyDown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      handleSubmit();
    }
  }
</script>

<div class="query-container">
  <div class="query-input-wrapper">
    <input
      type="text"
      class="query-input"
      placeholder="Enter query..."
      bind:value={inputValue}
      bind:this={queryInput}
      on:keydown={handleKeyDown}
    />
    <button class="submit-button" on:click={handleSubmit} aria-label="Submit query">
      <svg viewBox="0 0 24 24" class="arrow-icon">
        <path d="M2,21L23,12L2,3V10L17,12L2,14V21Z" />
      </svg>
    </button>
  </div>
</div>

<style>
  .query-container {
    padding: 15px;
    height: 100%;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  
  .query-input-wrapper {
    position: relative;
    display: flex;
    width: 100%;
    max-width: 800px;
    margin: 0 auto;
  }
  
  .query-input {
    flex: 1;
    padding: 10px 12px;
    font-size: 16px;
    background: var(--ui-bg-element, #333);
    border: 1px solid var(--ui-border, #444);
    color: var(--text-primary, #fff);
    border-radius: 4px;
    min-height: 48px;
    padding-right: 40px;
  }
  
  .submit-button {
    position: absolute;
    right: 5px;
    top: 50%;
    transform: translateY(-50%);
    background: transparent;
    border: none;
    cursor: pointer;
    color: var(--text-secondary, #aaa);
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s;
  }
  
  .submit-button:hover {
    color: var(--text-primary, #fff);
  }
  
  .arrow-icon {
    width: 20px;
    height: 20px;
    fill: currentColor;
  }
</style> 