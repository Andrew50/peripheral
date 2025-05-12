<!-- streamCell.svelte -->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable } from 'svelte/store';
	import { addStream } from '$lib/utils/stream/interface';
	import type { TradeData, Instance, CloseData } from '$lib/utils/types/types';

	export let instance: Instance;
	export let type: 'price' | 'change' | 'change %' | 'change % extended' | 'market cap' = 'change';

	let releaseSlow: Function = () => {};
	let releaseClose: Function = () => {};
	let currentSecurityId: number | null = null;

	interface ChangeStore {
		price?: number;
		prevClose?: number;
		change: string;
		shares?: number;
	}
	let changeStore = writable<ChangeStore>({ change: '--' });

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
			changeStore.set({ change: '--', shares: instance.totalShares });
		} else {
			changeStore.set({ change: '--' });
		}

		// Decide which streams to use based on type
		const slowStreamName = type === 'change % extended' ? 'slow-extended' : 'slow-regular';
		const closeStreamName = type === 'change % extended' ? 'close-extended' : 'close-regular';

		// Set up new streams
		releaseClose = addStream<CloseData>(instance, closeStreamName, (v: CloseData) => {
			changeStore.update((s: ChangeStore) => {
				const prevClose = v.price;

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
					change: s.price && prevClose ? getChange(s.price, prevClose) : '--'
				};
			});
		});

		releaseSlow = addStream<TradeData>(instance, slowStreamName, (v: TradeData) => {
			if (v && v.price) {
				changeStore.update((s: ChangeStore) => {
					const price = v.price;
					const prevClose = s.prevClose;

					// Update the instance with the price
					(instance as any)['price'] = price;

					// Update related fields on the instance based on type
					if (type === 'change') {
						// Calculate raw change and store it
						if (price && prevClose) {
							(instance as any)['change'] = price - prevClose;
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
					return {
						...s,
						price,
						prevClose,
						change: price && prevClose ? getChange(price, prevClose) : '--'
					};
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
		if (!price || !prevClose) return '--';
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
	class={type === 'change'
		? $changeStore.price != null &&
			$changeStore.prevClose != null &&
			$changeStore.price - $changeStore.prevClose < 0
			? 'red'
			: $changeStore.change === '--'
				? 'white'
				: 'green'
		: type === 'change %' || type === 'change % extended'
			? $changeStore.change.includes('-')
				? 'red'
				: 'green'
			: ''}
>
	{#if type === 'change'}
		{$changeStore.price != null && $changeStore.prevClose != null
			? ($changeStore.price - $changeStore.prevClose).toFixed(2)
			: '--'}
	{:else if type === 'price'}
		{$changeStore.price?.toFixed(2) ?? '--'}
	{:else if type === 'change %' || type === 'change % extended'}
		{$changeStore.change}
	{:else if type === 'market cap'}
		{formatMarketCap($changeStore.price, $changeStore.shares)}
	{:else}
		{'--'}
	{/if}
</div>
