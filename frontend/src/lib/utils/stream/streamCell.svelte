<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { writable } from 'svelte/store';
	import { addStream } from '$lib/utils/stream/interface';
	import type { TradeData, Instance, CloseData } from '$lib/core/types';

	export let instance: Instance;
	export let type: 'price' | 'change' | 'change %' | 'change % extended' = 'change';

	let releaseSlow: Function = () => {};
	let releaseClose: Function = () => {};
	let currentSecurityId: number | null = null;

	interface ChangeStore {
		price?: number;
		prevClose?: number;
		change: string;
	}
	let changeStore = writable<ChangeStore>({ change: '--' });

	function setupStreams() {
		// Only setup streams if security ID changed
		if (currentSecurityId === instance.securityId) {
			return;
		}

		currentSecurityId = instance.securityId ?? null;

		// Clean up existing streams
		releaseClose();
		releaseSlow();

		// Reset the store
		changeStore.set({ change: '--' });

		// Decide which streams to use based on type
		let slowStreamName = type === 'change % extended' ? 'slow-extended' : 'slow-regular';
		let closeStreamName = type === 'change % extended' ? 'close-extended' : 'close-regular';

		// Set up new streams
		releaseClose = addStream<CloseData>(instance, closeStreamName, (v: CloseData) => {
			changeStore.update((s: ChangeStore) => {
				const prevClose = v.price;
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
					console.log('prevClose', s.prevClose);
					return {
						...s,
						price,
						change: price && s.prevClose ? getChange(price, s.prevClose) : '--'
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
		console.log('price', price);
		console.log('prevClose', prevClose);
		if (!price || !prevClose) return '--';
		return ((price / prevClose - 1) * 100).toFixed(2) + '%';
	}
</script>

<div
	class={type === 'change'
		? ($changeStore.price != null && $changeStore.prevClose != null && $changeStore.price - $changeStore.prevClose < 0)
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
	{:else}
		{'--'}
	{/if}
</div>
