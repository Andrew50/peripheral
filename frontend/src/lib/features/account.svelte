<script lang="ts">
	import { privateFileRequest } from '$lib/core/backend';
	import { queueRequest } from '$lib/core/backend';
	import type { Instance } from '$lib/core/types';
	import List from '$lib/utils/modules/list.svelte';
	import { writable } from 'svelte/store';
	import { UTCTimestampToESTString } from '$lib/core/timestamp';

	// Add tab state
	let activeTab = 'trades';

	let files: FileList;
	let uploading = false;
	let message = '';
	let trades = writable<Trade[]>([]);

	let tickerStats = writable<TickerStats[]>([]);
	// Add filter states
	let sortDirection = 'desc';
	let selectedDate = '';
	let selectedHour: number | '' = '';
	let selectedTicker = '';

	// Add statistics state
	let statistics = writable<{
		total_trades: number;
		winning_trades: number;
		losing_trades: number;
		win_rate: number;
		avg_win: number;
		avg_loss: number;
		total_pnl: number;
		top_trades: Trade[];
		bottom_trades: Trade[];
		hourly_stats: {
			hour: number;
			hour_display: string;
			total_trades: number;
			winning_trades: number;
			losing_trades: number;
			win_rate: number;
			avg_pnl: number;
			total_pnl: number;
		}[];
		ticker_stats: {
			ticker: string;
			total_trades: number;
			winning_trades: number;
			losing_trades: number;
			win_rate: number;
			avg_pnl: number;
			total_pnl: number;
		}[];
	} | null>(null);

	// Add statistics filters
	let statStartDate = '';
	let statEndDate = '';
	let statTicker = '';

	interface Trade extends Instance {
		trade_direction: string;
		status: string;
		openQuantity: number;
		closedPnL: number | null;
	}
	interface TickerStats extends Instance {
		ticker: string;
		total_trades: number;
		winning_trades: number;
		losing_trades: number;
		win_rate: number;
		avg_pnl: number;
		total_pnl: number;
	}

	async function handleFileUpload() {
		if (!files || !files[0]) {
			message = 'Please select a file first';
			return;
		}

		uploading = true;
		message = 'Uploading...';

		try {
			const result = await privateFileRequest<{ trades: Trade[] }>('handle_trade_upload', files[0]);
			trades.set(result.trades);
			message = 'Upload successful!';
		} catch (error) {
			message = `Error: ${error}`;
			console.error('Upload error:', error);
		} finally {
			uploading = false;
		}
	}

	async function pullTrades() {
		try {
			console.log('pulling trades');
			const params: any = { sort: sortDirection };

			if (selectedDate) {
				params.date = selectedDate;
			}

			if (selectedHour !== '') {
				params.hour = selectedHour;
			}

			if (selectedTicker) {
				params.ticker = selectedTicker.toUpperCase();
			}

			const result = await queueRequest<Trade[]>('grab_user_trades', params);
			trades.set(result);
			console.log(result);
			message = 'Trades loaded successfully';
		} catch (error) {
			message = `Error: ${error}`;
			console.error('Load trades error:', error);
		}
	}

	async function fetchStatistics() {
		try {
			const params: any = {};

			if (statStartDate) {
				params.start_date = statStartDate;
			}

			if (statEndDate) {
				params.end_date = statEndDate;
			}

			if (statTicker) {
				params.ticker = statTicker.toUpperCase();
			}

			const result = await queueRequest('get_trade_statistics', params);
			statistics.set(result);
			message = 'Statistics loaded successfully';
		} catch (error) {
			message = `Error: ${error}`;
			console.error('Load statistics error:', error);
		}
	}

	// Add pullTickers function
	async function pullTickers() {
		try {
			const params: any = { sort: sortDirection };

			if (selectedDate) {
				params.date = selectedDate;
			}

			if (selectedHour !== '') {
				params.hour = selectedHour;
			}

			if (selectedTicker) {
				params.ticker = selectedTicker.toUpperCase();
			}

			const result = await queueRequest('get_ticker_performance', params);
			tickerStats.set(result);
			message = 'Ticker stats loaded successfully';
		} catch (error) {
			message = `Error: ${error}`;
			console.error('Load ticker stats error:', error);
		}
	}

	// Generate hours array for the select dropdown
	const hours = Array.from({ length: 24 }, (_, i) => ({
		value: i,
		label: `${i.toString().padStart(2, '0')}:00`
	}));
</script>

<div class="account-container">
	<!-- Tab Navigation -->
	<div class="tab-navigation">
		<button class:active={activeTab === 'trades'} on:click={() => (activeTab = 'trades')}>
			Trades
		</button>
		<button class:active={activeTab === 'tickers'} on:click={() => (activeTab = 'tickers')}>
			Tickers
		</button>
		<button class:active={activeTab === 'statistics'} on:click={() => (activeTab = 'statistics')}>
			Statistics
		</button>
		<button class:active={activeTab === 'other'} on:click={() => (activeTab = 'other')}>
			Other
		</button>
	</div>

	<!-- Trades Tab -->
	{#if activeTab === 'trades'}
		<div class="tab-content">
			<h2>Trade History Upload</h2>
			<div class="upload-section">
				<input type="file" accept=".csv" bind:files disabled={uploading} />
				<button on:click={handleFileUpload} disabled={uploading || !files}> Upload </button>
			</div>

			<div class="filters-section">
				<input
					type="text"
					placeholder="Ticker"
					bind:value={selectedTicker}
					on:change={pullTrades}
				/>
				<select class="default-select" bind:value={sortDirection} on:change={pullTrades}>
					<option value="desc">Newest First</option>
					<option value="asc">Oldest First</option>
				</select>

				<input type="date" bind:value={selectedDate} on:change={pullTrades} />

				<select class="default-select" bind:value={selectedHour} on:change={pullTrades}>
					<option value="">All Hours</option>
					{#each hours as hour}
						<option value={hour.value}>{hour.label}</option>
					{/each}
				</select>

				<button on:click={pullTrades}>Refresh Trades</button>
			</div>

			{#if message}
				<p class="message">{message}</p>
			{/if}

			<List
				on:contextmenu={(event) => {
					event.preventDefault();
				}}
				list={trades}
				columns={['timestamp', 'ticker', 'trade_direction', 'status', 'openQuantity', 'closedPnL']}
				formatters={{
					timestamp: (value) => (value ? UTCTimestampToESTString(value) : 'N/A'),
					closedPnL: (value) => (value !== null ? value.toFixed(2) : 'N/A')
				}}
				expandable={true}
				expandedContent={(trade) => ({
					trades: trade.trades.map((t) => ({
						time: UTCTimestampToESTString(t.time),
						type: t.type,
						price: `$${t.price.toFixed(2)}`,
						shares: t.shares
					}))
				})}
			/>
		</div>
	{/if}

	<!-- Tickers Tab -->
	{#if activeTab === 'tickers'}
		<div class="tab-content">
			<h2>Ticker Performance</h2>

			<div class="filters-section">
				<input
					type="text"
					placeholder="Ticker"
					bind:value={selectedTicker}
					on:change={pullTickers}
				/>
				<select class="default-select" bind:value={sortDirection} on:change={pullTickers}>
					<option value="desc">Newest First</option>
					<option value="asc">Oldest First</option>
				</select>

				<input type="date" bind:value={selectedDate} on:change={pullTickers} />

				<select class="default-select" bind:value={selectedHour} on:change={pullTickers}>
					<option value="">All Hours</option>
					{#each hours as hour}
						<option value={hour.value}>{hour.label}</option>
					{/each}
				</select>

				<button on:click={pullTickers}>Refresh Tickers</button>
			</div>

			{#if message}
				<p class="message">{message}</p>
			{/if}

			<List
				on:contextmenu={(event) => {
					event.preventDefault();
				}}
				list={tickerStats}
				columns={[
					'ticker',
					'total_trades',
					'win_rate',
					'winning_trades',
					'losing_trades',
					'avg_pnl',
					'total_pnl'
				]}
				formatters={{
					win_rate: (value) => `${value}%`,
					avg_pnl: (value) => `$${value.toFixed(2)}`,
					total_pnl: (value) => `$${value.toFixed(2)}`
				}}
				expandable={true}
				expandedContent={(ticker) => ({
					trades: ticker.trades.map((t) => ({
						time: UTCTimestampToESTString(t.time),
						type: t.type,
						price: `$${t.price.toFixed(2)}`,
						shares: t.shares
					}))
				})}
			/>
		</div>
	{/if}

	<!-- Statistics Tab -->
	{#if activeTab === 'statistics'}
		<div class="tab-content">
			<h2>Trading Statistics</h2>

			<div class="filters-section">
				<input
					type="text"
					placeholder="Ticker"
					bind:value={statTicker}
					on:change={fetchStatistics}
				/>
				<div class="date-range">
					<input
						type="date"
						bind:value={statStartDate}
						on:change={fetchStatistics}
						placeholder="Start Date"
					/>
					<span class="date-separator">to</span>
					<input
						type="date"
						bind:value={statEndDate}
						on:change={fetchStatistics}
						placeholder="End Date"
					/>
				</div>
				<button on:click={fetchStatistics}>Refresh Statistics</button>
			</div>

			{#if $statistics}
				<div class="statistics-grid">
					<div class="stat-card">
						<h3>Win Rate</h3>
						<p>{$statistics.win_rate}%</p>
						<small>({$statistics.winning_trades}/{$statistics.total_trades} trades)</small>
					</div>

					<div class="stat-card">
						<h3>Average Gain</h3>
						<p class="positive">${$statistics.avg_win}</p>
					</div>

					<div class="stat-card">
						<h3>Average Loss</h3>
						<p class="negative">${$statistics.avg_loss}</p>
					</div>

					<div class="stat-card">
						<h3>Total P&L</h3>
						<p class={$statistics.total_pnl >= 0 ? 'positive' : 'negative'}>
							${$statistics.total_pnl}
						</p>
					</div>
				</div>

				{#if $statistics?.top_trades && $statistics?.bottom_trades}
					<div class="best-worst-container">
						<div class="trade-list">
							<h3>Top Trades</h3>
							<table>
								<thead>
									<tr>
										<th>Date</th>
										<th>Ticker</th>
										<th>Direction</th>
										<th>P/L</th>
									</tr>
								</thead>
								<tbody>
									{#each $statistics.top_trades as trade}
										<tr>
											<td>{UTCTimestampToESTString(trade.timestamp)}</td>
											<td>{trade.ticker}</td>
											<td>{trade.direction}</td>
											<td class="positive">${trade.pnl.toFixed(2)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>

						<div class="trade-list">
							<h3>Bottom Trades</h3>
							<table>
								<thead>
									<tr>
										<th>Date</th>
										<th>Ticker</th>
										<th>Direction</th>
										<th>P/L</th>
									</tr>
								</thead>
								<tbody>
									{#each $statistics.bottom_trades as trade}
										<tr>
											<td>{UTCTimestampToESTString(trade.timestamp)}</td>
											<td>{trade.ticker}</td>
											<td>{trade.direction}</td>
											<td class="negative">${trade.pnl.toFixed(2)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					</div>
				{/if}

				{#if $statistics?.hourly_stats}
					<div class="hourly-stats-container">
						<h3>Performance by Hour</h3>
						<table class="hourly-stats-table">
							<thead>
								<tr>
									<th>Hour</th>
									<th>Total Trades</th>
									<th>Win Rate</th>
									<th>Winning Trades</th>
									<th>Losing Trades</th>
									<th>Avg P/L</th>
									<th>Total P/L</th>
								</tr>
							</thead>
							<tbody>
								{#each $statistics.hourly_stats as stat}
									<tr class:profitable={stat.total_pnl > 0}>
										<td>{stat.hour_display}</td>
										<td>{stat.total_trades}</td>
										<td>{stat.win_rate}%</td>
										<td>{stat.winning_trades}</td>
										<td>{stat.losing_trades}</td>
										<td class={stat.avg_pnl >= 0 ? 'positive' : 'negative'}>
											${stat.avg_pnl}
										</td>
										<td class={stat.total_pnl >= 0 ? 'positive' : 'negative'}>
											${stat.total_pnl}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}

				{#if $statistics?.ticker_stats}
					<div class="ticker-stats-container">
						<h3>Performance by Ticker</h3>
						<table class="ticker-stats-table">
							<thead>
								<tr>
									<th>Ticker</th>
									<th>Total Trades</th>
									<th>Win Rate</th>
									<th>Winning Trades</th>
									<th>Losing Trades</th>
									<th>Avg P/L</th>
									<th>Total P/L</th>
								</tr>
							</thead>
							<tbody>
								{#each $statistics.ticker_stats as stat}
									<tr class:profitable={stat.total_pnl > 0}>
										<td>{stat.ticker}</td>
										<td>{stat.total_trades}</td>
										<td>{stat.win_rate}%</td>
										<td>{stat.winning_trades}</td>
										<td>{stat.losing_trades}</td>
										<td class={stat.avg_pnl >= 0 ? 'positive' : 'negative'}>
											${stat.avg_pnl}
										</td>
										<td class={stat.total_pnl >= 0 ? 'positive' : 'negative'}>
											${stat.total_pnl}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			{:else}
				<p>Loading statistics...</p>
			{/if}
		</div>
	{/if}

	<!-- Other Tab -->
	{#if activeTab === 'other'}
		<div class="tab-content">
			<h2>Other</h2>
			<p>Additional content coming soon...</p>
		</div>
	{/if}
</div>

<style>
	.account-container {
		padding: 20px;
		color: white;
	}

	.tab-navigation {
		display: flex;
		gap: 10px;
		margin-bottom: 20px;
		border-bottom: 1px solid #444;
		padding-bottom: 10px;
	}

	.tab-navigation button.active {
		color: white;
		background-color: #444;
	}

	.tab-navigation button:hover {
		color: white;
		background-color: #333;
	}

	.tab-content {
		padding: 20px 0;
	}

	.upload-section {
		display: flex;
		gap: 10px;
		align-items: center;
		margin-bottom: 20px;
	}

	.filters-section {
		display: flex;
		gap: 10px;
		align-items: center;
		margin-bottom: 20px;
	}

	.message {
		margin-top: 10px;
		color: #ddd;
	}

	select,
	input[type='date'],
	input[type='text'] {
		padding: 8px;
		background-color: #333;
		color: white;
		border: 1px solid #444;
		border-radius: 4px;
	}

	select option {
		background-color: #333;
		color: white;
	}

	.statistics-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
		gap: 20px;
		margin-top: 20px;
	}

	.stat-card {
		background-color: #333;
		padding: 20px;
		border-radius: 8px;
		text-align: center;
	}

	.stat-card h3 {
		margin: 0 0 10px 0;
		font-size: 1.1em;
		color: #888;
	}

	.stat-card p {
		margin: 0;
		font-size: 1.8em;
		font-weight: bold;
	}

	.stat-card small {
		color: #888;
		font-size: 0.9em;
	}

	.positive {
		color: #4caf50;
	}

	.negative {
		color: #f44336;
	}

	.date-range {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.date-separator {
		color: #888;
	}

	.best-worst-container {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 20px;
		margin-top: 20px;
	}

	.trade-list {
		background-color: #333;
		padding: 15px;
		border-radius: 8px;
	}

	.trade-list h3 {
		margin: 0 0 10px 0;
		color: #888;
		font-size: 1.1em;
	}

	.trade-list table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9em;
	}

	.trade-list th {
		text-align: left;
		padding: 8px;
		border-bottom: 1px solid #444;
		color: #888;
	}

	.trade-list td {
		padding: 8px;
		border-bottom: 1px solid #444;
	}

	.trade-list tr:last-child td {
		border-bottom: none;
	}

	.hourly-stats-container {
		margin-top: 20px;
	}

	.hourly-stats-container h3 {
		color: #888;
		margin-bottom: 15px;
	}

	.hourly-stats-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9em;
	}

	.hourly-stats-table th {
		text-align: left;
		padding: 8px;
		border-bottom: 1px solid #444;
		color: #888;
		background-color: #333;
	}

	.hourly-stats-table td {
		padding: 8px;
		border-bottom: 1px solid #444;
	}

	.hourly-stats-table tr:hover {
		background-color: #2a2a2a;
	}

	.hourly-stats-table tr.profitable {
		background-color: rgba(76, 175, 80, 0.1);
	}

	.hourly-stats-table tr:not(.profitable) {
		background-color: rgba(244, 67, 54, 0.1);
	}

	.positive {
		color: #4caf50;
	}

	.negative {
		color: #f44336;
	}

	.ticker-stats-container {
		margin-top: 20px;
	}

	.ticker-stats-container h3 {
		color: #888;
		margin-bottom: 15px;
	}

	.ticker-stats-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9em;
	}

	.ticker-stats-table th {
		text-align: left;
		padding: 8px;
		border-bottom: 1px solid #444;
		color: #888;
		background-color: #333;
	}

	.ticker-stats-table td {
		padding: 8px;
		border-bottom: 1px solid #444;
	}

	.ticker-stats-table tr:hover {
		background-color: #2a2a2a;
	}

	.ticker-stats-table tr.profitable {
		background-color: rgba(76, 175, 80, 0.1);
	}

	.ticker-stats-table tr:not(.profitable) {
		background-color: rgba(244, 67, 54, 0.1);
	}

	:global(.profitable) {
		background: rgba(76, 175, 80, 0.1) !important;
	}

	:global(.unprofitable) {
		background: rgba(244, 67, 54, 0.1) !important;
	}

	:global(.profitable:hover) {
		background: rgba(76, 175, 80, 0.2) !important;
	}

	:global(.unprofitable:hover) {
		background: rgba(244, 67, 54, 0.2) !important;
	}
</style>
