<!-- instance.svelte -->
<script lang="ts" context="module">
	import '$lib/styles/global.css';
	import './input.css';
	import { privateRequest, publicRequest } from '$lib/utils/helpers/backend';
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
	import { isPublicViewing } from '$lib/utils/stores/stores';
	let currentSecurityResultRequest = 0;
	let loadedSecurityResultRequest = -1;
	let isLoadingSecurities = false;
	let filterOptions = [];

	let activePromiseReject: ((reason?: any) => void) | null = null;
	let isDocumentListenerActive = false; // Add guard for document listener
	
	// Only load security classifications if not in public viewing mode
	if (browser && !get(isPublicViewing)) {
		publicRequest<[]>('getSecurityClassifications', {}).then((v: []) => {
			filterOptions = v;
		}).catch((error) => {
			console.warn('Failed to load security classifications:', error);
			filterOptions = [];
		});
	}

	const inactiveInputQuery: InputQuery = {
		status: 'inactive',
		inputString: '',
		inputValid: true,
		inputType: '',
		requiredKeys: 'any',
		possibleKeys: [],
		instance: {},
		customTitle: undefined
	};
	export const inputQuery: Writable<InputQuery> = writable({ ...inactiveInputQuery });

	// Define determineInputType at module level
	export function determineInputType(inputString: string): void {
		const iQ = get(inputQuery);
		let inputType = iQ.inputType;

		// Only auto-classify input type if not already set (e.g., from forced type)
		if (inputString !== '' && !inputType) {
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
		} else if (inputString !== '' && inputType) {
			// If we have a forced input type and input string, handle special cases
			if (inputType === 'ticker' && inputString !== inputString.toUpperCase()) {
				// Convert ticker input to uppercase
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
		if (inputType === 'ticker') {
			isLoadingSecurities = true;

			// Extra update to ensure UI shows loading state immediately
			setTimeout(() => {
				inputQuery.update((v) => ({
					...v,
					securities: v.securities || [] // Preserve existing securities if any
				}));
			}, 0);
		}

		// Perform validation asynchronously in next event loop tick to avoid blocking UI
		setTimeout(() => {
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
		}, 0);
	}

	// Modified queryInstanceInput: if called while another query is active,
	// cancel the previous query (rejecting its promise) and reset the state.
	export async function queryInstanceInput(
		requiredKeys: InstanceAttributes[] | 'any',
		optionalKeys: InstanceAttributes[] | 'any',
		instance: Instance = {},
		forcedInputType?: string,
		customTitle?: string
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
			inputType: forcedInputType || '', // Set forced input type if provided
			customTitle: customTitle, // Set custom title if provided
			status: 'initializing'
		}));

		// If we have an initial input string, determine its type immediately (unless forced)
		if (initialInputString) {
			await tick(); // ensure UI is ready
			// Use setTimeout to ensure this runs after all other synchronous code
			setTimeout(() => {
				let initialType: string;
				
				// Only auto-detect type if no forced input type is provided
				if (!forcedInputType) {
					// First, forcibly detect the input type without waiting
					initialType = detectInputTypeSync(initialInputString, possibleKeys);

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
				} else {
					// If we have a forced input type, use it and validate accordingly
					initialType = forcedInputType;
					const currentState = get(inputQuery);
					if (forcedInputType === 'ticker') {
						isLoadingSecurities = true;
						inputQuery.update((q) => ({
							...q,
							securities: []
						}));
					}
					
					// Run validation with the forced type
					determineInputType(initialInputString);
				}

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
	} else if (forcedInputType) {
		// If no initial input string but we have a forced input type, set it up
		await tick();
		setTimeout(() => {
			inputQuery.update((q) => ({
				...q,
				inputType: forcedInputType,
				securities: forcedInputType === 'ticker' ? [] : q.securities
			}));
			
			// If forced to ticker type with no input, load popular tickers
			if (forcedInputType === 'ticker') {
				isLoadingSecurities = true;
				determineInputType(''); // This will trigger popular tickers loading
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
				if (activePromiseReject === reject) {
					activePromiseReject = null;
				}
				// Trigger a shutdown to reset state.
				inputQuery.update((v: InputQuery) => ({ ...v, status: 'shutdown' }));
			}
		});
	}

</script>

<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
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
			} else {
				// If no securities, at least set the ticker from input string
				iQ.instance.ticker = iQ.inputString.toUpperCase();
			}
		} else if (iQ.inputType === 'timeframe') {
			iQ.instance.timeframe = iQ.inputString;
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
		
		// Update the input string in the store immediately for responsive UI
		inputQuery.update((v) => ({
			...v,
			inputString: newValue
		}));
		
		// Make the API call non-blocking to avoid UI delays
		setTimeout(() => {
			determineInputType(newValue);
		}, 0);
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

	// Add helper function to safely manage document event listener
	function addDocumentListener() {
		if (!isDocumentListenerActive && browser) {
			document.body.removeEventListener('mousedown', handleOutsideClick); // Remove any existing
			document.body.addEventListener('mousedown', handleOutsideClick);
			document.body.setAttribute('data-input-click-listener', 'true');
			isDocumentListenerActive = true;
		}
	}

	function removeDocumentListener() {
		if (isDocumentListenerActive && browser) {
			document.body.removeEventListener('mousedown', handleOutsideClick);
			document.body.removeAttribute('data-input-click-listener');
			isDocumentListenerActive = false;
		}
	}

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
						addDocumentListener();
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
					removeDocumentListener();

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
					removeDocumentListener();

					// On cancellation we should also clear the inputString
					inputQuery.update((state) => ({
						...state,
						inputString: ''
					}));
				}
			}
		});

		if (browser && !get(isPublicViewing)) {
			type SecurityClassifications = {
				sectors: string[];
				industries: string[];
			};
			publicRequest<SecurityClassifications>('getSecurityClassifications', {}).then(
				(classifications: SecurityClassifications) => {
					sectors = classifications.sectors;
					industries = classifications.industries;
				}
			).catch((error) => {
				console.warn('Failed to load security classifications in onMount:', error);
				sectors = [];
				industries = [];
			});
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
				removeDocumentListener();

				unsubscribe();
			} catch (error) {
				console.error('Error removing event listeners:', error);
			}
		} else {
			// Just unsubscribe from the store when not in browser environment
			if (unsubscribe) unsubscribe();
		}
	});
	/*function displayValue(q: InputQuery, key: string): string {
		if (key === q.inputType) {
			return q.inputString;
		} else if (key in q.instance) {
			if (key === 'timestamp') {
				return UTCTimestampToESTString(q.instance.timestamp ?? 0);
			}  else {
				return String(q.instance[key as keyof Instance]);
			}
		}
		return '';
	} */

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
		<div class="content-container glass glass--rounded glass--responsive box-expand">
			{#if $inputQuery.inputType === 'timeframe'}
				<div class="timeframe-header-container">
					<div class="timeframe-title">Change Interval</div>
				</div>

				{:else if $inputQuery.inputType === 'ticker'}
					<div class="table-container">
						<div class="search-header">
							<span class="search-title">{$inputQuery.customTitle || 'Symbol Search'}</span>
						</div>
						<div class="search-divider"></div>
						{#if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
							{#if $inputQuery.inputString === '' || !$inputQuery.inputString}
								<div class="popular-section-header">
									<span class="popular-text">Popular</span>
								</div>
							{:else}
								<div class="securities-section-header">
									<span class="securities-text">Securities</span>
								</div>
							{/if}
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
								{:else if sec.ticker}
									<span class="default-ticker-icon">
										{sec.ticker.charAt(0).toUpperCase()}
									</span>
								{/if}
							</div>
										<div class="security-info-flex">
											<span class="ticker-flex">{sec.ticker}</span>
											<span class="name-flex">{sec.name}</span>
										</div>
									</div>
								{/each}
							</div>
						{:else if $inputQuery.inputString && $inputQuery.inputString.length > 0 && !isLoadingSecurities && loadedSecurityResultRequest !== -1 && loadedSecurityResultRequest === currentSecurityResultRequest}
							<div class="search-results-container">
								<div class="no-results">
									<span>No matching securities found</span>
								</div>
							</div>
						{/if}
					</div>
				{/if}
			</div>

		<div class="search-bar glass glass--pill glass--responsive search-bar-expand {$inputQuery.inputType === 'timeframe' && !$inputQuery.inputValid && $inputQuery.inputString ? 'error' : ''}">
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
