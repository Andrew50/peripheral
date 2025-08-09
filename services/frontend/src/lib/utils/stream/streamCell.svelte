<!-- streamCell.svelte -->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable } from 'svelte/store';
	import { addStream } from '$lib/utils/stream/interface';
	import type { TradeData, Instance, CloseData } from '$lib/utils/types/types';

	export let instance: Instance;
	export let type: 'price' | 'change' | 'change %' | 'change % extended' | 'market cap' = 'change';
	export let disableFlash: boolean = false;

	let releaseSlow: Function = () => {};
	let releaseClose: Function = () => {};
	let currentSecurityId: number | null = null;

	interface ChangeStore {
		price?: number;
		prevClose?: number;
		change: string;
		shares?: number;
	}
	let changeStore = writable<ChangeStore>({ change: '' });

	let lastPrice: number | undefined;
	let pulseClass = ''; // '' | 'flash-up' | 'flash-down'
	let unchanged = ''; // left part that didn't move
	let changed = ''; // right part that changed
	let lastPriceStr: string | undefined;
	function updateSlices(newPrice: number) {
		const next = newPrice.toFixed(2);
		const prev = lastPriceStr ?? '';

		// find the first differing character
		let idx = 0;
		while (idx < next.length && next[idx] === prev[idx]) idx++;

		unchanged = next.slice(0, idx);
		changed = next.slice(idx);
		lastPriceStr = next;
	}

	function firePulse(dir: 1 | -1) {
		pulseClass = '';
		requestAnimationFrame(() => {
			pulseClass = dir === 1 ? 'flash-up' : 'flash-down';
		});
	}

	// Reactive flags for class logic
	$: hasPriceAndPrevClose = $changeStore.price != null && $changeStore.prevClose != null;
	$: diff = ($changeStore.price ?? 0) - ($changeStore.prevClose ?? 0);
	// $: priceIncreasedOrSame = hasPriceAndPrevClose && ($changeStore.price! - $changeStore.prevClose! >= 0); // Not strictly needed if using !priceDecreased
	$: isPlaceholder = $changeStore.change === '';

	$: isRedForChange = type === 'change' && diff < 0;
	$: isWhiteForChange = type === 'change' && diff === null;
	$: isGreenForChange = type === 'change' && diff > 0;

	$: isRedForChangePercent =
		(type === 'change %' || type === 'change % extended') && $changeStore.change.includes('-');
	$: isGreenForChangePercent =
		(type === 'change %' || type === 'change % extended') &&
		!$changeStore.change.includes('-') &&
		!isPlaceholder &&
		$changeStore.change !== '';

	// Static color logic for when flash is disabled (exclude market cap)
	$: isStaticGreen = disableFlash && hasPriceAndPrevClose && diff > 0 && type !== 'market cap';
	$: isStaticRed = disableFlash && hasPriceAndPrevClose && diff < 0 && type !== 'market cap';

	function setupStreams() {
		// Only setup streams if security ID changed
		if (currentSecurityId === instance.securityId) {
			return;
		}

		currentSecurityId = instance.securityId ? Number(instance.securityId) : null;

		// Clean up existing streams
		releaseClose();
		releaseSlow();

		// Reset the store with shares if market cap type
		if (type === 'market cap' && instance.totalShares) {
			changeStore.set({ change: '', shares: instance.totalShares });
		} else {
			changeStore.set({ change: '' });
		}

		// Decide which streams to use based on type
		const slowStreamName = type === 'change % extended' ? 'slow-extended' : 'slow-regular';
		const closeStreamName = type === 'change % extended' ? 'close-extended' : 'close-regular';

		// Set up new streams
		releaseClose = addStream<CloseData>(instance, closeStreamName, (v: CloseData) => {
			changeStore.update((s: ChangeStore) => {
				const prevClose = v.price;
				const shouldUpdatePrice = v.shouldUpdatePrice;
				// Update the instance object with the prevClose value
				if (type === 'change %') {
					(instance as any)['prevClose'] = prevClose;
				} else if (type === 'change % extended') {
					(instance as any)['prevCloseExtended'] = prevClose;
				} else {
					(instance as any)['prevClose'] = prevClose;
				}

				return {
					...s,
					prevClose,
					shouldUpdatePrice,
					change: s.price && prevClose ? getChange(s.price, prevClose) : ''
				};
			});
		});

		releaseSlow = addStream<TradeData>(instance, slowStreamName, (v: TradeData) => {
			if (v && v.price) {
				changeStore.update((s: ChangeStore) => {
					if (v.size < 100) {
						return s;
					}
					const price = v.price;
					const prevClose = s.prevClose;

					// Skip price updates based on shouldUpdatePrice flag
					const shouldSkipPriceUpdate = !v.shouldUpdatePrice;

					// Only update instance price if not skipping price updates
					if (!shouldSkipPriceUpdate) {
						// Update the instance with the price
						(instance as any)['price'] = price;

						// Update related fields on the instance based on type
						if (type === 'change') {
							// Calculate raw change and store it
							if (price && prevClose) {
								(instance as any)['change'] = price - prevClose;
								s.change = (price - prevClose).toFixed(2);
							}
						} else if (type === 'change %') {
							// Calculate percentage change and store it
							if (price && prevClose) {
								(instance as any)['change%'] = (price / prevClose - 1) * 100;
							}
						} else if (type === 'change % extended') {
							// Calculate extended percentage change
							if (price && prevClose) {
								(instance as any)['change%extended'] = (price / prevClose - 1) * 100;
							}
						} else if (type === 'market cap' && instance.totalShares) {
							// Calculate market cap
							(instance as any)['marketCap'] = price * instance.totalShares;
						}

						if (type === 'market cap') {
							return {
								...s,
								price,
								prevClose
							};
						}
						// skip if shouldUpdatePrice is false, but already skipped in polygonSocket.go to not send to slow streams
						if (type === 'price' && v?.price) {
							const dir =
								lastPrice === undefined
									? 0
									: v.price > lastPrice
										? 1
										: v.price < lastPrice
											? -1
											: 0;

							updateSlices(v.price);
							lastPrice = v.price;

							if (dir && !disableFlash) firePulse(dir);
						}
						return {
							...s,
							price,
							prevClose,
							change: price && prevClose ? getChange(price, prevClose) : ''
						};
					} else {
						// When skipping price updates, only return current state without price changes
						return s;
					}
				});
			}
		});
	}

	// Watch for instance changes
	$: if (instance?.securityId) {
		setupStreams();
	}

	onDestroy(() => {
		releaseClose();
		releaseSlow();
	});

	function getChange(price: number, prevClose: number): string {
		// Removing frequent console logs for performance
		if (!price || !prevClose) return '';
		return ((price / prevClose - 1) * 100).toFixed(2) + '%';
	}

	function formatMarketCap(price?: number, shares?: number): string {
		if (!price || !shares) return 'N/A';
		const marketCap = price * shares;
		if (marketCap >= 1e12) {
			return `$${(marketCap / 1e12).toFixed(2)}T`;
		} else if (marketCap >= 1e9) {
			return `$${(marketCap / 1e9).toFixed(2)}B`;
		} else if (marketCap >= 1e6) {
			return `$${(marketCap / 1e6).toFixed(2)}M`;
		} else {
			return `$${marketCap.toFixed(2)}`;
		}
	}
</script>

<div
	class:red={isRedForChange || isRedForChangePercent || isStaticRed}
	class:white={isWhiteForChange}
	class:green={isGreenForChange || isGreenForChangePercent || isStaticGreen}
	class:price-type={type === 'price'}
	class:disable-flash-mode={disableFlash}
	class="price-cell {pulseClass}"
>
	{#if type === 'price'}
		{#if changed === ''}
			<!-- first print, nothing to colour -->
			{unchanged}
		{:else}
			{unchanged}<span
				class="diff"
				class:up={pulseClass === 'flash-up'}
				class:down={pulseClass === 'flash-down'}
			>
				{changed}
			</span>
		{/if}
	{:else if type === 'change'}
		{#if $changeStore.price != null && $changeStore.prevClose != null}
			{@const changeValue = ($changeStore.price - $changeStore.prevClose).toFixed(2)}
			{changeValue >= 0 ? '+' + changeValue : changeValue}
		{:else}
			{''}
		{/if}
	{:else if type === 'change %' || type === 'change % extended'}
		{#if $changeStore.change && !$changeStore.change.includes('-') && $changeStore.change !== ''}
			{'+' + $changeStore.change}
		{:else}
			{$changeStore.change}
		{/if}
	{:else if type === 'market cap'}
		{formatMarketCap($changeStore.price, $changeStore.shares)}
	{:else}
		{''}
	{/if}
</div>

<style>
	@keyframes flash-green {
		0% {
			color: var(--positive, rgb(72 225 72));
		}

		/* Using ease-out, so 100% to currentColor should provide a smooth transition */
		100% {
			color: currentcolor;
		}
	}

	@keyframes flash-red {
		0% {
			color: var(--negative, rgb(225 72 72));
		}

		100% {
			color: currentcolor;
		}
	}

	.flash-up {
		animation: flash-green 0.5s ease-out forwards;
	}

	.flash-down {
		animation: flash-red 0.5s ease-out forwards;
	}

	/* Sticky colour for the changed digits */
	.diff.up {
		color: var(--positive, rgb(72 225 72));
	}

	.diff.down {
		color: var(--negative, rgb(225 72 72));
	}

	/* Styles for change and change % consistency */
	.red {
		color: var(--negative, rgb(225 72 72));
	}

	.green {
		color: var(--positive, rgb(72 225 72));
	}

	.white {
		color: var(--neutral-placeholder-text, #888); /* Neutral color for placeholders */
	}

	/* basic cell padding so the flash fills the whole box */
	.price-cell {
		padding: 0 4px;
	}

	/* Override flash animations and colors specifically for price type when flash is disabled */
	@keyframes flash-white-price {
		0% {
			color: #fff;
		}

		100% {
			color: #fff;
		}
	}

	/* Only apply white overrides when flash is NOT disabled */
	.price-type:not(.disable-flash-mode).flash-up {
		animation: flash-white-price 0.5s ease-out forwards;
	}

	.price-type:not(.disable-flash-mode).flash-down {
		animation: flash-white-price 0.5s ease-out forwards;
	}

	.price-type:not(.disable-flash-mode) .diff.up {
		color: #fff;
	}

	.price-type:not(.disable-flash-mode) .diff.down {
		color: #fff;
	}
</style>
