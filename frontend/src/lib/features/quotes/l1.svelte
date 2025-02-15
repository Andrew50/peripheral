<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { type Writable, writable } from 'svelte/store';
	import type { QuoteData, Instance } from '$lib/core/types';
	import { addStream } from '$lib/utils/stream/interface';
	import { derived } from 'svelte/store';

	export let instance: Writable<Instance>;
	let store: Writable<QuoteData> = writable({});
	let release: Function = () => {};

	let previousBidPrice = 0;
	let previousAskPrice = 0;
	let bidPriceChange = 'no-change'; // Can be 'increase', 'decrease', or 'no-change'
	let askPriceChange = 'no-change'; // Can be 'increase', 'decrease', or 'no-change'
	function updateStore(v: QuoteData) {
		if (v) {
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
		if (!inst.securityId) return;
		release();
		release = addStream(inst, 'quote', updateStore);
	});

	onDestroy(() => {
		release();
	});
</script>

<div class="quote-container">
	<div class="quote-row">
		<!-- Bid section on the left -->
		<div class="bid">
			<div class="price">
				<span class="label">Bid:</span>
				<span class="value {bidPriceChange}">{$store?.bidPrice?.toFixed(2) ?? '--'}</span>
			</div>
			<div class="size">
				<span class="value">x {$store?.bidSize ?? '--'}</span>
			</div>
		</div>

		<!-- Ask section on the right -->
		<div class="ask">
			<div class="price">
				<span class="label">Ask:</span>
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
</style>
