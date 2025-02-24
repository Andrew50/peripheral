<!-- instance.svelte -->
<script lang="ts" context="module">
	import '$lib/core/global.css';
	import { privateRequest } from '$lib/core/backend';
	import { get, writable } from 'svelte/store';
	import { parse } from 'date-fns';
	import { tick } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';
	const allKeys = ['ticker', 'timestamp', 'timeframe', 'extendedHours', 'price'] as const;
	let currentSecurityResultRequest = 0;

	type InstanceAttributes = (typeof allKeys)[number];
	let filterOptions = [];
	let loadedSecurityResultRequest = -1;
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
		console.log(requiredKeys);
		if (get(inputQuery).status !== 'inactive') {
			if (activePromiseReject) {
				activePromiseReject(new Error('User cancelled input'));
				activePromiseReject = null;
			}
			inputQuery.update((q) => ({ ...inactiveInputQuery }));
			// Optionally wait a tick for the UI to update.
			await tick();
		}

		// Determine possible keys.
		let possibleKeys: InstanceAttributes[];
		if (optionalKeys === 'any') {
			possibleKeys = [...allKeys];
		} else {
			possibleKeys = Array.from(
				new Set([...requiredKeys, ...optionalKeys])
			) as InstanceAttributes[];
		}
		await tick();
		// Initialize the query with passed instance info.
		inputQuery.update((v: InputQuery) => ({
			...v,
			requiredKeys,
			possibleKeys,
			instance,
			status: 'initializing'
		}));

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
	import { browser } from '$app/environment';
	import { onDestroy, onMount } from 'svelte';
	import { ESTStringToUTCTimestamp, UTCTimestampToESTString } from '$lib/core/timestamp';
	let prevFocusedElement: HTMLElement | null = null;
	// flag to indicate that an async validation (ticker lookup) is in progress
	//let secQueryActive = false;

	let isLoadingSecurities = false;

	let manualInputType: string = 'auto';

	// Add this reactive statement
	$: if (manualInputType !== 'auto' && $inputQuery.status === 'active') {
		inputQuery.update((v) => ({
			...v,
			inputType: manualInputType
		}));
	}

	interface ValidateResponse {
		inputValid: boolean;
		securities: Instance[];
	}

	async function validateInput(inputString: string, inputType: string): Promise<ValidateResponse> {
		if (inputType === 'ticker') {
			isLoadingSecurities = true;
			try {
				const securities = await privateRequest<Instance[]>('getSecuritiesFromTicker', {
					ticker: inputString
				});
				if (Array.isArray(securities) && securities.length > 0) {
					return {
						//inputValid: securities.some((v) => v.ticker === inputString),
						inputValid: true,
						securities: securities
					};
				}
				return { inputValid: false, securities: [] };
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
		} else if (inputType === 'price') {
			const price = parseFloat(inputString);
			return { inputValid: !isNaN(price) && price > 0, securities: [] };
		}
		return { inputValid: false, securities: [] };
	}

	async function waitForSecurityResult(): Promise<void> {
		return new Promise((resolve) => {
			const check = () => {
				if (loadedSecurityResultRequest === currentSecurityResultRequest) {
					resolve();
				} else {
					// Check again after 50ms (or adjust as needed)
					setTimeout(check, 50);
				}
			};
			check();
		});
	}

	async function enterInput(iQ: InputQuery, tickerIndex: number = 0): Promise<InputQuery> {
		console.log(iQ);
		if (iQ.inputType === 'ticker') {
			const ts = iQ.instance.timestamp;
			await waitForSecurityResult();
			iQ = $inputQuery;
			console.log(iQ);
			if (Array.isArray(iQ.securities) && iQ.securities.length > 0) {
				iQ.instance = { ...iQ.instance, ...iQ.securities[tickerIndex] };
				console.log(iQ.instance);
				iQ.instance.timestamp = ts;
			}
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
			console.log(iQ.requiredKeys);
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
		// Reset manualInputType to auto after input is entered
		manualInputType = 'auto';
		return iQ;
	}

	/*async function fetchSecurityDetails(securities: Instance[]): Promise<Instance[]> {
		console.log(securities);
		return Promise.all(
			securities.map(async (security) => {
				const details = await privateRequest<Instance>('getTickerDetails', {
					securityId: security.securityId,
					ticker: security.ticker,
					timestamp: security.timestamp
				}).catch((v) => {
					console.warn(`get Details failed for ${security} ${v}`);
				});
				return {
					...security,
					...details
				};
			})
		);
	}*/

	// Mark handleKeyDown as async so we can await the validate call if needed.
	async function handleKeyDown(event: KeyboardEvent): Promise<void> {
		// Only process keys when the input UI is active
		const currentState = get(inputQuery);
		if (currentState.status !== 'active') return;
		event.stopPropagation();
		let iQ = { ...currentState };
		if (event.key === 'Escape') {
			inputQuery.update((q) => ({ ...q, status: 'cancelled' }));
		} else if (event.key === 'Enter') {
			event.preventDefault();
			if (iQ.inputValid) {
				const updatedQuery = await enterInput(iQ, 0);
				inputQuery.set(updatedQuery);
			}
		} else if (event.key === 'Tab') {
			event.preventDefault();
			inputQuery.update((q) => ({
				...q,
				instance: { ...q.instance, extendedHours: !q.instance.extendedHours }
			}));
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

			// Only auto-classify if manualInputType is set to 'auto'
			if (manualInputType === 'auto') {
				// classify the input string into a type
				if (iQ.inputString !== '') {
					if (iQ.possibleKeys.includes('ticker') && /^[A-Z]$/.test(iQ.inputString)) {
						iQ.inputType = 'ticker';
					} else if (
						iQ.possibleKeys.includes('price') &&
						/^(?:\d*\.\d+|\d{3,})$/.test(iQ.inputString)
					) {
						iQ.inputType = 'price';
						iQ.securities = [];
					} else if (
						iQ.possibleKeys.includes('timeframe') &&
						/^\d{1,2}[hdwmqs]?$/i.test(iQ.inputString)
					) {
						iQ.inputType = 'timeframe';
						iQ.securities = [];
					} else if (iQ.possibleKeys.includes('timestamp') && /^[\d-]+$/.test(iQ.inputString)) {
						iQ.inputType = 'timestamp';
						iQ.securities = [];
					} else if (iQ.possibleKeys.includes('ticker')) {
						iQ.inputType = 'ticker';
					} else {
						iQ.inputType = '';
					}
				} else {
					iQ.inputType = '';
				}
			} else {
				// Use the manually selected input type
				iQ.inputType = manualInputType;
			}

			inputQuery.update((v: InputQuery) => ({
				...v,
				inputString: iQ.inputString,
				inputType: iQ.inputType
			}));
			currentSecurityResultRequest++;
			const thisSecurityResultRequest = currentSecurityResultRequest;

			// Validate asynchronously, and then update the store.
			validateInput(iQ.inputString, iQ.inputType).then((validationResp: ValidateResponse) => {
				if (thisSecurityResultRequest === currentSecurityResultRequest) {
					inputQuery.update((v: InputQuery) => ({
						...v,
						...validationResp
					}));
					loadedSecurityResultRequest = thisSecurityResultRequest;

					/*fetchSecurityDetails([...validationResp.securities]).then(
						(securitiesWithDetails: Instance[]) => {
							if (thisSecurityResultRequest === currentSecurityResultRequest) {
								inputQuery.update((v: InputQuery) => {
									// Merge the newly fetched details into the instance if it matches
									/*if (v.instance && 'ticker' in v.instance && v.instance.ticker) {
										const matchedSec = securitiesWithDetails.find(
											(sec) => sec.ticker === v.instance.ticker
										);
										if (matchedSec) {
											v.instance = { ...v.instance, ...matchedSec };
										}
									}*/
					/*
									return {
										...v,
										securities: securitiesWithDetails
									};
								});
								console.log($inputQuery);
							}
						}
					);*/
				}
			});
		}
	}

	// onTouch handler (if needed) now removes the UI by updating via update() too.
	function onTouch(event: TouchEvent) {
		inputQuery.update((v: InputQuery) => ({ ...v, status: 'cancelled' }));
	}

	// Instead of repeatedly adding/removing listeners in the store subscription,
	// we add the keydown listener once on mount and remove it on destroy.
	let unsubscribe: () => void;
	const keydownHandler = (event: KeyboardEvent) => {
		handleKeyDown(event);
	};
	onMount(() => {
		prevFocusedElement = document.activeElement as HTMLElement;
		document.addEventListener('keydown', keydownHandler);
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

		type SecurityClassifications = {
			sectors: string[];
			industries: string[];
		};
		privateRequest<SecurityClassifications>('getSecurityClassifications', {}, true).then(
			(classifications: SecurityClassifications) => {
				sectors = classifications.sectors;
				industries = classifications.industries;
			}
		);
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
		} else if (key in q.instance) {
			if (key === 'timestamp') {
				return UTCTimestampToESTString(q.instance.timestamp ?? 0);
			} else if (key === 'extendedHours') {
				return q.instance.extendedHours ? 'True' : 'False';
			} else if (key === 'price') {
				return '$' + String(q.instance.price);
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
	<div class="popup-container" id="input-window" tabindex="-1">
		<div class="header">
			<div class="title">{capitalize($inputQuery.inputType)} Input</div>
			<div class="field-select">
				<span class="label">Field:</span>
				<select class="default-select" bind:value={manualInputType}>
					<option value="auto">Auto</option>
					{#each $inputQuery.possibleKeys as key}
						<option value={key}>{capitalize(key)}</option>
					{/each}
				</select>
			</div>
			<button class="utility-button" on:click={closeWindow}>×</button>
		</div>

		<div class="search-bar">
			<input type="text" placeholder="Enter Value" value={$inputQuery.inputString} readonly />
		</div>
		<div class="content-container">
			{#if $inputQuery.instance && Object.keys($inputQuery.instance).length > 0}
				{#if $inputQuery.inputType === ''}
					<div class="span-container">
						{#each $inputQuery.possibleKeys as key}
							<div class="span-row">
								<span
									class={$inputQuery.requiredKeys !== 'any' &&
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
							</div>
						{/each}
					</div>
				{:else if $inputQuery.inputType === 'ticker'}
					<div class="table-container">
						{#if isLoadingSecurities}
							<div class="loading-container">
								<div class="loading-spinner"></div>
								<span class="label">Loading securities...</span>
							</div>
						{:else if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
							<table>
								<!--<thead>
                        <tr class="defalt-tr">

                            <th class="defalt-th">Ticker</th>const capitalize = (str, lower = false) =>
  (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, match => match.toUpperCase());
;
                            <th class="defalt-th">Delist Date</th>
                        </tr>const capitalize = (str, lower = false) =>
  (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, match => match.toUpperCase());
;
                    </thead>-->
								<tbody>
									{#each $inputQuery.securities as sec, i}
										<tr
											on:click={async () => {
												const updatedQuery = await enterInput($inputQuery, i);
												inputQuery.set(updatedQuery);
											}}
										>
											<td class="defalt-td">
												<div
													style="background-color: transparent; width: 100px; height: 30px; display: flex; align-items: center; justify-content: center;"
												>
													{#if sec.icon}
														<img
															src={`data:image/jpeg;base64,${sec.icon}`}
															alt="Security Image"
															style="max-width: 100%; max-height: 100%; object-fit: contain;"
														/>
													{/if}
													<!--{#if sec.logo}
														<img
															src={`data:image/svg+xml;base64,${sec.logo}`}
															alt="Security Image"
															style="max-width: 100%; max-height: 100%; object-fit: contain;"
														/>
													{/if}-->
												</div>
											</td>
											<td class="defalt-td">{sec.ticker}</td>
											<td class="defalt-td">{sec.name}</td>
											<td class="defalt-td">{UTCTimestampToESTString(sec.timestamp)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						{/if}
					</div>
				{:else if $inputQuery.inputType === 'timestamp'}
					<div class="span-container">
						<div class="span-row">
							<span class="label">Timestamp</span>
							<input
								type="datetime-local"
								on:change={(e) => {
									const date = new Date(e.target?.value ?? '');
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
					<div class="span-container">
						<div class="span-row">
							<span class="label">Extended Hours</span>
							<span class="value">{$inputQuery.instance.extendedHours ? 'True' : 'False'}</span>
						</div>
					</div>
					0
				{/if}
			{/if}
		</div>
		<!-- TODO!<div class="filters">
            {#each [...filterOptions.industries, ...filterOptions.sectors] as item}
				<button
					class="filter-bubble"
					class:active={selectedFilter === item}
					on:click={() => (selectedFilter = item)}
				>
                {item}
				</button>
			{/each}
		</div>-->
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
		width: 700px;
		height: 600px;
		background: var(--ui-bg-primary);
		border: 1px solid var(--ui-border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
	}
	.span-container {
		display: felx;
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
	}

	.search-bar input {
		flex: 1;
		background: var(--ui-bg-element);
		border: 1px solid var(--ui-border);
		padding: 6px 10px;
		color: var(--text-primary);
		border-radius: 4px;
	}

	.search-icon {
		color: var(--text-secondary);
	}

	.filters {
		padding: 4px 8px;
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
		border-bottom: 1px solid var(--ui-border);
		height: auto;
		max-height: 80px;
		overflow-y: auto;
	}

	.filter-bubble {
		background: transparent;
		border: 1px solid var(--ui-border);
		color: var(--text-secondary);
		padding: 2px 8px;
		border-radius: 8px;
		font-size: 10px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.filter-bubble.active {
		background: var(--ui-accent);
		border-color: var(--ui-accent);
		color: var(--text-primary);
	}

	.results {
		padding: 0;
		flex: 1;
		overflow-y: auto;
	}

	.securities-list {
		display: flex;
		flex-direction: column;
	}

	.security-item {
		display: flex;
		align-items: center;
		padding: 8px 12px;
		cursor: pointer;
		border-bottom: 1px solid var(--ui-border);
		height: 40px;
	}

	.security-item:hover {
		background: var(--ui-bg-hover);
	}

	.security-icon {
		margin-right: 12px;
		width: 24px;
		height: 24px;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.security-icon img {
		width: 28px;
		height: 28px;
		object-fit: contain;
	}

	.security-info {
		flex: 1;
		display: flex;
		flex-direction: row;
		align-items: center;
		gap: 12px;
	}

	.security-main {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 1;
	}

	.ticker {
		font-weight: 600;
		color: var(--text-primary);
		min-width: 60px;
		font-size: 0.9em;
	}

	.name {
		color: var(--text-secondary);
		font-size: 0.85em;
		flex: 1;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.security-details {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 1;
		color: var(--text-secondary);
		font-size: 0.8em;
		margin-left: auto;
	}

	.sector {
		color: var(--text-secondary);
		font-size: 0.8em;
		margin-right: 8px;
	}

	.exchange {
		color: var(--text-secondary);
		font-size: 0.8em;
		min-width: 50px;
	}

	.date {
		color: var(--text-secondary);
		align-items: center;
		justify-content: center;
		padding: 20px;
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

	@keyframes spin {
		0% {
			transform: rotate(0deg);
		}
		100% {
			transform: rotate(360deg);
		}
	}

	.field-select {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-left: auto;
	}

	.field-select span {
		color: var(--text-secondary);
		font-size: 14px;
	}
</style>
