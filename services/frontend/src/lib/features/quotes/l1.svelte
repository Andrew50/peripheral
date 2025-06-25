<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { type Writable, writable } from 'svelte/store';
	import type { QuoteData, Instance, TradeData } from '$lib/utils/types/types';
	import { addStream } from '$lib/utils/stream/interface';
	import '$lib/styles/glass.css';

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

		currentSecurityId =
			typeof inst.securityId === 'string' ? parseInt(inst.securityId, 10) : inst.securityId;
		release();
		release = addStream(inst, 'quote', updateStore);
	});

	onDestroy(() => {
		release();
		currentSecurityId = null;
	});
</script>

<div class="quote-container glass glass--small glass--medium">
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

<style>
	.quote-container {
		font-family:
			'Inter',
			-apple-system,
			BlinkMacSystemFont,
			Segoe UI,
			Roboto,
			Oxygen,
			Ubuntu,
			Cantarell,
			Open Sans,
			Helvetica Neue,
			sans-serif;
		font-size: 14px;
		color: var(--text-primary, white);
		width: 100%;
		margin: 0 auto;
		padding: 10px;
		display: flex;
		justify-content: space-between;
	}

	.bid,
	.ask {
		display: flex;
		flex-direction: row;
		align-items: center;
		font-weight: 500;
		transition: all 0.15s ease;
		border-radius: 6px;
		padding: 4px 10px;
		cursor: default;
	}

	.bid:hover,
	.ask:hover {
		filter: brightness(1.15);
		transform: translateY(-1px);
		box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
	}

	.price {
		margin-right: 5px;
		font-weight: 600;
		letter-spacing: 0.2px;
	}

	.size {
		margin-left: 5px;
		opacity: 0.9;
	}

	.label {
		font-weight: 500;
		color: var(--text-secondary, #ccc);
	}

	.value {
		font-family: var(--font-mono, monospace);
		color: var(--text-primary, white);
	}

	/* Styling for bid/ask colors based on price change */
	.increase {
		color: var(--color-up, #4caf50);
	}

	.decrease {
		color: var(--color-down, #f44336);
	}

	.no-change {
		color: var(--text-primary, white);
	}

	/* Update the bid-ask styles to be more subtle and elegant */
	.bid {
		background-color: rgba(102, 187, 106, 0.15);
		color: var(--color-up, #66bb6a);
	}

	.ask {
		background-color: rgba(239, 83, 80, 0.15);
		color: var(--color-down, #ef5350);
	}
</style>
