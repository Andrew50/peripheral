<script lang="ts">
  import { queryInstanceInput } from '$lib/utils/popups/input.svelte';
  import { onMount } from 'svelte';
  import type { Instance } from '$lib/core/types';
	import { privateRequest } from '$lib/core/backend';
  
  // Define types for our response data
  type FunctionResult = {
    function_name: string;
    result?: any;
    error?: string;
  };
  
  type QueryResponse = {
    type: 'text' | 'function_calls' | string;
    text?: string;
    results?: FunctionResult[];
  };
  
  let inputValue = '';
  let queryInput: HTMLInputElement;
  let responseText = '';
  let isLoading = false;
  let hasResponse = false;
  let responseType = '';
  let functionResults: FunctionResult[] = [];
  
  onMount(() => {
    if (queryInput) {
      setTimeout(() => queryInput.focus(), 100);
    }
  });
  
  function handleSubmit() {
    if (!inputValue.trim()) return;
    
    isLoading = true;
    hasResponse = false;
    
    privateRequest('getQuery', { query: inputValue }).then((response) => {
      // Type assertion to handle the unknown response type
      const typedResponse = response as QueryResponse;
      console.log(typedResponse);
      isLoading = false;
      hasResponse = true;
      
      if (typedResponse.type === "text") {
        responseText = typedResponse.text || '';
        responseType = "text";
        functionResults = [];
      } else if (typedResponse.type === "function_calls") {
        responseText = typedResponse.text || "Function calls executed successfully.";
        responseType = "function_calls";
        functionResults = typedResponse.results || [];
      }
    }).catch(error => {
      console.error("Error fetching response:", error);
      isLoading = false;
      hasResponse = true;
      responseText = `Error: ${error.message || "Failed to get response"}`;
      responseType = "error";
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
    <button class="submit-button" on:click={handleSubmit} aria-label="Submit query" disabled={isLoading}>
      {#if isLoading}
        <span class="loading-spinner"></span>
      {:else}
        <svg viewBox="0 0 24 24" class="arrow-icon">
          <path d="M2,21L23,12L2,3V10L17,12L2,14V21Z" />
        </svg>
      {/if}
    </button>
  </div>
  
  {#if hasResponse}
    <div class="response-container">
      <div class="response-header">
        <span class="response-type">{responseType === "error" ? "Error" : "Response"}</span>
      </div>
      <div class="response-content">
        <div class="response-text">
          {#if responseText}
            <p>{responseText}</p>
          {/if}
        </div>
        
        {#if responseType === "function_calls" && functionResults.length > 0}
          <div class="function-results">
            <h4>Function Results:</h4>
            {#each functionResults as result}
              <div class="function-result">
                <span class="function-name">{result.function_name}</span>
                {#if result.error}
                  <span class="function-error">Error: {result.error}</span>
                {:else}
                  <div class="function-data">
                    {JSON.stringify(result.result, null, 2)}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
	.query-container {
		padding: clamp(0.75rem, 2vw, 1.5rem);
		height: 100%;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
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

	.submit-button:hover:not(:disabled) {
		color: var(--text-primary, #fff);
	}
  
  .submit-button:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }

	.arrow-icon {
		width: clamp(1rem, 2vw, 1.25rem);
		height: clamp(1rem, 2vw, 1.25rem);
		fill: currentColor;
	}
  
  .loading-spinner {
    width: 1.2rem;
    height: 1.2rem;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    border-top-color: #fff;
    animation: spin 1s ease-in-out infinite;
  }
  
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
  
  .response-container {
    width: 100%;
    max-width: min(90vw, 800px);
    margin: 0 auto;
    background: var(--ui-bg-element, #333);
    border: 1px solid var(--ui-border, #444);
    border-radius: clamp(4px, 0.5vw, 6px);
    overflow: hidden;
  }
  
  .response-header {
    padding: 0.75rem 1rem;
    background: var(--ui-bg-element-darker, #2a2a2a);
    border-bottom: 1px solid var(--ui-border, #444);
  }
  
  .response-type {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--text-secondary, #aaa);
    text-transform: uppercase;
  }
  
  .response-content {
    padding: 1rem;
  }
  
  .response-text {
    color: var(--text-primary, #fff);
    font-size: 0.9375rem;
    line-height: 1.5;
    white-space: pre-wrap;
    margin-bottom: 1rem;
    max-height: 400px;
    overflow-y: auto;
  }
  
  .function-results {
    margin-top: 1.5rem;
    border-top: 1px solid var(--ui-border, #444);
    padding-top: 1rem;
  }
  
  .function-results h4 {
    margin: 0 0 0.75rem;
    font-size: 0.875rem;
    color: var(--text-secondary, #aaa);
  }
  
  .function-result {
    padding: 0.75rem;
    background: var(--ui-bg-element-darker, #2a2a2a);
    border-radius: 4px;
    margin-bottom: 0.5rem;
  }
  
  .function-name {
    display: block;
    font-weight: 500;
    margin-bottom: 0.5rem;
    color: var(--accent-color, #3a8bf7);
  }
  
  .function-error {
    color: var(--error-color, #f44336);
  }
  
  .function-data {
    font-family: monospace;
    font-size: 0.8125rem;
    white-space: pre-wrap;
    overflow-x: auto;
  }
</style>
