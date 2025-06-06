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
		
		// Update the input string in the store
		inputQuery.update((v) => ({
			...v,
			inputString: newValue
		}));
		
		// Determine input type based on new value
		determineInputType(newValue);
	}

	// Handle special keys (Enter, Tab, Escape)
	async function handleKeyDown(event: KeyboardEvent): Promise<void> {
		const currentState = get(inputQuery);
		
		// Make sure we're in active state
		if (currentState.status !== 'active') {
			return;
		}

		// Handle special keys
		if (event.key === 'Escape') {
			event.preventDefault();
			inputQuery.update((q) => ({ ...q, status: 'cancelled' }));
			return;
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (currentState.inputValid) {
				const updatedQuery = await enterInput(currentState, 0);
				inputQuery.set(updatedQuery);
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

		<div class="search-bar">
			<input
				type="text"
				placeholder="Enter ticker, timestamp, timeframe..."
				bind:value={$inputQuery.inputString}
				on:input={handleInputChange}
				on:keydown={handleKeyDown}
				class="search-input"
				autocomplete="off"
				spellcheck="false"
			/>
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
										class="security-item-flex"
										on:click={async () => {
											const updatedQuery = await enterInput($inputQuery, i);
											inputQuery.set(updatedQuery);
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
											<span class="timestamp-flex"
												>{sec.timestamp !== undefined
													? UTCTimestampToESTString(sec.timestamp)
													: ''}</span
											>
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
	</div>
{/if}

<style>
	.popup-container {
		/* Responsive sizing */
		width: 90vw; /* Use viewport width */
		max-width: 700px; /* Max width */
		height: 85vh; /* Use viewport height */
		max-height: 600px; /* Max height */
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		overflow: hidden; /* Keep hidden to manage internal scrolling */
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		z-index: 10000; /* Ensure this is higher than the drawing menu */
	}

	/* Button styles for field selection */
	.field-buttons {
		display: flex;
		gap: 4px;
		flex-wrap: nowrap;
		margin-left: auto;
		/* Remove max-width: 75%; */
		justify-content: flex-end;
		overflow-x: auto; /* Allows horizontal scrolling */
		white-space: nowrap;
		padding-left: 10px; /* Keep padding */
		/* Add scrollbar styling for better UX */
		scrollbar-width: thin;
		scrollbar-color: var(--ui-border) transparent;
	}

	.toggle-button {
		margin-right: 0;
		padding: 4px 8px;
		min-width: 60px;
		height: 26px;
		font-weight: 500;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.3px;
		transition: all 0.2s ease;
		border-radius: 4px;
		position: relative;
		overflow: hidden;
		background: transparent;
		border: 1px solid var(--ui-border);
		color: var(--text-secondary);
		cursor: pointer;
	}

	.toggle-button:after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 0;
		width: 0;
		height: 2px;
		background: var(--ui-accent, #4a80f0);
		transition: width 0.2s ease;
	}

	.toggle-button:hover {
		background: var(--ui-bg-hover);
	}

	.toggle-button:hover:after {
		width: 100%;
	}

	.toggle-button.active {
		background: var(--ui-bg-hover);
		border-bottom: 2px solid var(--ui-accent, #4a80f0);
		color: var(--text-primary);
	}

	.toggle-button.active:after {
		width: 100%;
	}

	.span-container {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 100%;
	}
	.span-container span {
		align-items: top;
		display: block;
		flex-direction: row;
		width: 100%;
		font-size: 30px;
	}
	.span-row {
		/* Each row is a flex container, left label and right value */
		display: flex;
		flex-direction: row;
		justify-content: space-between;
		/* optionally align items on the baseline or center */
		align-items: baseline;
	}

	.span-row span {
		/* Let it inherit the global font instead of forcing a size */
		font-size: inherit;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 8px 12px;
		border-bottom: 1px solid var(--ui-border);
		height: 40px;
	}

	.title {
		font-size: 16px;
		font-weight: 500;
		color: var(--text-primary);
	}

	.search-bar {
		padding: 8px 12px;
		display: flex;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid var(--ui-border);
		height: 48px;
		position: relative;
	}

	.search-bar input {
		flex: 1;
		background: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		padding: 6px 10px;
		color: var(--text-primary);
		border-radius: 4px;
		font-size: 14px; /* Ensure consistent font size */
	}

	.search-bar input:focus {
		outline: none;
		border-color: var(--ui-accent, #4a80f0);
		box-shadow: 0 0 0 2px rgba(74, 128, 240, 0.2);
	}

	/* Styles for Flexbox security list */
	.content-container {
		flex: 1; /* Allow container to grow and shrink */
		overflow-y: auto; /* Scroll within this container */
		padding: 8px 12px; /* Add padding */
	}

	.table-container { /* Style the container for the flex list */
		height: 100%;
		display: flex;
		flex-direction: column;
	}

	.securities-list-flex {
		display: flex;
		flex-direction: column;
		gap: 4px; /* Spacing between items */
	}

	.security-item-flex {
		display: flex;
		align-items: center;
		padding: 6px 8px;
		cursor: pointer;
		border-radius: 4px;
		border: 1px solid transparent; /* For hover effect */
		transition: background-color 0.15s ease, border-color 0.15s ease;
		gap: 12px; /* Space between icon and info */
	}

	.security-item-flex:hover {
		background-color: var(--ui-bg-hover);
		border-color: var(--ui-border);
	}

	.security-icon-flex {
		width: 60px; /* Fixed width for icon container */
		height: 30px;
		flex-shrink: 0; /* Prevent shrinking */
		display: flex;
		align-items: center;
		justify-content: center;
		overflow: hidden; /* Hide overflow if icon is too large */
	}

	.security-icon-flex img {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain; /* Scale icon while preserving aspect ratio */
	}

	.security-info-flex {
		flex: 1; /* Take remaining space */
		display: flex;
		align-items: baseline; /* Align text baselines */
		gap: 10px; /* Space between ticker, name, timestamp */
		overflow: hidden; /* Prevent content from overflowing the item */
		font-size: 13px; /* Base font size */
		white-space: nowrap; /* Prevent wrapping within info */
	}

	.ticker-flex {
		font-weight: 600;
		color: var(--text-primary);
		flex-basis: 70px; /* Give ticker a base width */
		flex-shrink: 0; /* Don't shrink ticker */
	}

	.name-flex {
		color: var(--text-secondary);
		flex-grow: 1; /* Allow name to take up available space */
		overflow: hidden; /* Hide overflow */
		text-overflow: ellipsis; /* Add ellipsis for long names */
		min-width: 100px; /* Ensure name has some minimum width */
	}

	.timestamp-flex {
		color: var(--text-secondary);
		font-size: 0.9em; /* Slightly smaller timestamp */
		flex-shrink: 0; /* Don't shrink timestamp */
		margin-left: auto; /* Push timestamp to the right */
		padding-left: 10px; /* Add padding */
	}

	.loading-spinner {
		width: 30px;
		height: 30px;
		border: 3px solid var(--ui-border);
		border-top: 3px solid var(--text-primary);
		border-radius: 50%;
		animation: spin 1s linear infinite;
		margin-bottom: 10px;
	}

	.loading-text {
		color: var(--text-secondary);
		font-size: 14px;
		text-align: center;
		display: block;
	}

	.loading-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		padding: 30px;
		height: 200px;
	}

	.no-results {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 200px;
		color: var(--text-secondary);
		font-size: 14px;
		text-align: center;
	}

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}

	/* Added styles for extended hours buttons */
	.extended-hours-container .span-row {
		align-items: center; /* Center align items vertically */
		justify-content: flex-start; /* Align items to the start */
		gap: 15px; /* Add space between label and button group */
	}

	.extended-hours-container .hours-buttons {
		display: flex;
		gap: 8px; /* Space between buttons */
	}

	.extended-hours-container .hours-buttons .toggle-button {
		/* Reuse existing toggle-button styles */
		/* Add specific adjustments if needed */
		min-width: 80px; /* Give buttons a bit more width */
	}

	.extended-hours-container .label .hint {
		font-size: 0.8em; /* Smaller font size */
		color: var(--text-secondary); /* Lighter color */
		font-weight: 400; /* Normal weight */
		margin-left: 4px; /* Small space from label */
	}

	.extended-hours-container .label .hint kbd {
		background-color: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		border-radius: 3px;
		padding: 1px 4px;
		font-family: monospace;
		font-size: 0.9em;
		box-shadow: 1px 1px 1px rgba(0, 0, 0, 0.1);
	}

	/* Style the specific row in both Auto and dedicated views */
	.extended-hours-container .span-row,
	.span-container .extended-hours-row {
		align-items: center; /* Keep vertical alignment */
		justify-content: flex-start; /* Align label and button group to the start */
		gap: 15px; /* Space between label and button group */
	}

	/* Style for the hint text and kbd */
	.label .hint {
		font-size: 0.8em; /* Smaller font size */
		color: var(--text-secondary); /* Lighter color */
		font-weight: 400; /* Normal weight */
		margin-left: 4px; /* Small space from label */
	}

	.label .hint kbd {
		background-color: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		border-radius: 3px;
		padding: 1px 4px;
		font-family: monospace;
		font-size: 0.9em;
		box-shadow: 1px 1px 1px rgba(0, 0, 0, 0.1);
	}

	/* Override layout for extended hours rows */
	.span-row.extended-hours-row {
		justify-content: flex-start !important;
		gap: 8px;
	}

	/* Ensure buttons in extended-hours rows are inline and spaced */
	.span-row.extended-hours-row .hours-buttons {
		display: flex;
		gap: 8px;
	}


</style>
