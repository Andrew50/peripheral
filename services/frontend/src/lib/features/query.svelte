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
		padding: clamp(0.75rem, 2vw, 1.5rem);
		height: 100%;
		display: flex;
		flex-direction: column;
		justify-content: center;
	}

	.query-input-wrapper {
		position: relative;
		display: flex;
		width: 100%;
		max-width: min(90vw, 800px);
		margin: 0 auto;
	}

	.query-input {
		flex: 1;
		padding: clamp(0.5rem, 1vw, 0.75rem) clamp(0.75rem, 1.5vw, 1rem);
		font-size: clamp(0.875rem, 1vw, 1rem);
		background: var(--ui-bg-element, #333);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #fff);
		border-radius: clamp(4px, 0.5vw, 6px);
		min-height: clamp(36px, 5vh, 48px);
		padding-right: clamp(2.5rem, 5vw, 3rem);
	}

	.submit-button {
		position: absolute;
		right: clamp(0.25rem, 0.5vw, 0.5rem);
		top: 50%;
		transform: translateY(-50%);
		background: transparent;
		border: none;
		cursor: pointer;
		color: var(--text-secondary, #aaa);
		width: clamp(2rem, 4vw, 2.25rem);
		height: clamp(2rem, 4vw, 2.25rem);
		display: flex;
		align-items: center;
		justify-content: center;
		transition: color 0.2s;
	}

	.submit-button:hover {
		color: var(--text-primary, #fff);
	}

	.arrow-icon {
		width: clamp(1rem, 2vw, 1.25rem);
		height: clamp(1rem, 2vw, 1.25rem);
		fill: currentColor;
	}
</style>
