<!-- instance.svelte -->
<script lang="ts" context="module">
	import '$lib/styles/global.css';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { get, writable } from 'svelte/store';
	// Ignore the date-fns import error for now as it's likely installed in the full environment
	import { parse } from 'date-fns';
	import { tick } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/utils/types/types';
	// Ignore the $app/environment import error for now
	import { browser } from '$app/environment';

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

	const allKeys = ['ticker', 'timestamp', 'timeframe', 'extendedHours'] as const;
	let currentSecurityResultRequest = 0;
	let loadedSecurityResultRequest = -1;
	let manualInputType: string = 'auto';
	let isLoadingSecurities = false;

	type InstanceAttributes = (typeof allKeys)[number];
	let filterOptions = [];

	privateRequest<[]>('getSecurityClassifications', {}).then((v: []) => {
		filterOptions = v;
	});
	interface InputQuery {
		// 'inactive': no UI shown
		// 'initializing' setting up event handlers
		// 'active': window is open waiting for input
		// 'complete': one field completed (may still be active if more required)
		// 'cancelled': user cancelled via Escape
		// 'shutdown': about to close and reset to inactive
		status: 'inactive' | 'initializing' | 'active' | 'complete' | 'cancelled' | 'shutdown';
		inputString: string;
		inputType: string;
		inputValid: boolean;
		instance: Instance;
		requiredKeys: InstanceAttributes[] | 'any';
		possibleKeys: InstanceAttributes[];
		securities?: Instance[];
	}

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

	// Move validateInput here
	async function validateInput(
		inputString: string,
		inputType: string
	): Promise<{
		inputValid: boolean;
		securities: Instance[];
	}> {
		if (inputType === 'ticker') {
			isLoadingSecurities = true;

			try {
				// Add a small delay to avoid too many rapid requests during typing
				await new Promise((resolve) => setTimeout(resolve, 10));

				const securities = await privateRequest<Instance[]>('getSecuritiesFromTicker', {
					ticker: inputString
				});

				if (Array.isArray(securities) && securities.length > 0) {
					return {
						inputValid: true,
						securities: securities
					};
				}
				return { inputValid: false, securities: [] };
			} catch (error) {
				console.error('Error fetching securities:', error);
				// Return empty results but mark as valid if we have some input
				// This allows the UI to stay responsive even if backend request fails
				return {
					inputValid: inputString.length > 0,
					securities: []
				};
			} finally {
				isLoadingSecurities = false;
			}
		} else if (inputType === 'timeframe') {
			const regex = /^\d{1,3}[yqmwhds]?$/i;
			return { inputValid: regex.test(inputString), securities: [] };
		} else if (inputType === 'timestamp') {
			const formats = ['yyyy-MM-dd H:m:ss', 'yyyy-MM-dd H:m', 'yyyy-MM-dd H', 'yyyy-MM-dd'];
			for (const format of formats) {
				try {
					const parsedDate = parse(inputString, format, new Date());
					if (!isNaN(parsedDate.getTime())) {
						return { inputValid: true, securities: [] };
					}
				} catch {
					/* try next format */
				}
			}
			return { inputValid: false, securities: [] };
		}
		return { inputValid: false, securities: [] };
	}

	// Define determineInputType at module level
	export function determineInputType(inputString: string): void {
		const iQ = get(inputQuery);
		let inputType = iQ.inputType;

		// Only auto-classify if manualInputType is set to 'auto'
		if (manualInputType === 'auto') {
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
				inputType = '';
			}
		} else {
			// Use the manually selected input type
			inputType = manualInputType;
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

	// Hold the reject function of the currently active promise (if any)
	let activePromiseReject: ((reason?: any) => void) | null = null;

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

	// Add this new helper function above the existing determineInputType function
	function detectInputTypeSync(
		inputString: string,
		possibleKeysArg: InstanceAttributes[] | 'any'
	): string {
		// Make sure we have a valid array of possible keys
		const possibleKeys = Array.isArray(possibleKeysArg) ? possibleKeysArg : [...allKeys];

		if (!inputString || inputString === '') {
			return '';
		}

		// Test for ticker - check for uppercase letters
		if (possibleKeys.includes('ticker') && /^[A-Z]+$/.test(inputString)) {
			return 'ticker';
		} else if (possibleKeys.includes('timeframe') && /^\d{1,2}[hdwmqs]?$/i.test(inputString)) {
			return 'timeframe';
		} else if (possibleKeys.includes('timestamp') && /^[\d-]+$/.test(inputString)) {
			return 'timestamp';
		} else if (possibleKeys.includes('ticker') && /^[a-zA-Z]+$/.test(inputString)) {
			// Default to ticker for any alphabetic input if ticker is possible
			return 'ticker';
		}

		return '';
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
			// Reset manualInputType to auto after input is entered
			manualInputType = 'auto';
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
			}
			return;
		} else if (event.key === 'ArrowUp') {
			event.preventDefault();
			if (currentState.inputType === 'ticker' && currentState.securities && currentState.securities.length > 0) {
				highlightedIndex = Math.max(highlightedIndex - 1, 0);
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
		
		// For all other keys, let the normal input behavior handle it
		// The handleInputChange function will be called automatically
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
								// Ensure we're starting with auto detection for initial strings
								manualInputType = 'auto';
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

	function closeWindow() {
		inputQuery.update((v) => ({ ...v, status: 'cancelled' }));
	}

	let sectors: string[] = [];
	let industries: string[] = [];
	
	function capitalize(str: string, lower = false): string {
		return (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, (match: string) =>
			match.toUpperCase()
		);
	}

	function formatTimeframe(timeframe: string): string {
		const match = timeframe.match(/^(\d+)([dwmsh]?)$/i) ?? null;
		let result = timeframe;
		if (match) {
			switch (match[2]) {
				case 'd':
					result = `${match[1]} days`;
					break;
				case 'w':
					result = `${match[1]} weeks`;
					break;
				case 'm':
					result = `${match[1]} months`;
					break;
				case 'h':
					result = `${match[1]} hours`;
					break;
				case 's':
					result = `${match[1]} seconds`;
					break;
				default:
					result = `${match[1]} minutes`;
					break;
			}
			if (match[1] === '1') {
				result = result.slice(0, -1);
			}
		}
		return result;
	}
</script>

{#if $inputQuery.status === 'active' || $inputQuery.status === 'initializing'}
	<div class="popup-container" id="input-window" tabindex="-1" on:click|stopPropagation>
		<div class="header">
			<div class="title">{capitalize($inputQuery.inputType)} Search</div>
			<div class="field-buttons">
				<button
					class="toggle-button {manualInputType === 'auto' && $inputQuery.inputType === ''
						? 'active'
						: ''}"
					on:click|stopPropagation={async () => {
						manualInputType = 'auto';
						inputQuery.update((v) => ({
							...v,
							inputType: '',
							inputString: '',
							inputValid: true
						}));
						await tick(); // Wait for next UI update cycle
						console.log('After Auto click, inputType is:', get(inputQuery).inputType); // Log state
					}}
				>
					Auto
				</button>
				{#if Array.isArray($inputQuery.possibleKeys)}
					{#each $inputQuery.possibleKeys as key}
						<button
							class="toggle-button {manualInputType === key ? 'active' : ''}"
							on:click|stopPropagation={() => {
								manualInputType = key;
								inputQuery.update((v) => ({
									...v,
									inputType: manualInputType,
									inputValid: true // Reset validity when manually changing type
								}));
							}}
						>
							{capitalize(key)}
						</button>
					{/each}
				{/if}
			</div>
			<button class="utility-button" on:click|stopPropagation={closeWindow}>×</button>
		</div>

		<div class="content-container">
			{#if true}
				{#if $inputQuery.inputType === ''}
					<div class="span-container">
						{#if Array.isArray($inputQuery.possibleKeys)}
							{#each $inputQuery.possibleKeys as key}
								{#if key === 'extendedHours'}
									<!-- Render the specific row for extendedHours -->
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
								{:else}
									<!-- Render standard row for other keys -->
									<button type="button" 
										class="span-row"
										on:click={() => {
											// Logic to select the key
											manualInputType = key;
											inputQuery.update((v) => ({
												...v,
												inputType: manualInputType,
												inputValid: true // Reset validity when manually changing type
											}));
										}}
									>
										<span
											class={Array.isArray($inputQuery.requiredKeys) &&
											$inputQuery.requiredKeys.includes(key) &&
											!$inputQuery.instance[key]
												? 'red'
												: ''}
										>
											{capitalize(key)}
										</span>
										<span class="value">
											{displayValue($inputQuery, key)}
										</span>
									</button>
								{/if}
							{/each}
						{/if}
					</div>
				{:else if $inputQuery.inputType === 'ticker'}
					<div class="table-container">
						{#if isLoadingSecurities}
							<div class="loading-container">
								<div class="loading-spinner"></div>
								<span class="loading-text">Loading securities...</span>
							</div>
						{:else if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
							<div class="securities-list-flex">
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
						{:else if $inputQuery.inputString && $inputQuery.inputString.length > 0}
							<!-- Show initially blank loading state until load state is set -->
							{#if loadedSecurityResultRequest === -1 || loadedSecurityResultRequest !== currentSecurityResultRequest}
								<div class="loading-container">
									<div class="loading-spinner"></div>
									<span class="loading-text">Loading securities...</span>
								</div>
							{:else}
								<div class="no-results">
									<span>No matching securities found</span>
								</div>
							{/if}
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
				{:else if $inputQuery.inputType === 'timeframe'}
					<div class="span-container">
						<div class="span-row">
							<span class="label">Timeframe</span>
							<span class="value">{formatTimeframe($inputQuery.inputString)}</span>
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
			{/if}
		</div>

		<div class="search-bar">
			<div class="search-icon">
				<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
					<path d="M21 21L16.514 16.506L21 21ZM19 10.5C19 15.194 15.194 19 10.5 19C5.806 19 2 15.194 2 10.5C2 5.806 5.806 2 10.5 2C15.194 2 19 5.806 19 10.5Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</div>
			<input
				type="text"
				placeholder="Search"
				bind:value={$inputQuery.inputString}
				on:input={handleInputChange}
				on:keydown={handleKeyDown}
				class="search-input"
				autocomplete="off"
				spellcheck="false"
			/>
		</div>
	</div>
{/if}

<style>
	#input-window.popup-container {
		width: 90vw;
		max-width: 600px;
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
		bottom: 50px !important;
		left: 50% !important;
		top: auto !important;
		transform: translateX(-50%) !important;
		z-index: 99999 !important;
		gap: 8px;
	}

	.header {
		display: none;
	}



	.search-bar {
		display: flex;
		align-items: center;
		height: 56px;
		background: rgba(0, 0, 0, 0.4);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 28px;
		padding: 0 4px;
		position: relative;
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
		backdrop-filter: var(--backdrop-blur);
	}

	.search-icon {
		padding: 12px 4px 12px 20px;
		display: flex;
		align-items: center;
		color: #ffffff;
		position: absolute;
		left: 8px;
		z-index: 1;
		filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.8));
	}

	.search-icon svg {
		width: 18px;
		height: 18px;
		opacity: 1;
	}

	.search-bar input {
		flex: 1;
		background: transparent;
		border: none;
		border-radius: 24px;
		padding: 12px 16px 12px 48px;
		color: #ffffff;
		font-size: 16px;
		margin: 8px;
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.search-bar input:focus {
		outline: none;
	}

	.search-bar input::placeholder {
		color: rgba(255, 255, 255, 0.9);
		opacity: 1;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.content-container {
		background: rgba(0, 0, 0, 0.5);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 12px;
		overflow-y: auto;
		padding: 8px;
		height: 240px;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
		backdrop-filter: var(--backdrop-blur);
		scrollbar-width: thin;
		scrollbar-color: rgba(255, 255, 255, 0.3) transparent;
	}





	.securities-list-flex {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}

	.security-item-flex {
		display: flex;
		align-items: center;
		padding: 4px 6px;
		cursor: pointer;
		border-radius: 6px;
		border: 1px solid transparent;
		transition: background-color 0.15s ease, border-color 0.15s ease;
		gap: 8px;
		min-height: 36px;
	}

	.security-item-flex.highlighted {
		background-color: rgba(255, 255, 255, 0.2);
		backdrop-filter: blur(8px);
	}

	.security-icon-flex {
		width: 32px;
		height: 20px;
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
		gap: 8px;
		overflow: hidden;
		font-size: 12px;
		white-space: nowrap;
	}

	.ticker-flex {
		font-weight: 600;
		color: #ffffff;
		flex-basis: 50px;
		flex-shrink: 0;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.name-flex {
		color: #ffffff;
		flex-grow: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: 100px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}



	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 30px;
		height: 200px;
	}

	.loading-spinner {
		width: 30px;
		height: 30px;
		border: 3px solid rgba(255, 255, 255, 0.3);
		border-top: 3px solid #ffffff;
		border-radius: 50%;
		animation: spin 1s linear infinite;
		margin-bottom: 10px;
	}

	.loading-text {
		color: #ffffff;
		font-size: 14px;
		text-align: center;
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.no-results {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 200px;
		color: #ffffff;
		font-size: 14px;
		text-align: center;
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	@keyframes spin {
		0% { transform: rotate(0deg); }
		100% { transform: rotate(360deg); }
	}
</style>
