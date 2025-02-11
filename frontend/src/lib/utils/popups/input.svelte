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
	let inputQuery: Writable<InputQuery> = writable({ ...inactiveInputQuery });

	export async function queryInstanceInput(
		requiredKeys: InstanceAttributes[] | 'any',
		optionalKeys: InstanceAttributes[] | 'any',
		instance: Instance = {}
	): Promise<Instance> {
		let possibleKeys: InstanceAttributes[];
		if (optionalKeys === 'any') {
			possibleKeys = [...allKeys];
		} else {
			possibleKeys = Array.from(
				new Set([...requiredKeys, ...optionalKeys])
			) as InstanceAttributes[];
		}
		await tick();
		if (get(inputQuery).status === 'inactive') {
			// initialize with the passed instance info
			inputQuery.update((v: InputQuery) => ({
				...v,
				requiredKeys,
				possibleKeys,
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
	//let secQueryActive = false;

	interface ValidateResponse {
		inputValid: boolean;
		securities: Security[];
	}

	async function validateInput(inputString: string, inputType: string): Promise<ValidateResponse> {
		if (inputType === 'ticker') {
			const securities = await privateRequest<Security[]>('getSecuritiesFromTicker', {
				ticker: inputString
			});
			console.log(securities);

			if (Array.isArray(securities) && securities.length > 0) {
				// Fetch details for each security
				const securitiesWithDetails = await Promise.all(
					securities.map(async (security) => {
						try {
							const details = await privateRequest<Security>('getTickerDetails', {
								securityId: security.securityId,
								ticker: security.ticker,
								timestamp: security.timestamp
							}).catch((v) => {});
							return {
								...security,
								...details
							};
						} catch {
							//console.error('Error fetching ticker details:', error);
							return security;
						}
					})
				);

				return {
					inputValid: securitiesWithDetails.some((v) => v.ticker === inputString),
					securities: securitiesWithDetails
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
				if (iQ.possibleKeys.includes('ticker') && /^[A-Z]$/.test(iQ.inputString)) {
					iQ.inputType = 'ticker';
				} else if (
					iQ.possibleKeys.includes('timeframe') &&
					/^\d+(\.\d+)?$/.test(iQ.inputString) &&
					/^\d{1,3}$/.test(iQ.inputString)
				) {
					iQ.inputType = 'timeframe';
					iQ.securities = [];
				} else if (iQ.possibleKeys.includes('price') && /^\d+(\.\d+)?$/.test(iQ.inputString)) {
					iQ.inputType = 'price';
					iQ.securities = [];
				} else if (
					iQ.possibleKeys.includes('timeframe') &&
					/^\d{1,2}(?:[hdwmqs])?$/.test(iQ.inputString)
				) {
					iQ.inputType = 'timeframe';
					iQ.securities = [];
				} else if (iQ.possibleKeys.includes('timestamp') && /^\d{3}?.*$/.test(iQ.inputString)) {
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

			inputQuery.update((v: InputQuery) => ({
				...v,
				inputString: iQ.inputString,
				inputType: iQ.inputType
			}));

			// Validate asynchronously, and then update the store.
			validateInput(iQ.inputString, iQ.inputType).then((validationResp: ValidateResponse) => {
				console.log(validationResp);
				inputQuery.update((v: InputQuery) => ({
					...v,
					//inputString: iQ.inputString,
					//inputType: iQ.inputType,
					...validationResp
				}));
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
	function capitalize(str, lower = false) {
		return (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, (match) =>
			match.toUpperCase()
		);
	}
</script>

{#if $inputQuery.status === 'active' || $inputQuery.status === 'initializing'}
	<div class="popup-container" id="input-window" tabindex="-1">
		<div class="header">
			<div class="title">{capitalize($inputQuery.inputType)} Input</div>
			<button class="close-button" on:click={closeWindow}>×</button>
		</div>

		<div class="search-bar">
			<input type="text" placeholder="Search symbol" value={$inputQuery.inputString} readonly />
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
									{key}
								</span>
								<span>
									{displayValue($inputQuery, key)}
								</span>
							</div>
						{/each}
					</div>
				{:else if $inputQuery.inputType === 'ticker'}
					<div class="table-container">
						{#if Array.isArray($inputQuery.securities) && $inputQuery.securities.length > 0}
							<table>
								<!--<thead>
                        <tr>

                            <th>Ticker</th>const capitalize = (str, lower = false) =>
  (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, match => match.toUpperCase());
;
                            <th>Delist Date</th>
                        </tr>const capitalize = (str, lower = false) =>
  (lower ? str.toLowerCase() : str).replace(/(?:^|\s|["'([{])+\S/g, match => match.toUpperCase());
;
                    </thead>-->
								<tbody>
									{#each $inputQuery.securities as sec, i}
										<tr
											on:click={() => {
												inputQuery.set(enterInput(get(inputQuery), i));
											}}
										>
											<td>{sec.ticker}</td>
											<td>{sec.maxDate === null ? 'Current' : sec.maxDate}</td>
											<td>
												<div
													style="background-color: transparent; width: 100px; height: 30px; display: flex; align-items: center; justify-content: center;"
												>
													{#if sec.logo}
														<img
															src={`data:image/svg+xml;base64,${sec.logo}`}
															alt="Security Image"
															style="max-width: 100%; max-height: 100%; object-fit: contain;"
														/>
													{/if}
												</div>
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						{/if}
					</div>
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
		background: var(--c2);
		border: 1px solid var(--c4);
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

		border-bottom: 1px solid var(--c4);
		height: 40px;
	}

	.title {
		font-size: 16px;
		font-weight: 500;
		color: #fff;
	}

	.close-button {
		background: none;
		border: none;
		color: #666;
		font-size: 20px;
		cursor: pointer;
		padding: 0 5px;
	}

	.close-button:hover {
		color: #fff;
	}

	.search-bar {
		padding: 8px 12px;
		display: flex;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid var(--c4);
		height: 48px;
	}

	.search-bar input {
		flex: 1;
		background: #2d2d2d;
		border: 1px solid #444;
		padding: 6px 10px;
		color: #fff;
		border-radius: 4px;
	}

	.search-icon {
		color: #666;
	}

	.filters {
		padding: 4px 8px;
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
		border-bottom: 1px solid var(--c4);
		height: auto;
		max-height: 80px;
		overflow-y: auto;
	}

	.filter-bubble {
		background: transparent;
		border: 1px solid #444;
		color: #888;
		padding: 2px 8px;
		border-radius: 8px;
		font-size: 10px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.filter-bubble.active {
		background: #2962ff;
		border-color: #2962ff;
		color: white;
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
		border-bottom: 1px solid var(--c4);
		height: 40px;
	}

	.security-item:hover {
		background: #2d2d2d;
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
		color: var(--f1);
		min-width: 60px;
		font-size: 0.9em;
	}

	.name {
		color: var(--f2);
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
		color: var(--f2);
		font-size: 0.8em;
		margin-left: auto;
	}

	.sector {
		color: var(--f2);
		font-size: 0.8em;
		margin-right: 8px;
	}

	.exchange {
		color: var(--f2);
		font-size: 0.8em;
		min-width: 50px;
	}

	.date {
		color: var(--f2);
		font-size: 0.8em;
		min-width: 70px;
	}
</style>
