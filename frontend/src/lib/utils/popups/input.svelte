<!-- instance.svelte -->
<script lang="ts" context="module">
	import '$lib/core/global.css';
	import { privateRequest } from '$lib/core/backend';
	import { get, writable } from 'svelte/store';
	import { parse } from 'date-fns';
	import { tick } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';

	interface Security {
		securityId: number;
		ticker: string;
		maxDate: string | null;
		name: string;
	}
	const possibleDisplayKeys = ['ticker', 'timestamp', 'timeframe', 'extendedHours', 'price'];
	type InstanceAttributes = (typeof possibleDisplayKeys)[number];
	interface InputQuery {
		// 'inactive': no UI shown
		// 'initializing': setting up event handlers
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
		securities?: Security[];
	}

	const inactiveInputQuery: InputQuery = {
		status: 'inactive',
		inputString: '',
		inputValid: true,
		inputType: '',
		requiredKeys: 'any',
		instance: {}
	};
	let inputQuery: Writable<InputQuery> = writable({ ...inactiveInputQuery });

	export async function queryInstanceInput(
		requiredKeys: InstanceAttributes[] | 'any',
		instance: Instance = {}
	): Promise<Instance> {
		await tick();
		if (get(inputQuery).status === 'inactive') {
			// initialize with the passed instance info
			inputQuery.update((v: InputQuery) => ({
				...v,
				requiredKeys,
				instance,
				status: 'initializing'
			}));
			return new Promise<Instance>((resolve, reject) => {
				const unsubscribe = inputQuery.subscribe((iQ: InputQuery) => {
					if (iQ.status === 'cancelled') {
						cleanup();
						reject(new Error('User cancelled input'));
					} else if (iQ.status === 'complete') {
						const re = iQ.instance;
						cleanup();
						resolve(re);
					}
				});
				function cleanup() {
					unsubscribe();
					// trigger shutdown so the onMount subscription resets the state
					inputQuery.update((v: InputQuery) => ({ ...v, status: 'shutdown' }));
				}
			});
		} else {
			return Promise.reject(new Error('Input query already active'));
		}
	}
</script>

<script lang="ts">
	import { browser } from '$app/environment';
	import { onDestroy, onMount } from 'svelte';
	import { ESTStringToUTCTimestamp, UTCTimestampToESTString } from '$lib/core/timestamp';
	let prevFocusedElement: HTMLElement | null = null;
	// flag to indicate that an async validation (ticker lookup) is in progress
	let secQueryActive = false;

	interface ValidateResponse {
		inputValid: boolean;
		securities: Security[];
	}

	async function validateInput(inputString: string, inputType: string): Promise<ValidateResponse> {
		if (inputType === 'ticker') {
			secQueryActive = true;
			const securities = await privateRequest<Security[]>('getSecuritiesFromTicker', {
				ticker: inputString
			});
			secQueryActive = false;
			if (Array.isArray(securities) && securities.length > 0) {
				return {
					inputValid: securities.some((v: Security) => v.ticker === inputString),
					securities: securities
				};
			} else {
				return { inputValid: false, securities: [] };
			}
		} else if (inputType === 'timeframe') {
			const regex = /^\d{1,3}[yqmwhds]?$/i;
			return { inputValid: regex.test(inputString), securities: [] };
		} else if (inputType === 'timestamp') {
			const formats = ['yyyy-MM-dd H:m:ss', 'yyyy-MM-dd H:m', 'yyyy-MM-dd H', 'yyyy-MM-dd'];
			for (const format of formats) {
				try {
					const parsedDate = parse(inputString, format, new Date());
					if (parsedDate != 'Invalid Date') {
						return { inputValid: true, securities: [] };
					}
				} catch {
					/* try next format */
				}
			}
			return { inputValid: false, securities: [] };
		} else if (inputType === 'price') {
			const price = parseFloat(inputString);
			return { inputValid: !isNaN(price) && price > 0, securities: [] };
		}
		return { inputValid: false, securities: [] };
	}

	function enterInput(iQ: InputQuery, tickerIndex: number = 0): InputQuery {
		if (iQ.inputType === 'ticker' && Array.isArray(iQ.securities) && iQ.securities.length > 0) {
			iQ.instance.securityId = iQ.securities[tickerIndex].securityId;
			iQ.instance.ticker = iQ.securities[tickerIndex].ticker;
		} else if (iQ.inputType === 'timeframe') {
			iQ.instance.timeframe = iQ.inputString;
		} else if (iQ.inputType === 'timestamp') {
			iQ.instance.timestamp = ESTStringToUTCTimestamp(iQ.inputString);
		} else if (iQ.inputType === 'price') {
			iQ.instance.price = parseFloat(iQ.inputString);
		}
		// Mark as complete but then check if further input is needed.
		iQ.status = 'complete';
		if (iQ.requiredKeys === 'any') {
			if (Object.keys(iQ.instance).length === 0) {
				iQ.status = 'active';
			}
		} else {
			for (const attribute of iQ.requiredKeys) {
				if (!iQ.instance[attribute]) {
					iQ.status = 'active';
					break;
				}
			}
		}
		iQ.inputString = '';
		iQ.inputType = '';
		iQ.inputValid = true;
		return iQ;
	}

	// Mark handleKeyDown as async so we can await the validate call if needed.
	async function handleKeyDown(event: KeyboardEvent): Promise<void> {
		// Only process keys when the input UI is active
		const currentState = get(inputQuery);
		if (currentState.status !== 'active') return;

		event.stopPropagation();

		// Do not process if a validation (security query) is underway
		if (secQueryActive) return;

		let iQ = { ...currentState };
		if (event.key === 'Escape') {
			iQ.status = 'cancelled';
			inputQuery.set(iQ);
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (iQ.inputValid) {
				iQ = enterInput(iQ, 0);
			}
			inputQuery.set(iQ);
		} else if (event.key === 'Tab') {
			event.preventDefault();
			iQ.instance.extendedHours = !iQ.instance.extendedHours;
			inputQuery.set({ ...iQ });
		} else {
			// Process alphanumeric and a few special characters.
			if (
				/^[a-zA-Z0-9]$/.test(event.key) ||
				/[-:.]/.test(event.key) ||
				(event.key === ' ' && iQ.inputType === 'timestamp')
			) {
				const key = iQ.inputType === 'timeframe' ? event.key : event.key.toUpperCase();
				iQ.inputString += key;
			} else if (event.key === 'Backspace') {
				iQ.inputString = iQ.inputString.slice(0, -1);
			}

			// classify the input string into a type
			if (iQ.inputString !== '') {
				if (/^[A-Z]$/.test(iQ.inputString)) {
					iQ.inputType = 'ticker';
				} else if (/^\d+(\.\d+)?$/.test(iQ.inputString)) {
					if (/^\d{1,3}$/.test(iQ.inputString)) {
						iQ.inputType = 'timeframe';
						iQ.securities = [];
					} else {
						iQ.inputType = 'price';
						iQ.securities = [];
					}
				} else if (/^\d{1,2}(?:[hdwmqs])?$/.test(iQ.inputString)) {
					iQ.inputType = 'timeframe';
					iQ.securities = [];
				} else if (/^\d{3}?.*$/.test(iQ.inputString)) {
					iQ.inputType = 'timestamp';
					iQ.securities = [];
				} else {
					iQ.inputType = 'ticker';
				}
			} else {
				iQ.inputType = '';
			}

			// Validate asynchronously, and then update the store.
			const validateResponse: ValidateResponse = await validateInput(iQ.inputString, iQ.inputType);
			inputQuery.update((v: InputQuery) => ({
				...v,
				inputString: iQ.inputString,
				inputType: iQ.inputType,
				...validateResponse
			}));
		}
	}

	// onTouch handler (if needed) now removes the UI by updating via update() too.
	function onTouch(event: TouchEvent) {
		inputQuery.update((v: InputQuery) => ({ ...v, status: 'cancelled' }));
	}

	// Instead of repeatedly adding/removing listeners in the store subscription,
	// we add the keydown listener once on mount and remove it on destroy.
	let unsubscribe: () => void;
	onMount(() => {
		// Save the original focused element so we can restore focus later.
		prevFocusedElement = document.activeElement as HTMLElement;

		// Keydown listener simply calls our async handler.
		const keydownHandler = (event: KeyboardEvent) => {
			handleKeyDown(event);
		};
		document.addEventListener('keydown', keydownHandler);

		// One subscription to the store handles transitions that affect focus.
		unsubscribe = inputQuery.subscribe((v: InputQuery) => {
			if (browser) {
				if (v.status === 'initializing') {
					// Focus the hidden input (after a tick to allow rendering)
					tick().then(() => {
						document.getElementById('hidden-input')?.focus();
					});
					// Use update() to mark that the UI is now active.
					inputQuery.update((state) => ({ ...state, status: 'active' }));
				} else if (v.status === 'shutdown') {
					// Restore focus and then update to inactive.
					prevFocusedElement?.focus();
					inputQuery.update((state) => ({ ...state, status: 'inactive', inputString: '' }));
				}
			}
		});

		// (If you need a touch listener, add it once here, too.)
		// document.addEventListener('touchstart', onTouch);

		// Cleanup on destroy
	});
	onDestroy(() => {
		try {
			document.removeEventListener('keydown', keydownHandler);
			// document.removeEventListener('touchstart', onTouch);
			unsubscribe();
		} catch (error) {
			console.error('Error removing event listeners:', error);
		}
	});
	function displayValue(q: InputQuery, key: string): string {
		if (key === q.inputType) {
			return q.inputString;
		} else if (q.instance[key] !== undefined) {
			if (key === 'timestamp') {
				return UTCTimestampToESTString(q.instance.timestamp);
			} else if (key === 'extendedHours') {
				return q.instance.extendedHours ? 'True' : 'False';
			} else if (key === 'price') {
				return '$' + String(q.instance.price);
			} else {
				return q.instance[key];
			}
		}
		return '';
	}
</script>

{#if $inputQuery.status === 'active' || $inputQuery.status === 'initializing'}
	<div class="popup-container" id="input-window" tabindex="-1">
		<div class="content-container">
			{#if $inputQuery.instance && Object.keys($inputQuery.instance).length > 0}
				{#each possibleDisplayKeys as key}
					<div class="span-container">
						<span
							class={$inputQuery.requiredKeys.includes(key) && !$inputQuery.instance[key]
								? 'red'
								: ''}
						>
							{key}
						</span>
						<span
							class={key === $inputQuery.inputType ? ($inputQuery.inputValid ? 'blue' : 'red') : ''}
						>
							{displayValue($inputQuery, key)}
						</span>
					</div>
				{/each}
			{/if}
			<div class="table-container">
				{#if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
					<table>
						<thead>
							<tr>
								<th>Ticker</th>
								<th>Delist Date</th>
							</tr>
						</thead>
						<tbody>
							{#each $inputQuery.securities as sec, i}
								<tr
									on:click={() => {
										inputQuery.set(enterInput(get(inputQuery), i));
									}}
								>
									<td>{sec.ticker}</td>
									<td>{sec.maxDate === null ? 'Current' : sec.maxDate}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		</div>
	</div>
{/if}
<input
	autocomplete="off"
	type="text"
	id="hidden-input"
	style="position: absolute; opacity: 0; z-index: -1;"
/>

<style>
	.popup-container {
		width: 400px;
		height: 500px;
	}
</style>
