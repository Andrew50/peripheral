<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { type Writable, writable } from 'svelte/store';
	import type { QuoteData, Instance, TradeData } from '$lib/core/types';
	import { addStream } from '$lib/utils/stream/interface';
	import { derived } from 'svelte/store';

	export let instance: Writable<Instance>;
	let store = writable<QuoteData>({
		timestamp: 0,
		bidPrice: 0,
		askPrice: 0,
		bidSize: 0,
		askSize: 0
	});
	let quoteStore: Writable<QuoteData>;
	let currentSecurityId: number | null = null;
	let release: Function = () => {};

	let previousBidPrice = 0;
	let previousAskPrice = 0;
	let bidPriceChange = 'no-change'; // Can be 'increase', 'decrease', or 'no-change'
	let askPriceChange = 'no-change'; // Can be 'increase', 'decrease', or 'no-change'

	function updateStore(v: QuoteData | TradeData | number) {
		if (typeof v === 'object' && 'bidPrice' in v && 'askPrice' in v) {
			// Check bid price change
			if (v.bidPrice !== undefined && v.bidPrice !== previousBidPrice) {
				bidPriceChange = v.bidPrice > previousBidPrice ? 'increase' : 'decrease';
				previousBidPrice = v.bidPrice;
			}
			if (v.askPrice !== undefined && v.askPrice !== previousAskPrice) {
				askPriceChange = v.askPrice > previousAskPrice ? 'increase' : 'decrease';
				previousAskPrice = v.askPrice;
			}
			store.set(v);
		}
	}

	instance.subscribe((inst: Instance) => {
		if (!inst.securityId) {
			return;
		}

		// Check if we already have a stream for this security ID
		if (currentSecurityId === inst.securityId) {
			return;
		}

		currentSecurityId = inst.securityId;
		release();
		release = addStream(inst, 'quote', updateStore);
	});

	onDestroy(() => {
		release();
		currentSecurityId = null;
	});

	// Format for compact display
	$: bidDisplay = `${$store?.bidPrice?.toFixed(2) ?? '--'}×${$store?.bidSize ?? '--'}`;
	$: askDisplay = `${$store?.askPrice?.toFixed(2) ?? '--'}×${$store?.askSize ?? '--'}`;
</script>

<div class="quote-container">
	<div class="quote-row">
		<!-- Bid section on the left -->
		<div class="bid">
			<div class="price">
				<span class="value {bidPriceChange}">{$store?.bidPrice?.toFixed(2) ?? '--'}</span>
			</div>
			<div class="size">
				<span class="value">x {$store?.bidSize ?? '--'}</span>
			</div>
		</div>

		<!-- Ask section on the right -->
		<div class="ask">
			<div class="price">
				<span class="value {askPriceChange}">{$store?.askPrice?.toFixed(2) ?? '--'}</span>
			</div>
			<div class="size">
				<span class="value">x {$store?.askSize ?? '--'}</span>
			</div>
		</div>
	</div>
</div>

<style>
	.quote-container {
		font-family: Arial, sans-serif;
		font-size: 14px;
		color: white;
		background-color: black;
		width: 100%;
		border: 1px solid #333;
		margin: 0 auto;
		border-radius: 5px;
	}

	.quote-row {
		display: flex;
		justify-content: space-between;
		padding: 10px;
		margin-bottom: 2px;
	}

	.bid,
	.ask {
		display: flex;
		flex-direction: row;
		align-items: center;
	}

	.price {
		margin-right: 5px;
		font-weight: bold;
	}

	.size {
		margin-left: 5px;
	}

	.label {
		font-weight: bold;
		color: #ccc;
	}

	.value {
		font-family: monospace;
		color: white;
	}

	/* Styling for bid/ask colors based on price change */
	.increase {
		color: green;
	}

	.decrease {
		color: red;
	}

	.no-change {
		color: white;
	}

	/* Update the bid-ask-row styles */
	.bid-ask-row {
		display: flex;
		gap: 10px;
		margin: 4px 0;
	}

	.bid {
		background-color: rgba(64, 84, 178, 0.6);
		color: #a1b0ff;
		padding: 2px 8px;
		border-radius: 12px;
		font-size: 14px;
		font-family: monospace;
	}

	.ask {
		background-color: rgba(178, 64, 64, 0.6);
		color: #ff9e9e;
		padding: 2px 8px;
		border-radius: 12px;
		font-size: 14px;
		font-family: monospace;
	}
</style>
