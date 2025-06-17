<script lang="ts">
	import { onMount } from 'svelte';
	import { privateRequest } from '$lib/utils/helpers/backend';
	import { writable } from 'svelte/store';

	interface DailyStat {
		date: string; // YYYY-MM-DD
		total_pnl: number;
		trade_count: number;
	}

	interface WeeklyStat {
		week_start_date: string; // YYYY-MM-DD of the Sunday
		total_pnl: number;
		trade_count: number;
	}

	interface MonthlyStatsResponse {
		daily_stats: DailyStat[];
		monthly_pnl: number;
		weekly_stats: WeeklyStat[];
	}

	let currentDate = new Date();
	let currentYear = writable(currentDate.getFullYear());
	let currentMonth = writable(currentDate.getMonth()); // 0-indexed (0 = January)
	let dailyStatsMap = writable<Map<string, DailyStat>>(new Map());
	let monthlyPnl = writable<number>(0);
	let weeklyStats = writable<Map<string, WeeklyStat>>(new Map());
	let weeks = writable<Array<Array<Date | null>>>([]);
	let isLoading = writable(false);
	let errorMessage = writable<string | null>(null);

	const monthNames = [
		'January', 'February', 'March', 'April', 'May', 'June',
		'July', 'August', 'September', 'October', 'November', 'December'
	];

	async function fetchDailyStats(year: number, month: number) {
		isLoading.set(true);
		errorMessage.set(null);
		try {
			const response = await privateRequest<MonthlyStatsResponse>('get_daily_trade_stats', {
				year: year,
				month: month + 1 // Backend expects 1-indexed month
			});

			const statsMap = new Map<string, DailyStat>();
			response.daily_stats.forEach(stat => {
				statsMap.set(stat.date, stat);
			});
			dailyStatsMap.set(statsMap);
			monthlyPnl.set(response.monthly_pnl);

			// Populate weekly stats map
			const weeklyStatsMap = new Map<string, WeeklyStat>();
			response.weekly_stats.forEach(stat => {
				weeklyStatsMap.set(stat.week_start_date, stat);
			});
			weeklyStats.set(weeklyStatsMap);

			generateCalendar(year, month);
		} catch (error) {
			console.error('Error fetching daily stats:', error);
			errorMessage.set('Failed to load calendar data.');
		} finally {
			isLoading.set(false);
		}
	}

	function generateCalendar(year: number, month: number) {
		const monthWeeks: Array<Array<Date | null>> = [];
		const firstDayOfMonth = new Date(year, month, 1);
		const lastDayOfMonth = new Date(year, month + 1, 0);
		const firstDayWeekday = firstDayOfMonth.getDay(); // 0 = Sunday, 1 = Monday, etc.

		let currentWeek: Array<Date | null> = [];

		// Add padding for days before the 1st of the month
		for (let i = 0; i < firstDayWeekday; i++) {
			currentWeek.push(null);
		}

		// Add days of the month
		for (let day = 1; day <= lastDayOfMonth.getDate(); day++) {
			currentWeek.push(new Date(year, month, day));
			if (currentWeek.length === 7) {
				monthWeeks.push(currentWeek);
				currentWeek = [];
			}
		}

		// Add padding for days after the last day of the month
		if (currentWeek.length > 0) {
			while (currentWeek.length < 7) {
				currentWeek.push(null);
			}
			monthWeeks.push(currentWeek);
		}

		weeks.set(monthWeeks);
	}

	function changeMonth(delta: number) {
		let newMonth = $currentMonth + delta;
		let newYear = $currentYear;

		if (newMonth > 11) {
			newMonth = 0;
			newYear++;
		} else if (newMonth < 0) {
			newMonth = 11;
			newYear--;
		}

		currentMonth.set(newMonth);
		currentYear.set(newYear);
		fetchDailyStats(newYear, newMonth);
	}

	function changeYear(delta: number) {
		const newYear = $currentYear + delta;
		currentYear.set(newYear);
		fetchDailyStats(newYear, $currentMonth);
	}

	onMount(() => {
		fetchDailyStats($currentYear, $currentMonth);
	});

	function getDayKey(date: Date): string {
		const year = date.getFullYear();
		const month = (date.getMonth() + 1).toString().padStart(2, '0');
		const day = date.getDate().toString().padStart(2, '0');
		return `${year}-${month}-${day}`;
	}

	// Helper function to get the start of the week (Sunday) for a given date
	function getWeekStartDate(date: Date): Date {
		const tempDate = new Date(date); // Clone date to avoid modifying original
		const dayOfWeek = tempDate.getDay(); // 0 = Sunday, 6 = Saturday
		const diff = tempDate.getDate() - dayOfWeek;
		return new Date(tempDate.setDate(diff));
	}

	// Helper function to format a date as YYYY-MM-DD key
	function formatDateKey(date: Date): string {
		const year = date.getFullYear();
		const month = (date.getMonth() + 1).toString().padStart(2, '0');
		const day = date.getDate().toString().padStart(2, '0');
		return `${year}-${month}-${day}`;
	}

</script>

<div class="trade-calendar-container">
	<div class="calendar-header">
		<button on:click={() => changeYear(-1)}>&lt;&lt;</button>
		<button on:click={() => changeMonth(-1)}>&lt;</button>
		<h2>{monthNames[$currentMonth]} {$currentYear}</h2>
		<button on:click={() => changeMonth(1)}>&gt;</button>
		<button on:click={() => changeYear(1)}>&gt;&gt;</button>
		<div class="monthly-pnl">
			Monthly P&L: <span class={$monthlyPnl >= 0 ? 'positive' : 'negative'}>${$monthlyPnl.toFixed(2)}</span>
		</div>
	</div>

	{#if $isLoading}
		<div class="loading-message">Loading calendar...</div>
	{:else if $errorMessage}
		<div class="error-message">{$errorMessage}</div>
	{:else}
		<div class="calendar-grid-container">
			<div class="calendar-header-grid">
				<div>Sun</div>
				<div>Mon</div>
				<div>Tue</div>
				<div>Wed</div>
				<div>Thu</div>
				<div>Fri</div>
				<div>Sat</div>
				<div class="weekly-header">Total</div>
			</div>
			<div class="calendar-grid">
				{#each $weeks as week, weekIndex}
					{#each week as day}
						<div class="calendar-day" class:empty={!day}>
							{#if day}
								{@const dayKey = getDayKey(day)}
								{@const stat = $dailyStatsMap.get(dayKey)}
								<div class="day-number">{day.getDate()}</div>
								<div class="day-stats">
									{#if stat}
										<div class="pnl" class:positive={stat.total_pnl > 0} class:negative={stat.total_pnl < 0}>
											${stat.total_pnl.toFixed(2)}
										</div>
										<div class="trade-count">{stat.trade_count} trade{stat.trade_count !== 1 ? 's' : ''}</div>
									{:else}
										<div class="pnl">$0.00</div>
										<div class="trade-count">0 trades</div>
									{/if}
								</div>
							{/if}
						</div>
					{/each}
					<!-- Weekly Total Cell - Updated Lookup Logic -->
					{@const firstDayOfWeek = week.find(d => d !== null)}
					{@const weekStartDateKey = firstDayOfWeek ? formatDateKey(getWeekStartDate(firstDayOfWeek)) : null}
					{@const weeklyStat = weekStartDateKey ? $weeklyStats.get(weekStartDateKey) : null}
					<div class="calendar-day weekly-total">
						{#if weeklyStat}
							<div class="day-stats">
								<div class="pnl" class:positive={weeklyStat.total_pnl > 0} class:negative={weeklyStat.total_pnl < 0}>
									${weeklyStat.total_pnl.toFixed(2)}
								</div>
								<div class="trade-count">{weeklyStat.trade_count} trade{weeklyStat.trade_count !== 1 ? 's' : ''}</div>
							</div>
						{:else}
							<div class="day-stats">
								<div class="pnl">$0.00</div>
								<div class="trade-count">0 trades</div>
							</div>
						{/if}
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>

<style>
	.trade-calendar-container {
		font-family: sans-serif;
		color: var(--text-primary, #ccc);
		background-color: var(--ui-bg-primary, #1e1e1e);
		padding: 55px;
		border-radius: 8px;
	}

	.calendar-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 15px;
		flex-wrap: wrap;
	}

	.calendar-header h2 {
		margin: 0 10px;
		font-size: 1.4em;
		flex-grow: 1;
		text-align: center;
	}

	.calendar-header button {
		background: var(--ui-bg-element, #333);
		border: 1px solid var(--ui-border, #444);
		color: var(--text-primary, #ccc);
		padding: 5px 10px;
		border-radius: 4px;
		cursor: pointer;
	}

	.calendar-header button:hover {
		background: var(--ui-bg-hover, #444);
	}

	.monthly-pnl {
		margin-left: auto;
		padding-left: 15px;
		font-size: 0.9em;
		white-space: nowrap;
	}

	.calendar-grid-container {
		display: flex;
		flex-direction: column;
	}

	.calendar-header-grid,
	.calendar-grid {
		display: grid;
		grid-template-columns: repeat(8, 1fr);
		gap: 1px;
		background-color: var(--ui-border, #444);
		border: 1px solid var(--ui-border, #444);
	}

	.calendar-header-grid > div {
		text-align: center;
		padding: 8px 0;
		font-weight: normal;
		color: var(--text-secondary, #888);
		background-color: var(--ui-bg-primary, #1e1e1e);
	}

	.calendar-grid {
		border-top: none;
	}

	.calendar-day {
		border: 1px solid var(--ui-border, #444);
		height: 100px;
		vertical-align: top;
		padding: 5px;
		position: relative;
		background-color: var(--ui-bg-element, #2a2a2a);
		aspect-ratio: 1 / 1;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
	}

	.calendar-day.empty {
		background-color: transparent;
		border: none;
	}

	.day-number {
		font-size: 0.9em;
		color: var(--text-secondary, #888);
		margin-bottom: 5px;
	}

	.day-stats {
		font-size: 0.9em;
	}

	.pnl {
		font-weight: bold;
		margin-bottom: 3px;
		color: var(--text-primary, #ccc);
	}

	.trade-count {
		font-size: 0.8em;
		color: var(--text-secondary, #888);
	}

	.positive {
		color: var(--positive, #4caf50);
	}

	.negative {
		color: var(--negative, #f44336);
	}

	.loading-message, .error-message {
		text-align: center;
		padding: 20px;
		color: var(--text-secondary, #888);
	}

	.error-message {
		color: var(--negative, #f44336);
	}

	.weekly-total {
		background-color: var(--ui-bg-secondary, #222);
		justify-content: center;
		align-items: flex-end;
	}

	.weekly-total .day-stats {
		text-align: right;
	}
</style> 