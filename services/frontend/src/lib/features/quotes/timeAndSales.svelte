<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import type { Writable } from 'svelte/store';
	import type { TradeData, QuoteData, Instance } from '$lib/utils/types/types';
	import { addStream } from '$lib/utils/stream/interface';
	import '$lib/styles/global.css';
	import { settings } from '$lib/utils/stores/stores';
	import { get } from 'svelte/store';
	import { privateRequest } from '$lib/utils/helpers/backend';

	export let instance: Writable<Instance>;
	let store: Writable<TradeData>;
	let quoteStore: Writable<QuoteData>;
	let releaseTrade: () => void = () => {};
	let releaseQuote: () => void = () => {};
	let unsubscribeTrade = () => {};
	let unsubscribeQuote = () => {};

	interface TaS extends TradeData {
		color: string;
		exchangeName: string;
	}

	type Exchanges = { [exchangeId: number]: string };
	let exchanges: Exchanges = {};
	let allTrades: TaS[] = [];
	let currentBid = 0;
	let currentAsk = 0;
	const maxLength = 20;
	let prevSecId: number = -1;
	let divideTaS = get(settings).divideTaS;
	let filterTaS = get(settings).filterTaS;

	// Fetch exchanges on mount
	onMount(() => {
		privateRequest<Exchanges>('getExchanges', {}).then((v: Exchanges) => {
			exchanges = v;
		});
	});

	function updateTradeStore(newTrade: TradeData) {
		if (newTrade.timestamp === undefined || newTrade.timestamp === 0) {
			return;
		}
		if (!filterTaS && newTrade.size < 100) {
			return;
		}

		// Skip price updates if price is -1 (indicates skip OHLC condition)
		if (newTrade.price < 0) {
			return;
		}

		if (divideTaS) {
			newTrade.size = Math.floor(newTrade.size / 100);
		}
		const exchangeName = exchanges[newTrade.exchange];
		const newRow: TaS = {
			color: getPriceColor(newTrade.price),
			...newTrade,
			exchangeName
		};
		allTrades = [newRow, ...allTrades].slice(0, maxLength);
	}
	function updateQuoteStore(last: QuoteData) {
		currentBid = last.bidPrice;
		currentAsk = last.askPrice;
	}

	// Subscribe to instance changes
	let currentSecurityId: number | null = null;
	instance.subscribe((instance: Instance) => {
		// Convert securityId to number or use null if it doesn't exist
		const securityIdNum = instance.securityId !== undefined ? Number(instance.securityId) : null;

		if (!securityIdNum || securityIdNum === prevSecId) return;
		currentSecurityId = securityIdNum;
		// Release previous streams
		releaseTrade();
		releaseQuote();

		// Add new streams using the passed update functions
		releaseTrade = addStream<TradeData>(instance, 'all', updateTradeStore) as () => void;
		releaseQuote = addStream<QuoteData>(instance, 'quote', updateQuoteStore) as () => void;

		// Reset trades
		allTrades = [];

		prevSecId = securityIdNum;
	});

	// Cleanup on destroy
	onDestroy(() => {
		unsubscribeTrade();
		releaseTrade();
		unsubscribeQuote();
		releaseQuote();
	});

	function getPriceColor(price: number): string {
		if (price > currentAsk) {
			return 'dark-green';
		} else if (price === currentAsk) {
			return 'green';
		} else if (price === currentBid) {
			return 'red';
		} else if (price < currentBid) {
			return 'dark-red';
		} else {
			return 'white';
		}
	}

	// Modify the time format to be more compact
	function formatTime(timestamp: number): string {
		const date = new Date(timestamp);
		return date.toLocaleTimeString([], {
			hour12: false,
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	}
</script>

<!-- Table for displaying time and sales data -->
<div class="time-and-sales">
	<table class="trade-table">
		{#if Array.isArray(allTrades)}
			<thead>
				<tr class="header-row">
					<th>Price</th>
					<th>{$settings.divideTaS ? 'Sz*100' : 'Size'}</th>
					<th>Exch</th>
					<th>Time</th>
				</tr>
			</thead>
			<tbody>
				{#each allTrades as trade}
					<tr class="trade-row {trade.color}">
						<td>{trade.price}</td>
						<td>{trade.size}</td>
						<td>{trade.exchangeName?.substring(0, 4) || '-'}</td>
						<td>{formatTime(trade.timestamp)}</td>
					</tr>
				{/each}
				{#each Array(maxLength - allTrades.length).fill(0) as _}
					<tr class="empty-row">
						<td>&nbsp;</td>
						<td>&nbsp;</td>
						<td>&nbsp;</td>
						<td>&nbsp;</td>
					</tr>
				{/each}
			</tbody>
		{/if}
	</table>
</div>

<style>
	.time-and-sales {
		font-family: var(
			--font-primary,
			-apple-system,
			BlinkMacSystemFont,
			Segoe UI,
			Roboto,
			Oxygen,
			Ubuntu,
			Cantarell,
			Open Sans,
			Helvetica Neue,
			sans-serif
		);
		font-size: 12px;
		width: 100%;
		overflow-y: auto;
		background-color: var(--ui-bg-secondary, rgba(18, 18, 18, 0.9));
		border-radius: 6px;
		border: 1px solid var(--ui-border, #333);
		margin-top: 5px;
	}

	.trade-table {
		width: 100%;
		border-collapse: collapse;
		table-layout: fixed;
	}

	.trade-table th,
	.trade-table td {
		padding: 3px 6px;
		text-align: right;
		font-size: 12px;
		border: none;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		line-height: 1.5;
	}

	/* First column left-aligned */
	.trade-table th:first-child,
	.trade-table td:first-child {
		text-align: left;
	}

	.trade-table th:nth-child(1),
	.trade-table td:nth-child(1) {
		width: 30%;
	}
	.trade-table th:nth-child(2),
	.trade-table td:nth-child(2) {
		width: 25%;
	}
	.trade-table th:nth-child(3),
	.trade-table td:nth-child(3) {
		width: 20%;
	}
	.trade-table th:nth-child(4),
	.trade-table td:nth-child(4) {
		width: 25%;
	}

	.trade-table th {
		color: var(--text-primary, #fff);
		font-weight: 600;
		background-color: var(--ui-bg-highlight, #222);
		padding: 3px 6px;
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		opacity: 0.85;
	}

	.header-row {
		position: sticky;
		top: 0;
		z-index: 1;
	}

	.trade-row {
		background-color: transparent;
		transition: background-color 0.15s ease;
	}

	.trade-row:hover {
		background-color: var(--ui-bg-hover, rgba(255, 255, 255, 0.05));
	}

	.empty-row td {
		color: transparent;
		height: 15px;
	}

	/* Color styles */
	.dark-green {
		color: var(--color-up-strong, #43a047);
	}
	.green {
		color: var(--color-up, #66bb6a);
	}
	.dark-red {
		color: var(--color-down-strong, #e53935);
	}
	.red {
		color: var(--color-down, #ef5350);
	}
	.white {
		color: var(--text-primary, #fff);
	}
</style>
