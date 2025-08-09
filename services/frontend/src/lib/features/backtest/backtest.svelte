<script lang="ts">
	import { writable, get } from 'svelte/store';
	import StrategyDropdown from '$lib/components/strategyDropdown.svelte';
	import List from '$lib/components/list.svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';

	/***********************
	 *     ─ Types ─       *
	 ***********************/
	type StrategyId = number | 'new' | null;
	interface Instance {
		[key: string]: any;
	}

	interface Summary {
		count: number;
		timeframe: string;
		columns: string[];
		date_range?: {
			start: string;
			end: string;
			start_ms: number;
			end_ms: number;
		};
	}

	/***********************
	 *   ─ Component State ─
	 ***********************/
	let selectedId: StrategyId = null;
	const list = writable<Instance[] | null>([]);
	let columns: string[] = [];
	let summary: Summary | null = null;
	const running = writable(false);
	let errorMsg: string | null = null;

	// Date range filters (for future implementation)
	let startDate: string = '';
	let endDate: string = '';

	/***********************
	 *     ─ Helpers ─     *
	 ***********************/
	function prettify(col: string): string {
		if (col === 'timestamp') return 'Timestamp';
		if (col === 'ticker') return 'Ticker';
		return col.charAt(0).toUpperCase() + col.slice(1).replace(/_/g, ' ');
	}

	// Helper function to process any remaining numeric objects that might come through
	function processNumericObject(value: any): any {
		if (value && typeof value === 'object' && !Array.isArray(value)) {
			// Check if this looks like a PostgreSQL numeric type
			if ('Exp' in value && 'Int' in value) {
				const exp = Number(value.Exp);
				const int = Number(value.Int);
				if (!isNaN(exp) && !isNaN(int)) {
					return int * Math.pow(10, exp);
				}
			}
		}
		return value;
	}

	function formatDate(dateString: string): string {
		if (!dateString) return '';
		return new Date(dateString).toLocaleDateString('en-US', {
			year: 'numeric',
			month: 'short',
			day: 'numeric'
		});
	}

	async function runBacktest() {
		if (selectedId === null || selectedId === 'new') return;
		running.set(true);
		errorMsg = null;
		list.set([]);
		summary = null;

		try {
			console.log('running');
			const res = await privateRequest<any>(
				'run_backtest',
				{ strategyId: selectedId, returnResults: true },
				true
			);
			console.log(res);

			// Extract instances and summary
			const instances: Instance[] = res?.instances ?? [];
			summary = res?.summary || null;

			// Update the list with the instances data, null if none
			if (instances.length === 0) {
				list.set(null);
			} else {
				list.set(instances);
			}

			// Get column names from the first instance or from summary.columns if available
			if (instances.length) {
				// Get all column names from the first instance
				let allColumns = Object.keys(instances[0]);

				// Filter out securityId which we don't want to display
				allColumns = allColumns.filter((col) => col !== 'securityId');

				// Order columns: ticker and timestamp first, then other columns
				let orderedColumns = [];

				// Add ticker first if it exists
				if (allColumns.includes('ticker')) {
					orderedColumns.push('ticker');
					allColumns = allColumns.filter((col) => col !== 'ticker');
				}

				// Add timestamp second if it exists
				if (allColumns.includes('timestamp')) {
					orderedColumns.push('timestamp');
					allColumns = allColumns.filter((col) => col !== 'timestamp');
				}

				// Add remaining columns
				columns = [...orderedColumns, ...allColumns];
				console.log(columns);

				// Convert to pretty column names
				//columns = orderedColumns.map(prettify);
			} else if (summary?.columns) {
				// Fallback to summary.columns if available
				// Filter and order columns from summary
				let summaryColumns = summary.columns.filter((col) => col !== 'securityId');
				let orderedColumns = [];

				if (summaryColumns.includes('ticker')) {
					orderedColumns.push('ticker');
					summaryColumns = summaryColumns.filter((col) => col !== 'ticker');
				}

				if (summaryColumns.includes('timestamp')) {
					orderedColumns.push('timestamp');
					summaryColumns = summaryColumns.filter((col) => col !== 'timestamp');
				}

				orderedColumns = [...orderedColumns, ...summaryColumns];
				columns = orderedColumns.map(prettify);
			} else {
				columns = [];
			}

			// Set date range inputs to match the summary dates (when implemented in backend)
			if (summary?.date_range) {
				startDate = summary.date_range.start.split('T')[0]; // Extract YYYY-MM-DD
				endDate = summary.date_range.end.split('T')[0]; // Extract YYYY-MM-DD
			}
		} catch (err: any) {
			errorMsg = err?.message || 'Failed to run backtest.';
		} finally {
			running.set(false);
		}
	}
</script>

<!-- Main container for the backtest feature -->
<div class="feature-container backtest-container">
	<!-- Control panel for strategy selection, date range, and run button -->
	<div class="controls-container control-panel">
		<div class="strategy-selection">
			<div class="dropdown-container">
				<!-- Strategy dropdown component -->
				<StrategyDropdown bind:selectedId placeholder="Select strategy…" />
			</div>
			<!-- Run backtest button -->
			<button
				class="action-button run-btn"
				on:click={runBacktest}
				disabled={selectedId === null || selectedId === 'new' || $running}
			>
				{#if $running}
					<span class="spinner"></span> Running…
				{:else}
					<span class="play-icon">▶</span> Run Backtest
				{/if}
			</button>
		</div>

		<!-- Date range selection controls -->
		<div class="date-range-controls">
			<div class="date-input-group">
				<label for="start-date">Start Date</label>
				<input type="date" id="start-date" bind:value={startDate} placeholder="Start date" />
			</div>
			<div class="date-input-group">
				<label for="end-date">End Date</label>
				<input type="date" id="end-date" bind:value={endDate} placeholder="End date" />
			</div>
		</div>
	</div>

	<!-- Display error messages -->
	{#if errorMsg}
		<div class="error-container">
			<p class="error-message">{errorMsg}</p>
		</div>
	{/if}

	<!-- Display summary information after a backtest run -->
	{#if summary}
		<div class="summary-panel">
			<div class="summary-item">
				<span class="label">Results</span>
				<span class="value monospace">{summary.count}</span>
			</div>
			{#if summary.timeframe}
				<div class="summary-item">
					<span class="label">Timeframe</span>
					<span class="value">{summary.timeframe}</span>
				</div>
			{/if}
			{#if summary.date_range}
				<div class="summary-item">
					<span class="label">Period</span>
					<span class="value">
						{formatDate(summary.date_range.start)} — {formatDate(summary.date_range.end)}
					</span>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Container for the results list/table -->
	{#if $list !== null && $list.length}
		<div class="table-container results-container scrollable">
			<!-- Process any remaining numeric objects before passing to List -->
			{#each $list as item}
				{#each Object.keys(item) as key}
					{@const processed = processNumericObject(item[key])}
					{#if processed !== item[key]}
						{@const _ = item[key] = processed}
						<!-- Processed numeric object -->
					{/if}
				{/each}
			{/each}
			<!-- List component to display backtest instances -->
			<List {list} {columns} expandable={false} />
		</div>
	{:else if $list !== null && !$running && !errorMsg}
		<!-- Hint shown when no results are available yet -->
		<p class="hint">Select a strategy and click "Run Backtest" to fetch data.</p>
	{/if}
</div>

<style>
	/* Inherit feature container styles */
	.backtest-container {
		padding: 1rem; /* Add padding around the entire feature */
		gap: 1rem;
		display: flex;
		flex-direction: column;
		height: 100%;
	}

	/* Control panel styling */
	.control-panel {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding: 1rem;
		background-color: var(--ui-bg-secondary); /* Dark background */
		border: 1px solid var(--ui-border); /* Subtle border */
		border-radius: 6px;
		margin-bottom: 1rem; /* Space below controls */
	}

	.strategy-selection {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		width: 100%;
	}

	.dropdown-container {
		flex: 1; /* Allow dropdown to take available space */
	}

	/* Style the select element within the dropdown container */
	:global(.dropdown-container select) {
		width: 100%;
		padding: 8px 12px; /* Consistent padding */
		background: var(--ui-bg-element); /* Darker element background */
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		color: var(--text-primary); /* Light text */
		font-size: 14px;
		cursor: pointer;
		transition: all 0.2s ease;
	}

	:global(.dropdown-container select:hover) {
		border-color: var(--ui-accent);
	}

	:global(.dropdown-container select:focus) {
		outline: none;
		border-color: var(--ui-accent);
		box-shadow: 0 0 0 2px rgb(59 130 246 / 30%); /* Subtle focus ring */
	}

	/* Date range controls layout */
	.date-range-controls {
		display: flex;
		gap: 1rem;
		width: 100%;
	}

	.date-input-group {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		flex: 1; /* Each date input takes half the space */
	}

	/* Style labels for date inputs */
	.date-input-group label {
		font-size: 12px; /* Smaller label */
		font-weight: 500;
		color: var(--text-secondary); /* Lighter grey text */
		margin-bottom: 2px;
	}

	/* Style date input fields */
	[type='date'] {
		padding: 8px 12px;
		border: 1px solid var(--ui-border);
		border-radius: 4px;
		font-size: 14px;
		color: var(--text-primary);
		background-color: var(--ui-bg-element); /* Dark background */
		transition: border-color 150ms;
		font-family: var(--font-primary); /* Ensure consistent font */
	}

	/* Style for the date picker indicator */
	[type='date']::-webkit-calendar-picker-indicator {
		filter: invert(0.8); /* Make the calendar icon lighter */
		cursor: pointer;
	}

	[type='date']:focus {
		outline: none;
		border-color: var(--ui-accent); /* Highlight border on focus */
		box-shadow: 0 0 0 2px rgb(59 130 246 / 30%); /* Subtle focus ring */
	}

	/* Run button styling - uses action-button class */
	.run-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 0.5rem; /* Increased gap */
		font-weight: 500;
		white-space: nowrap;
		min-width: 140px; /* Slightly wider */
		height: 38px; /* Match input height */
		padding: 8px 16px; /* Match standard button padding */
	}

	/* Ensure action-button styles apply correctly */
	.run-btn.action-button {
		/* Styles are inherited from button.css */
	}

	.run-btn:disabled {
		/* Styles inherited from button.css */
		background-color: var(--c4); /* Use a grey background when disabled */
		border-color: var(--c4);
		color: var(--f2);
	}

	.play-icon {
		font-size: 0.7rem; /* Adjust icon size if needed */
		line-height: 1; /* Prevent extra spacing */
	}

	/* Spinner animation */
	.spinner {
		display: inline-block;
		width: 1em; /* Relative size */
		height: 1em;
		border: 2px solid rgb(255 255 255 / 30%); /* Lighter border */
		border-radius: 50%;
		border-top-color: var(--f1); /* White top border */
		animation: spin 0.8s linear infinite;
		margin-right: 0.35rem; /* Space between spinner and text */
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	/* Summary panel styling */
	.summary-panel {
		display: flex;
		flex-wrap: wrap;
		gap: 1.5rem; /* Space between summary items */
		padding: 0.75rem 1rem;
		background-color: var(--ui-bg-secondary); /* Dark background */
		border: 1px solid var(--ui-border);
		border-radius: 6px;
	}

	.summary-item {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	/* Use global label/value classes */
	.summary-item .label {
		/* Styles inherited from text.css */
		font-size: 12px; /* Ensure consistent size */
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.summary-item .value {
		/* Styles inherited from text.css */
		font-size: 1rem;
		font-weight: 600;
	}

	/* Results container styling */
	.results-container {
		border: 1px solid var(--ui-border);
		border-radius: 6px;
		overflow: hidden; /* Ensure List component respects border radius */
		flex-grow: 1; /* Allow results to take remaining space */
		background-color: var(--c2); /* Match main background */
		position: relative;
		min-height: 200px; /* Ensure there's always some space for results */
	}

	/* Add scrolling capability */
	.scrollable {
		overflow-y: auto;
		max-height: calc(100vh - 300px); /* Adjust based on your layout */
	}

	/* Ensure the List component inside fits well */
	:global(.results-container .list-component) {
		/* Adjust if List component has a different class */

		/* Add specific styles if needed, e.g., removing internal borders/backgrounds */
		border: none;
		background: none;
	}

	/* Error message container */
	.error-container {
		padding: 0.75rem 1rem;
		background-color: rgb(239 68 68 / 10%); /* Red background tint */
		border: 1px solid var(--c5); /* Red border */
		border-radius: 6px;
	}

	/* Use global error message class */
	.error-message {
		/* Styles inherited from text.css */
		color: var(--c5); /* Ensure red color */
		margin: 0; /* Remove default margins */
		font-size: 0.9rem;
	}

	/* Hint text styling */
	.hint {
		color: var(--text-secondary); /* Use secondary text color */
		font-size: 0.9rem;
		margin: 1rem 0;
		text-align: center;
		padding: 1.5rem;
		background-color: var(--ui-bg-secondary); /* Dark background */
		border: 1px solid var(--ui-border);
		border-radius: 6px;
	}
</style>
