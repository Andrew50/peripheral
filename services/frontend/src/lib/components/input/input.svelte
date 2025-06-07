<!-- instance.svelte -->
<script lang="ts" context="module">
	import '$lib/styles/global.css';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { get, writable } from 'svelte/store';
	import { tick } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';
	// Ignore the $app/environment import error for now
	import { browser } from '$app/environment';
	import { capitalize, formatTimeframe, detectInputTypeSync, validateInput } from '$lib/components/input/utils/inputUtils';

	/**
	 * Focus Management Strategy:
	 * 1. When input component is activated, we store the previously focused element
	 * 2. We use a hidden input element that's positioned on top of the visible input to capture keyboard events
	 * 3. We add a document click handler to detect clicks outside the input window, which cancels the input
	 * 4. When input is completed or cancelled, we restore focus to the previously focused element
	 * 5. We clean up all event listeners when the component is destroyed or deactivated
	 *
	 * This approach prevents the input from capturing all keyboard events when it's not active.
	 */

	import { allKeys, type InstanceAttributes, type InputQuery } from '$lib/components/input/utils/inputTypes';
	let currentSecurityResultRequest = 0;
	let loadedSecurityResultRequest = -1;
	let isLoadingSecurities = false;
	let filterOptions = [];

	let activePromiseReject: ((reason?: any) => void) | null = null;
	privateRequest<[]>('getSecurityClassifications', {}).then((v: []) => {
		filterOptions = v;
	});

	const inactiveInputQuery: InputQuery = {
		status: 'inactive',
		inputString: '',
		inputValid: true,
		inputType: '',
		requiredKeys: 'any',
		possibleKeys: [],
		instance: {}
	};
	export const inputQuery: Writable<InputQuery> = writable({ ...inactiveInputQuery });

	// Define determineInputType at module level
	export function determineInputType(inputString: string): void {
		const iQ = get(inputQuery);
		let inputType = iQ.inputType;

		// Always auto-classify input type
		if (inputString !== '') {
			// Use our sync detection function for consistency
			inputType = detectInputTypeSync(inputString, iQ.possibleKeys);

			// If we detect a ticker, but the input is lowercase, convert to uppercase
			if (inputType === 'ticker' && inputString !== inputString.toUpperCase()) {
				// Update the input string with uppercase version
				setTimeout(() => {
					inputQuery.update((q) => ({
						...q,
						inputString: inputString.toUpperCase()
					}));
				}, 0);
			}
		} else {
			// If input is empty, keep the current inputType if it was already set
			// Only reset to empty if we were in an empty state to begin with
			if (iQ.inputType !== '') {
				inputType = iQ.inputType; // Keep current type
			} else {
				inputType = '';
			}
		}

		// Update the input type immediately
		inputQuery.update((v) => ({
			...v,
			inputType,
			// If we're switching to ticker type, initialize securities array if empty
			securities: inputType === 'ticker' && !v.securities ? [] : v.securities
		}));

		// Trigger validation after type is updated
		currentSecurityResultRequest++;
		const thisSecurityResultRequest = currentSecurityResultRequest;

		// Set isLoadingSecurities to true before validation starts
		if (inputType === 'ticker' && inputString.length > 0) {
			isLoadingSecurities = true;

			// Extra update to ensure UI shows loading state immediately
			setTimeout(() => {
				inputQuery.update((v) => ({
					...v,
					securities: v.securities || [] // Preserve existing securities if any
				}));
			}, 0);
		}

		// Perform validation asynchronously
		validateInput(inputString.toUpperCase(), inputType)
			.then((validationResp) => {
				if (thisSecurityResultRequest === currentSecurityResultRequest) {
					inputQuery.update((v: InputQuery) => ({
						...v,
						...validationResp
					}));
					loadedSecurityResultRequest = thisSecurityResultRequest;

					// Reset loading state after validation completes
					if (inputType === 'ticker') {
						isLoadingSecurities = false;
					}
				}
			})
			.catch((error) => {
				console.error('Validation error:', error);
				isLoadingSecurities = false;
			});
	}


	// Modified queryInstanceInput: if called while another query is active,
	// cancel the previous query (rejecting its promise) and reset the state.
	export async function queryInstanceInput(
		requiredKeys: InstanceAttributes[] | 'any',
		optionalKeys: InstanceAttributes[] | 'any',
		instance: Instance = {}
	): Promise<Instance> {
		// If an input query is already active, force its cancellation.
		if (get(inputQuery).status !== 'inactive') {
			if (activePromiseReject) {
				activePromiseReject(new Error('User cancelled input'));
				activePromiseReject = null;
			}
			inputQuery.update((q) => ({ ...inactiveInputQuery }));
			// Optionally wait a tick for the UI to update.
			await tick();
		}

		// Determine possible keys - always use a valid array of keys, never the string 'any'
		let possibleKeys: InstanceAttributes[] = [];
		if (optionalKeys === 'any') {
			possibleKeys = [...allKeys];
		} else if (Array.isArray(optionalKeys)) {
			if (Array.isArray(requiredKeys)) {
				possibleKeys = Array.from(
					new Set([...requiredKeys, ...optionalKeys])
				) as InstanceAttributes[];
			} else {
				possibleKeys = [...optionalKeys];
			}
		} else if (Array.isArray(requiredKeys)) {
			possibleKeys = [...requiredKeys];
		} else {
			// Default to all available keys if neither requiredKeys nor optionalKeys is an array
			possibleKeys = [...allKeys];
		}
		await tick();

		// Check if there's an initial inputString in the instance
		const initialInputString =
			'inputString' in instance && instance['inputString'] != null
				? String(instance['inputString'])
				: '';
		// Remove inputString property (not part of the Instance interface)
		if ('inputString' in instance) {
			const instanceAny = instance as any;
			delete instanceAny.inputString;
		}

		// Initialize the query with passed instance info.
		inputQuery.update((v: InputQuery) => ({
			...v,
			requiredKeys,
			possibleKeys,
			instance,
			inputString: initialInputString, // Use the initial input string if provided
			status: 'initializing'
		}));

		// If we have an initial input string, determine its type immediately
		if (initialInputString) {
			await tick(); // ensure UI is ready
			// Use setTimeout to ensure this runs after all other synchronous code
			setTimeout(() => {
				// First, forcibly detect the input type without waiting
				const initialType = detectInputTypeSync(initialInputString, possibleKeys);

				// Update the store synchronously with the detected type
				inputQuery.update((q) => ({
					...q,
					inputType: initialType,
					// Mark as loading if it's likely a ticker
					securities: initialType === 'ticker' ? [] : q.securities
				}));

				// If it looks like a ticker, explicitly set loading state
				if (initialType === 'ticker') {
					isLoadingSecurities = true;
				}

				// Then run the full determination with validation
				determineInputType(initialInputString);

				// For tickers, ensure we make multiple validation attempts
				if (initialType === 'ticker' || /^[A-Za-z]+$/.test(initialInputString)) {
					// First retry after a short delay
					setTimeout(() => {
						const currentInput = get(inputQuery).inputString;
						if (currentInput && currentInput.length > 0) {
							isLoadingSecurities = true;
							determineInputType(currentInput);

							// Second retry with longer delay for slow networks
							setTimeout(() => {
								const latestInput = get(inputQuery).inputString;
								if (
									latestInput &&
									latestInput.length > 0 &&
									loadedSecurityResultRequest !== currentSecurityResultRequest
								) {
									isLoadingSecurities = true;
									determineInputType(latestInput);
								}
							}, 1000); // Increased to 1000ms for better network reliability
						}
					}, 250); // Increased for better timing
				}
			}, 0);
		}

		// Wait for next tick to ensure UI updates
		await tick();

		// Return a new promise that resolves when input is complete or rejects on cancellation.
		return new Promise<Instance>((resolve, reject) => {
			// Save the reject function so a subsequent call can cancel this query.
			activePromiseReject = reject;

			const unsubscribe = inputQuery.subscribe((iQ: InputQuery) => {
				if (iQ.status === 'cancelled') {
					cleanup();
					reject(new Error('User cancelled input'));
				} else if (iQ.status === 'complete') {
					const result = iQ.instance;
					cleanup();
					resolve(result);
				}
			});

			function cleanup() {
				unsubscribe();
				activePromiseReject = null;
				// Trigger a shutdown to reset state.
				inputQuery.update((v: InputQuery) => ({ ...v, status: 'shutdown' }));
			}
		});
	}

</script>

<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { ESTStringToUTCTimestamp, UTCTimestampToESTString } from '$lib/utils/helpers/timestamp';
	let prevFocusedElement: HTMLElement | null = null;
	let highlightedIndex = -1;
	
	// Reactive statement to auto-highlight first result for ticker searches
	$: if ($inputQuery.inputType === 'ticker' && 
		  Array.isArray($inputQuery.securities) && 
		  $inputQuery.securities.length > 0 && 
		  highlightedIndex === -1) {
		highlightedIndex = 0;
	}

	async function enterInput(iQ: InputQuery, tickerIndex: number = 0): Promise<InputQuery> {
		if (iQ.inputType === 'ticker') {
			// Store the timestamp to preserve it
			const ts = iQ.instance.timestamp;

			// Wait for securities to load if needed
			if (loadedSecurityResultRequest !== currentSecurityResultRequest) {
				await waitForSecurityResult();
			}

			// Get the latest state after waiting
			iQ = $inputQuery;

			// Check if securities are available
			if (Array.isArray(iQ.securities) && iQ.securities.length > 0) {
				// Apply the selected security to the instance
				iQ.instance = { ...iQ.instance, ...iQ.securities[tickerIndex] };
				// Restore timestamp if it was previously set
				if (ts) iQ.instance.timestamp = ts;
			} else {
				// If no securities, at least set the ticker from input string
				iQ.instance.ticker = iQ.inputString.toUpperCase();
			}
		} else if (iQ.inputType === 'timeframe') {
			iQ.instance.timeframe = iQ.inputString;
		} else if (iQ.inputType === 'timestamp') {
			iQ.instance.timestamp = ESTStringToUTCTimestamp(iQ.inputString);
		}

		// Always clear the input string when a field is entered successfully
		iQ.inputString = '';

		// Mark as complete but then check if further input is needed.
		iQ.status = 'complete';
		if (iQ.requiredKeys === 'any') {
			if (Object.keys(iQ.instance).length === 0) {
				iQ.status = 'active';
			}
		} else if (Array.isArray(iQ.requiredKeys)) {
			for (const attribute of iQ.requiredKeys) {
				if (!iQ.instance[attribute]) {
					iQ.status = 'active';
					break;
				}
			}
		}

		// Only reset input type and validity if we're fully complete
		if (iQ.status === 'complete') {
			iQ.inputType = '';
			iQ.inputValid = true;
		}

		return iQ;
	}

	// Function to wait for security results to be loaded
	async function waitForSecurityResult(): Promise<void> {
		const startTime = Date.now();
		const maxWaitTime = 5000; // Maximum wait time in ms

		return new Promise<void>((resolve) => {
			const check = () => {
				// If the loaded request matches the current request, we're done
				if (loadedSecurityResultRequest === currentSecurityResultRequest) {
					isLoadingSecurities = false;
					resolve();
					return;
				}

				// If we've waited too long, resolve anyway
				if (Date.now() - startTime > maxWaitTime) {
					isLoadingSecurities = false;
					resolve();
					return;
				}

				// Check again after 50ms
				setTimeout(check, 50);
			};
			check();
		});
	}

	// Handle input changes (typing, pasting, etc.)
	function handleInputChange(event: Event) {
		const target = event.target as HTMLInputElement;
		const newValue = target.value;
		
		// Reset highlighted index when input changes, but set to 0 if we'll have securities
		const currentState = get(inputQuery);
		if (currentState.inputType === 'ticker' && newValue.length > 0) {
			highlightedIndex = 0; // Highlight first result by default for tickers
		} else {
			highlightedIndex = -1;
		}
		
		// Update the input string in the store
		inputQuery.update((v) => ({
			...v,
			inputString: newValue
		}));
		
		// Determine input type based on new value
		determineInputType(newValue);
	}

	// Handle special keys (Enter, Tab, Escape, Arrow keys)
	async function handleKeyDown(event: KeyboardEvent): Promise<void> {
		const currentState = get(inputQuery);
		
		// Make sure we're in active state
		if (currentState.status !== 'active') {
			return;
		}

		// Handle special keys
		if (event.key === 'Escape') {
			event.preventDefault();
			highlightedIndex = -1;
			inputQuery.update((q) => ({ ...q, status: 'cancelled' }));
			return;
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (currentState.inputValid) {
				// Use highlighted index if one is selected, otherwise use 0
				const selectedIndex = highlightedIndex >= 0 ? highlightedIndex : 0;
				const updatedQuery = await enterInput(currentState, selectedIndex);
				inputQuery.set(updatedQuery);
				highlightedIndex = -1; // Reset after selection
			}
			return;
		} else if (event.key === 'ArrowDown') {
			event.preventDefault();
			if (currentState.inputType === 'ticker' && currentState.securities && currentState.securities.length > 0) {
				highlightedIndex = Math.min(highlightedIndex + 1, currentState.securities.length - 1);
				scrollToHighlighted();
			}
			return;
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			if (currentState.inputType === 'ticker' && currentState.securities && currentState.securities.length > 0) {
				highlightedIndex = Math.max(highlightedIndex - 1, 0);
				scrollToHighlighted();
			}
			return;
		} else if (event.key === 'Tab') {
			event.preventDefault();
			inputQuery.update((q) => ({
				...q,
				instance: { ...q.instance, extendedHours: !q.instance.extendedHours }
			}));
			return;
		}
		
	}

	// onTouch handler (if needed) now removes the UI by updating via update() too.
	function onTouch(event: TouchEvent) {
		inputQuery.update((v: InputQuery) => ({ ...v, status: 'cancelled' }));
	}

	// Instead of repeatedly adding/removing listeners in the store subscription,
	// we add the keydown listener once on mount and remove it on destroy.
	let unsubscribe: () => void;
	let componentActive = false;

	// Define keydownHandler to call the handleKeyDown function
	const keydownHandler = (event: KeyboardEvent) => {
		handleKeyDown(event);
	};

	onMount(() => {
		if (browser) {
			prevFocusedElement = document.activeElement as HTMLElement;
		}

		unsubscribe = inputQuery.subscribe((v: InputQuery) => {
			if (browser) {
				if (v.status === 'initializing') {
					componentActive = true;
					// Store the currently focused element before focusing the input
					prevFocusedElement = document.activeElement as HTMLElement;

					// Focus the search input after a tick to allow rendering
					tick().then(() => {
						const searchInput = document.querySelector('.search-input') as HTMLInputElement;
						if (searchInput) {
							searchInput.focus();
							
							// Process initial input string if present
							if (v.inputString) {
								determineInputType(v.inputString);
							}
						}

						// Add a click handler to the document to detect clicks outside the popup
						if (browser) {
							document.body.removeEventListener('mousedown', handleOutsideClick);
							document.body.addEventListener('mousedown', handleOutsideClick);
							document.body.setAttribute('data-input-click-listener', 'true');
						}
					});
					
					// Use update() to mark that the UI is now active.
					inputQuery.update((state) => ({ ...state, status: 'active' }));
				} else if (v.status === 'shutdown') {
					componentActive = false;
					// Remove focus from the search input before restoring previous focus
					const searchInput = document.querySelector('.search-input') as HTMLInputElement;
					if (searchInput && document.activeElement === searchInput) {
						searchInput.blur();
					}

					// Remove document click handler when component is inactive
					if (browser) {
						document.body.removeEventListener('mousedown', handleOutsideClick);
						document.body.removeAttribute('data-input-click-listener');
					}

					// Restore focus and then update to inactive.
					if (prevFocusedElement && browser) {
						prevFocusedElement.focus();
					}

					// Clear the inputString only when we're fully shutting down
					inputQuery.update((state) => ({
						...state,
						status: 'inactive',
						inputString: ''
					}));
				} else if (v.status === 'cancelled') {
					componentActive = false;
					// Remove focus from the search input when cancelled
					const searchInput = document.querySelector('.search-input') as HTMLInputElement;
					if (searchInput && document.activeElement === searchInput) {
						searchInput.blur();
					}

					// Remove document click handler
					if (browser) {
						document.body.removeEventListener('mousedown', handleOutsideClick);
						document.body.removeAttribute('data-input-click-listener');
					}

					// On cancellation we should also clear the inputString
					inputQuery.update((state) => ({
						...state,
						inputString: ''
					}));
				}
			}
		});

		if (browser) {
			type SecurityClassifications = {
				sectors: string[];
				industries: string[];
			};
			privateRequest<SecurityClassifications>('getSecurityClassifications', {}, false).then(
				(classifications: SecurityClassifications) => {
					sectors = classifications.sectors;
					industries = classifications.industries;
				}
			);
		}
	});

	// Handle clicks outside the input window to cancel it
	function handleOutsideClick(event: MouseEvent) {
		if (!componentActive || !browser) return;

		const inputWindow = document.getElementById('input-window');
		const target = event.target as Node;

		// If we clicked outside the input window, cancel the input
		if (inputWindow && !inputWindow.contains(target)) {
			inputQuery.update((v) => ({ ...v, status: 'cancelled' }));
		}
	}

	onDestroy(() => {
		if (browser) {
			try {
				// Remove document click handler if it exists
				document.body.removeEventListener('mousedown', handleOutsideClick);
				document.body.removeAttribute('data-input-click-listener');

				unsubscribe();
			} catch (error) {
				console.error('Error removing event listeners:', error);
			}
		} else {
			// Just unsubscribe from the store when not in browser environment
			if (unsubscribe) unsubscribe();
		}
	});
	function displayValue(q: InputQuery, key: string): string {
		if (key === q.inputType) {
			return q.inputString;
		} else if (key in q.instance) {
			if (key === 'timestamp') {
				return UTCTimestampToESTString(q.instance.timestamp ?? 0);
			} else if (key === 'extendedHours') {
				return q.instance.extendedHours ? 'True' : 'False';
			} else {
				return String(q.instance[key as keyof Instance]);
			}
		}
		return '';
	}

	// Scroll highlighted item into view
	function scrollToHighlighted() {
		setTimeout(() => {
			const highlightedElement = document.querySelector('.security-item-flex.highlighted');
			if (highlightedElement) {
				highlightedElement.scrollIntoView({
					behavior: 'smooth',
					block: 'nearest'
				});
			}
		}, 0);
	}

	let sectors: string[] = [];
	let industries: string[] = [];
	
</script>
<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
{#if $inputQuery.status === 'active' || $inputQuery.status === 'initializing'}
	<div class="popup-container {$inputQuery.inputType === 'timeframe' ? 'timeframe-popup' : ''}" id="input-window" tabindex="-1" on:click|stopPropagation>
		<div class="content-container box-expand">
			{#if $inputQuery.inputType === 'timeframe'}
				<div class="timeframe-header-container">
					<div class="timeframe-title">Change Interval</div>
				</div>

				{:else if $inputQuery.inputType === 'ticker'}
					<div class="table-container">
						<div class="search-header">
							<span class="search-title">Symbol Search</span>
						</div>
						<div class="search-divider"></div>
						{#if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
							<div class="securities-list-flex securities-scrollable">
								{#each $inputQuery.securities as sec, i}
									<div
										class="security-item-flex {i === highlightedIndex ? 'highlighted' : ''}"
										on:click={async () => {
											const updatedQuery = await enterInput($inputQuery, i);
											inputQuery.set(updatedQuery);
										}}
										on:mouseenter={() => {
											highlightedIndex = i;
										}}
										on:mouseleave={() => {
											// Keep the highlight on the current item, don't reset
										}}
										role="button"
										tabindex="0"
										on:keydown={(e) => {
											if (e.key === 'Enter' || e.key === ' ') {
												e.currentTarget.click();
											}
										}}
									>
										<div class="security-icon-flex">
											{#if sec.icon}
												<img
													src={sec.icon.startsWith('data:')
														? sec.icon
														: `data:image/jpeg;base64,${sec.icon}`}
													alt="Security Icon"
													on:error={() => {}}
												/>
											{/if}
										</div>
										<div class="security-info-flex">
											<span class="ticker-flex">{sec.ticker}</span>
											<span class="name-flex">{sec.name}</span>
										</div>
									</div>
								{/each}
							</div>
						{:else if $inputQuery.inputString && $inputQuery.inputString.length > 0 && loadedSecurityResultRequest !== -1 && loadedSecurityResultRequest === currentSecurityResultRequest}
							<div class="no-results">
								<span>No matching securities found</span>
							</div>
						{/if}
					</div>
				{:else if $inputQuery.inputType === 'timestamp'}
					<div class="span-container">
						<div class="span-row">
							<span class="label">Timestamp</span>
							<input
								type="datetime-local"
								on:change={(e) => {
									// Use type casting in a different way that works better with Svelte compiler
									const target = e.target;
									const inputValue = target && 'value' in target ? String(target.value) : '';
									const date = new Date(inputValue);
									inputQuery.update((q) => ({
										...q,
										instance: {
											...q.instance,
											timestamp: !isNaN(date.getTime()) ? date.getTime() : q.instance.timestamp
										},
										inputValid: !isNaN(date.getTime())
									}));
								}}
							/>
						</div>
					</div>

				{:else if $inputQuery.inputType === 'extendedHours'}
					<div class="span-container extended-hours-container">
						<div class="span-row extended-hours-row">
							<span class="label">Market Hours <span class="hint"><kbd>Tab</kbd> to toggle</span></span>
							<div class="hours-buttons">
								<button
									class="toggle-button {!$inputQuery.instance.extendedHours ? 'active' : ''}"
									on:click={() => {
										inputQuery.update((q) => ({
											...q,
											instance: { ...q.instance, extendedHours: false }
										}));
									}}
								>
									Regular
								</button>
								<button
									class="toggle-button {$inputQuery.instance.extendedHours ? 'active' : ''}"
									on:click={() => {
										inputQuery.update((q) => ({
											...q,
											instance: { ...q.instance, extendedHours: true }
										}));
									}}
								>
									Extended
								</button>
							</div>
						</div>
					</div>
				{/if}
		</div>

		<div class="search-bar search-bar-expand {$inputQuery.inputType === 'timeframe' && !$inputQuery.inputValid && $inputQuery.inputString ? 'error' : ''}">
			<div class="search-icon">
				<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M21 21L16.514 16.506L21 21ZM19 10.5C19 15.194 15.194 19 10.5 19C5.806 19 2 15.194 2 10.5C2 5.806 5.806 2 10.5 2C15.194 2 19 5.806 19 10.5Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</div>
			<input
				type="text"
				placeholder={$inputQuery.inputType === 'timeframe' ? '' : 'Search'}
				bind:value={$inputQuery.inputString}
				on:input={handleInputChange}
				on:keydown={handleKeyDown}
				class="search-input"
				autocomplete="off"
				spellcheck="false"
			/>
		</div>

		{#if $inputQuery.inputType === 'timeframe'}
			<div class="timeframe-preview-below">
				{#if $inputQuery.inputString}
					{#if $inputQuery.inputValid}
						<span class="preview-text-below">{formatTimeframe($inputQuery.inputString)}</span>
					{:else}
						<span class="preview-text-below error">Invalid format</span>
					{/if}
				{:else}
					<span class="preview-text-below hint">e.g., 1m, 5h, 1d</span>
				{/if}
			</div>
		{/if}

	</div>
{/if}

<style>
	#input-window.popup-container {
		width: min(90vw, 45rem);
		height: auto;
		max-height: 70vh;
		background: transparent;
		border: none;
		border-radius: 0;
		display: flex;
		flex-direction: column;
		overflow: visible;
		box-shadow: none;
		position: fixed !important;
		bottom: max(3vh, 50px) !important;
		left: 50% !important;
		top: auto !important;
		transform: translateX(-50%) !important;
		z-index: 99999 !important;
		gap: max(0.5vh, 8px);
	}

	#input-window.timeframe-popup {
		top: 50% !important;
		bottom: auto !important;
		transform: translate(-50%, -50%) !important;
		width: min(20vw, 250px);
		min-width: 200px;
	}

	.timeframe-popup .content-container,
	.timeframe-popup .search-bar {
		width: 100%;
		margin-left: auto;
		margin-right: auto;
		transform-origin: center;
	}

	.search-bar {
		display: flex;
		align-items: center;
		height: max(3.5rem, 7vh);
		background: rgba(0, 0, 0, 0.4);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: max(1.75rem, 3.5vh);
		padding: 0 max(0.25rem, 0.5vh);
		position: relative;
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
		backdrop-filter: var(--backdrop-blur);
	}

	.timeframe-popup .search-bar {
		border-radius: 0 0 max(0.75rem, 1.5vh) max(0.75rem, 1.5vh);
		height: max(3.5rem, 7vh);
		margin-top: 0;
	}

	.search-icon {
		padding: max(0.75rem, 1.5vh) max(0.25rem, 0.5vh) max(0.75rem, 1.5vh) max(1.25rem, 2.5vh);
		display: flex;
		align-items: center;
		color: #ffffff;
		position: absolute;
		left: max(0.5rem, 1vh);
		z-index: 1;
		filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.8));
	}

	.search-icon svg {
		width: max(1.125rem, 2.25vh);
		height: max(1.125rem, 2.25vh);
		opacity: 1;
	}

	.search-bar input {
		flex: 1;
		background: transparent;
		border: none;
		border-radius: max(1.5rem, 3vh);
		padding: max(0.75rem, 1.5vh) max(1rem, 2vh) max(0.75rem, 1.5vh) max(3rem, 6vh);
		color: #ffffff;
		font-size: max(1rem, 2vh);
		margin: max(0.5rem, 1vh);
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.timeframe-popup .search-icon {
		display: none;
	}

	.timeframe-popup .search-bar input {
		padding: max(0.75rem, 1.5vh) max(1rem, 2vh);
		text-align: center;
		font-size: max(1.125rem, 2.25vh);
		font-weight: 600;
	}

	.timeframe-popup .search-bar:focus-within {
		border-color: #4a80f0;
		box-shadow: 0 0 0 2px rgba(74, 128, 240, 0.2), 0 8px 32px rgba(0, 0, 0, 0.5);
	}

	.search-bar input:focus {
		outline: none;
	}

	.search-bar input::placeholder {
		color: rgba(255, 255, 255, 0.9);
		opacity: 1;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.timeframe-popup .search-bar.error {
		border: 1px solid #ff4444 !important;
		box-shadow: 0 0 8px rgba(255, 68, 68, 0.3);
	}



	.content-container {
		background: rgba(0, 0, 0, 0.5);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: max(0.75rem, 1.5vh);
		overflow-y: auto;
		padding: max(0.5rem, 1vh);
		height: min(30vh, 15rem);
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
		backdrop-filter: var(--backdrop-blur);
		scrollbar-width: thin;
		scrollbar-color: rgba(255, 255, 255, 0.3) transparent;
	}

	.timeframe-popup .content-container {
		height: auto;
		min-height: max(3.75rem, 7.5vh);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: max(1rem, 2vh);
		border-radius: max(0.75rem, 1.5vh) max(0.75rem, 1.5vh) 0 0;
		margin-bottom: 0;
	}

	.timeframe-header-container {
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: max(0.5rem, 1vh);
	}

	.timeframe-popup .timeframe-title {
		color: #ffffff;
		font-size: max(1.25rem, 2.5vh);
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.timeframe-preview-below {
		text-align: center;
		margin-top: max(0.5rem, 1vh);
		width: 100%;
		margin-left: auto;
		margin-right: auto;
	}

	.preview-text-below {
		color: rgba(255, 255, 255, 0.8);
		font-size: max(0.75rem, 1.5vh);
		font-weight: 400;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.preview-text-below.error {
		color: #ff6b6b;
	}

	.preview-text-below.hint {
		color: rgba(255, 255, 255, 0.5);
	}

	.securities-list-flex {
		display: flex;
		flex-direction: column;
		gap: max(0.25rem, 0.5vh);
	}

	.securities-scrollable {
		max-height: min(22.5vh, 11.25rem);
		overflow-y: auto;
		scrollbar-width: thin;
		scrollbar-color: rgba(255, 255, 255, 0.3) transparent;
	}

	.security-item-flex {
		display: flex;
		align-items: center;
		padding: max(0.25rem, 0.5vh) max(0.375rem, 0.75vh);
		cursor: pointer;
		border-radius: max(0.375rem, 0.75vh);
		border: 1px solid transparent;
		transition: background-color 0.15s ease, border-color 0.15s ease;
		gap: max(0.5rem, 1vh);
		min-height: max(2.25rem, 4.5vh);
	}

	.security-item-flex.highlighted {
		background-color: rgba(255, 255, 255, 0.2);
		backdrop-filter: blur(8px);
	}

	.security-icon-flex {
		width: max(2rem, 4vh);
		height: max(1.25rem, 2.5vh);
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden;
	}

	.security-icon-flex img {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain;
	}

	.security-info-flex {
		flex: 1;
		display: flex;
		align-items: baseline;
		gap: max(0.5rem, 1vh);
		overflow: hidden;
		font-size: max(0.75rem, 1.5vh);
		white-space: nowrap;
	}

	.ticker-flex {
		font-weight: 600;
		color: #ffffff;
		flex-basis: max(3.125rem, 6.25vh);
		flex-shrink: 0;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.name-flex {
		color: #ffffff;
		flex-grow: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: max(6.25rem, 12.5vh);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.no-results {
		display: flex;
		align-items: center;
		justify-content: center;
		height: min(25vh, 12.5rem);
		color: #ffffff;
		font-size: max(0.875rem, 1.75vh);
		text-align: center;
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.search-header {
		padding: max(0.5rem, 1vh) max(0.75rem, 1.5vh) max(0.25rem, 0.5vh) max(0.75rem, 1.5vh);
		display: flex;
		align-items: center;
	}

	.search-title {
		color: #ffffff;
		font-size: max(0.875rem, 1.75vh);
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		opacity: 0.9;
	}

	.search-divider {
		height: 1px;
		background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.3), transparent);
		margin: 0 max(0.5rem, 1vh) max(0.5rem, 1vh) max(0.5rem, 1vh);
	}



	.box-expand,
	.search-bar-expand {
		animation-duration: 0.15s;
		animation-timing-function: ease-out;
		animation-fill-mode: both;
		transform-origin: center;
	}

	.box-expand {
		animation-name: boxExpand;
	}

	.search-bar-expand {
		animation-name: searchBarExpand;
	}

	@keyframes boxExpand {
		from {
			transform: scale(0.85);
			opacity: 0.6;
		}
		to {
			transform: scale(1);
			opacity: 1;
		}
	}

	@keyframes searchBarExpand {
		from {
			transform: scaleX(0.3);
			opacity: 0.4;
		}
		to {
			transform: scaleX(1);
			opacity: 1;
		}
	}
</style>
