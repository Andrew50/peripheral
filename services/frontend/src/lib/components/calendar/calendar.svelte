<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { browser } from '$app/environment';
	import { activeChartInstance } from '$lib/features/chart/interface';
	import { queryChart } from '$lib/features/chart/interface';
	import { get } from 'svelte/store';

	const dispatch = createEventDispatcher();

	export let visible = false;
	export let initialTimestamp: number | null = null;

	// Calendar state
	let calendarView: 'month' | 'year' | 'decade' = 'month';
	let currentMonth = new Date().getMonth();
	let currentYear = new Date().getFullYear();
	let selectedDate = '';
	let selectedTime = '';

	// Initialize with current date/time or provided timestamp
	function initializeDateTime() {
		const now = initialTimestamp ? new Date(initialTimestamp) : new Date();
		selectedDate = formatDateForInput(now);
		selectedTime = formatTimeForInput(now);
		currentMonth = now.getMonth();
		currentYear = now.getFullYear();
	}

	// Format date as YYYY-MM-DD
	function formatDateForInput(date: Date): string {
		return (
			date.getFullYear() +
			'-' +
			String(date.getMonth() + 1).padStart(2, '0') +
			'-' +
			String(date.getDate()).padStart(2, '0')
		);
	}

	// Format time as HH:MM
	function formatTimeForInput(date: Date): string {
		return (
			String(date.getHours()).padStart(2, '0') + ':' + String(date.getMinutes()).padStart(2, '0')
		);
	}

	// Convert selected date and time to timestamp (treating input as EST)
	function getSelectedTimestamp(): number {
		if (selectedDate && selectedTime) {
			// Parse the date and time components
			const [year, month, day] = selectedDate.split('-').map(Number);
			const [hours, minutes] = selectedTime.split(':').map(Number);

			// Create date in EST/EDT using proper timezone handling
			// This creates the time as if it were in EST, then converts to UTC
			const estDate = new Date();
			estDate.setFullYear(year, month - 1, day); // month is 0-based
			estDate.setHours(hours, minutes, 0, 0);

			// Get the EST offset for this date (handles DST automatically)
			const isDST = isDateInDST(estDate);
			const offsetHours = isDST ? -4 : -5; // EDT is UTC-4, EST is UTC-5

			// Convert to UTC by subtracting the EST offset
			const utcTimestamp = estDate.getTime() - offsetHours * 60 * 60 * 1000;

			return utcTimestamp;
		}
		return Date.now();
	}

	// Helper function to determine if a date is in Daylight Saving Time
	function isDateInDST(date: Date): boolean {
		const year = date.getFullYear();

		// DST starts on second Sunday in March
		const dstStart = new Date(year, 2, 1); // March 1st
		dstStart.setDate(dstStart.getDate() + ((14 - dstStart.getDay()) % 7)); // Second Sunday

		// DST ends on first Sunday in November
		const dstEnd = new Date(year, 10, 1); // November 1st
		dstEnd.setDate(dstEnd.getDate() + ((7 - dstEnd.getDay()) % 7)); // First Sunday

		return date >= dstStart && date < dstEnd;
	}

	function handleDateInput(event: Event) {
		const target = event.target as HTMLInputElement;
		let value = target.value;

		// Remove all non-digit characters
		let numbers = value.replace(/\D/g, '');

		// Limit to 8 digits (YYYYMMDD)
		numbers = numbers.substring(0, 8);

		// Format as YYYY-MM-DD
		let formatted = '';
		if (numbers.length > 0) {
			formatted = numbers.substring(0, 4); // Year
			if (numbers.length > 4) {
				formatted += '-' + numbers.substring(4, 6); // Month
				if (numbers.length > 6) {
					formatted += '-' + numbers.substring(6, 8); // Day
				}
			}
		}

		// Update the input value
		target.value = formatted;
		selectedDate = formatted;

		// Update current month/year if we have a complete date
		if (formatted.length === 10) {
			const dateParts = formatted.split('-');
			const year = parseInt(dateParts[0]);
			const month = parseInt(dateParts[1]) - 1; // Month is 0-based

			if (year >= 1900 && year <= 2100 && month >= 0 && month <= 11) {
				currentYear = year;
				currentMonth = month;
			}
		}
	}

	function handleDateKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			handleConfirm();
		}
	}

	function handleTimeInput(event: Event) {
		const target = event.target as HTMLInputElement;
		let value = target.value;

		// Allow only digits and colon, limit length
		value = value.replace(/[^\d:]/g, '').substring(0, 5);

		// Auto-insert colon when typing 3rd digit (if no colon exists)
		if (value.length === 3 && !value.includes(':')) {
			value = value.substring(0, 2) + ':' + value.substring(2);
		}

		// Validate and correct hours
		if (value.length >= 2) {
			let hours = parseInt(value.substring(0, 2));
			if (hours > 23) {
				value = '23' + value.substring(2);
			}
		}

		// Validate and correct minutes
		if (value.length === 5 && value.includes(':')) {
			let minutes = parseInt(value.substring(3, 5));
			if (minutes > 59) {
				value = value.substring(0, 3) + '59';
			}
		}

		selectedTime = value;
	}

	function handleTimeKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			handleConfirm();
		}
	}

	// Calendar navigation functions
	function previousPeriod() {
		if (calendarView === 'month') {
			currentMonth--;
			if (currentMonth < 0) {
				currentMonth = 11;
				currentYear--;
			}
		} else if (calendarView === 'year') {
			currentYear--;
		} else if (calendarView === 'decade') {
			currentYear -= 10;
		}
	}

	function nextPeriod() {
		if (calendarView === 'month') {
			currentMonth++;
			if (currentMonth > 11) {
				currentMonth = 0;
				currentYear++;
			}
		} else if (calendarView === 'year') {
			currentYear++;
		} else if (calendarView === 'decade') {
			currentYear += 10;
		}
	}

	function switchCalendarView() {
		if (calendarView === 'month') {
			calendarView = 'year';
		} else if (calendarView === 'year') {
			calendarView = 'decade';
		}
	}

	function getMonthName(month: number): string {
		const monthNames = [
			'January',
			'February',
			'March',
			'April',
			'May',
			'June',
			'July',
			'August',
			'September',
			'October',
			'November',
			'December'
		];
		return monthNames[month];
	}

	// Generate calendar days - make it reactive
	let daysInMonth: any[] = [];

	$: {
		const days = [];
		const firstDay = new Date(currentYear, currentMonth, 1);
		const lastDay = new Date(currentYear, currentMonth + 1, 0);
		const startDate = new Date(firstDay);
		startDate.setDate(startDate.getDate() - firstDay.getDay());

		const today = new Date();
		const selectedDateObj = selectedDate ? new Date(selectedDate + 'T00:00:00') : null;

		for (let i = 0; i < 42; i++) {
			const date = new Date(startDate);
			date.setDate(startDate.getDate() + i);

			const isCurrentMonth = date.getMonth() === currentMonth;
			const isToday = date.toDateString() === today.toDateString();
			const isSelected = selectedDateObj && date.toDateString() === selectedDateObj.toDateString();

			days.push({
				day: date.getDate(),
				date: date,
				isCurrentMonth,
				isToday,
				isSelected
			});
		}

		daysInMonth = days;
	}

	function selectDate(day: any) {
		if (!day.isCurrentMonth) return;

		selectedDate = formatDateForInput(day.date);
	}

	// Generate months for year view - make it reactive
	let monthsInYear: any[] = [];

	$: {
		const months = [];
		const monthNames = [
			'Jan',
			'Feb',
			'Mar',
			'Apr',
			'May',
			'Jun',
			'Jul',
			'Aug',
			'Sep',
			'Oct',
			'Nov',
			'Dec'
		];

		for (let i = 0; i < 12; i++) {
			months.push({
				month: i,
				name: monthNames[i],
				isSelected: i === currentMonth
			});
		}
		monthsInYear = months;
	}

	function selectMonth(month: any) {
		currentMonth = month.month;
		calendarView = 'month';
	}

	// Generate years for decade view - make it reactive
	let yearsInDecade: any[] = [];

	$: {
		const years = [];
		const startYear = Math.floor(currentYear / 10) * 10;

		for (let i = 0; i < 12; i++) {
			const year = startYear + i - 1;
			years.push({
				year: year,
				isSelected: year === currentYear
			});
		}
		yearsInDecade = years;
	}

	function selectYear(year: any) {
		currentYear = year.year;
		calendarView = 'year';
	}

	function handleCancel() {
		visible = false;
		dispatch('cancel');
	}

	async function handleConfirm() {
		const timestamp = getSelectedTimestamp();

		// Get current chart instance from the store
		const currentChart = get(activeChartInstance);
		if (!currentChart) return;
		// Create new instance preserving everything except timestamp
		const updatedInstance = {
			...currentChart,
			timestamp: timestamp,
			// Explicitly preserve the key fields
			ticker: currentChart.ticker,
			timeframe: currentChart.timeframe,
			extendedHours: currentChart.extendedHours,
			securityId: currentChart.securityId
		};
		console.log(updatedInstance);
		// Call queryChart directly - no input system needed
		queryChart(updatedInstance, true);

		visible = false;
		dispatch('confirm', { timestamp });
	}

	// Handle keyboard events
	function handleKeyboard(event: KeyboardEvent) {
		if (!visible) return;
		if (event.key === 'Escape') {
			handleCancel();
		} else if (event.key === 'Enter') {
			handleConfirm();
		}
	}

	// Add/remove keyboard listener when visibility changes
	$: if (browser) {
		if (visible) {
			document.addEventListener('keydown', handleKeyboard);
		} else {
			document.removeEventListener('keydown', handleKeyboard);
		}
	}

	// Initialize when component becomes visible
	$: if (visible && browser) {
		initializeDateTime();
	}
</script>

{#if visible}
	<div 
		class="popup-container calendar-popup" 
		id="calendar-window" 
		on:click|stopPropagation
		on:keydown={(e) => {
			if (e.key === 'Escape') {
				e.preventDefault();
				handleCancel();
			}
		}}
		role="dialog"
		aria-label="Calendar picker"
	>
		<div class="content-container">
			<div class="calendar-header-section">
				<span class="calendar-title">Go to Date</span>
				<button class="close-button" on:click={handleCancel}>×</button>
			</div>

			<div class="calendar-inputs">
				<div class="input-group">
					<label class="input-label" for="date-input">Date</label>
					<input
						id="date-input"
						type="text"
						class="date-input"
						placeholder="YYYY-MM-DD"
						bind:value={selectedDate}
						on:input={handleDateInput}
						on:keydown={handleDateKeydown}
					/>
				</div>
				<div class="input-group">
					<label class="input-label" for="time-input">Time (EST)</label>
					<input
						id="time-input"
						type="text"
						class="time-input"
						placeholder="HH:MM"
						maxlength="5"
						bind:value={selectedTime}
						on:input={handleTimeInput}
						on:keydown={handleTimeKeydown}
					/>
				</div>
			</div>

			<div class="calendar-container">
				<div class="calendar-header">
					<button class="nav-button" on:click={previousPeriod}>
						<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
							<path
								d="M15 18L9 12L15 6"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							/>
						</svg>
					</button>

					<button
						class="calendar-nav-title"
						on:click={switchCalendarView}
						on:keydown={(e) => {
							if (e.key === 'Enter' || e.key === ' ') {
								e.preventDefault();
								switchCalendarView();
							}
						}}
					>
						{#if calendarView === 'month'}
							{getMonthName(currentMonth)} {currentYear}
						{:else if calendarView === 'year'}
							{currentYear}
						{:else if calendarView === 'decade'}
							{Math.floor(currentYear / 10) * 10}-{Math.floor(currentYear / 10) * 10 + 9}
						{/if}
					</button>

					<button class="nav-button" on:click={nextPeriod}>
						<svg viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
							<path
								d="M9 18L15 12L9 6"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
							/>
						</svg>
					</button>
				</div>

				{#if calendarView === 'month'}
					<div class="calendar-weekdays">
						<span>Su</span><span>Mo</span><span>Tu</span><span>We</span><span>Th</span><span
							>Fr</span
						><span>Sa</span>
					</div>
					<div class="calendar-days">
						{#each daysInMonth as day}
							<button
								class="day-button {day.isCurrentMonth
									? 'current-month'
									: 'other-month'} {day.isSelected ? 'selected' : ''} {day.isToday ? 'today' : ''}"
								on:click={() => selectDate(day)}
								disabled={!day.isCurrentMonth}
							>
								{day.day}
							</button>
						{/each}
					</div>
				{:else if calendarView === 'year'}
					<div class="calendar-months">
						{#each monthsInYear as month}
							<button
								class="month-button {month.isSelected ? 'selected' : ''}"
								on:click={() => selectMonth(month)}
							>
								{month.name}
							</button>
						{/each}
					</div>
				{:else if calendarView === 'decade'}
					<div class="calendar-years">
						{#each yearsInDecade as year}
							<button
								class="year-button {year.isSelected ? 'selected' : ''}"
								on:click={() => selectYear(year)}
							>
								{year.year}
							</button>
						{/each}
					</div>
				{/if}
			</div>

			<div class="calendar-actions">
				<button class="action-button cancel" on:click={handleCancel}>Cancel</button>
				<button class="action-button confirm" on:click={handleConfirm}>Confirm</button>
			</div>
		</div>
	</div>
{/if}

<style>
	/* Copy exact structure from input component */
	.popup-container.calendar-popup {
		width: min(400px, 85vw);
		height: auto;
		max-height: 60vh;
		background: transparent;
		border: none;
		border-radius: 0;
		display: flex;
		flex-direction: column;
		overflow: visible;
		box-shadow: none;
		position: fixed !important;
		bottom: max(5vh, 60px) !important;
		left: 50% !important;
		top: auto !important;
		transform: translateX(-50%) !important;
		z-index: 99999 !important;
		gap: 0.5rem;
	}

	.content-container {
		background: rgba(0, 0, 0, 0.5);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 0.75rem;
		overflow-y: auto;
		padding: 1rem;
		height: auto;
		max-height: 50vh;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
		backdrop-filter: var(--backdrop-blur);
		scrollbar-width: thin;
		scrollbar-color: rgba(255, 255, 255, 0.3) transparent;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.calendar-header-section {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0 0.25rem;
	}

	.calendar-title {
		color: #ffffff;
		font-size: 0.875rem;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		opacity: 0.9;
	}

	.close-button {
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 0.375rem;
		color: #ffffff;
		font-size: 1rem;
		line-height: 1;
		padding: 0.25rem 0.5rem;
		cursor: pointer;
		transition: background-color 0.15s ease;
		width: 1.5rem;
		height: 1.5rem;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.close-button:hover {
		background: rgba(255, 255, 255, 0.2);
	}

	.calendar-inputs {
		display: flex;
		gap: 0.75rem;
	}

	.input-group {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
	}

	.input-label {
		color: #ffffff;
		font-size: 0.75rem;
		font-weight: 600;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
		opacity: 0.8;
	}

	.date-input,
	.time-input {
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.3);
		border-radius: 0.375rem;
		padding: 0.5rem;
		color: #ffffff;
		font-size: 0.875rem;
		font-weight: 500;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.date-input:focus,
	.time-input:focus {
		outline: none;
		border-color: #4a80f0;
		box-shadow: 0 0 0 2px rgba(74, 128, 240, 0.2);
	}

	.date-input::placeholder {
		color: rgba(255, 255, 255, 0.5);
	}

	.calendar-container {
		background: rgba(0, 0, 0, 0.2);
		border: 1px solid rgba(255, 255, 255, 0.15);
		border-radius: 0.5rem;
		padding: 0.75rem;
	}

	.calendar-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.75rem;
	}

	.nav-button {
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 0.375rem;
		padding: 0.375rem;
		color: #ffffff;
		cursor: pointer;
		display: flex;
		align-items: center;
		justify-content: center;
		transition: background-color 0.15s ease;
	}

	.nav-button:hover {
		background: rgba(255, 255, 255, 0.2);
	}

	.nav-button svg {
		width: 0.875rem;
		height: 0.875rem;
	}

	.calendar-nav-title {
		background: transparent;
		border: none;
		color: #ffffff;
		font-size: 0.9rem;
		font-weight: 600;
		cursor: pointer;
		padding: 0.375rem 0.75rem;
		border-radius: 0.375rem;
		transition: background-color 0.15s ease;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
	}

	.calendar-nav-title:hover {
		background: rgba(255, 255, 255, 0.1);
	}

	.calendar-weekdays {
		display: grid;
		grid-template-columns: repeat(7, 1fr);
		gap: 0.25rem;
		margin-bottom: 0.5rem;
	}

	.calendar-weekdays span {
		text-align: center;
		color: rgba(255, 255, 255, 0.7);
		font-size: 0.625rem;
		font-weight: 600;
		padding: 0.25rem 0;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.6);
	}

	.calendar-days {
		display: grid;
		grid-template-columns: repeat(7, 1fr);
		gap: 0.125rem;
	}

	.day-button {
		background: transparent;
		border: 1px solid transparent;
		border-radius: 0.25rem;
		padding: 0.25rem;
		color: #ffffff;
		cursor: pointer;
		font-size: 0.75rem;
		font-weight: 500;
		transition: all 0.15s ease;
		min-height: 1.5rem;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.day-button.other-month {
		color: rgba(255, 255, 255, 0.3);
		cursor: not-allowed;
	}

	.day-button.current-month:hover {
		background: rgba(255, 255, 255, 0.1);
		border-color: rgba(255, 255, 255, 0.3);
	}

	.day-button.selected {
		background: #4a80f0;
		border-color: #4a80f0;
		color: #ffffff;
		font-weight: 600;
	}

	.day-button.today {
		border-color: #4a80f0;
		color: #4a80f0;
		font-weight: 600;
	}

	.day-button.selected.today {
		background: #4a80f0;
		color: #ffffff;
	}

	.calendar-months,
	.calendar-years {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.375rem;
	}

	.month-button,
	.year-button {
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.2);
		border-radius: 0.375rem;
		padding: 0.5rem 0.375rem;
		color: #ffffff;
		cursor: pointer;
		font-size: 0.75rem;
		font-weight: 500;
		transition: all 0.15s ease;
		text-align: center;
	}

	.month-button:hover,
	.year-button:hover {
		background: rgba(255, 255, 255, 0.2);
		border-color: rgba(255, 255, 255, 0.4);
	}

	.month-button.selected,
	.year-button.selected {
		background: #4a80f0;
		border-color: #4a80f0;
		color: #ffffff;
		font-weight: 600;
	}

	.calendar-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		margin-top: 0.25rem;
	}

	.action-button {
		padding: 0.5rem 1rem;
		border-radius: 0.375rem;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s ease;
		font-size: 0.75rem;
	}

	.action-button.cancel {
		background: rgba(255, 255, 255, 0.1);
		border: 1px solid rgba(255, 255, 255, 0.3);
		color: #ffffff;
	}

	.action-button.cancel:hover {
		background: rgba(255, 255, 255, 0.2);
	}

	.action-button.confirm {
		background: #4a80f0;
		border: 1px solid #4a80f0;
		color: #ffffff;
	}

	.action-button.confirm:hover {
		background: #3a70e0;
		border-color: #3a70e0;
	}

	@media (max-width: 768px) {
		.popup-container.calendar-popup {
			width: min(350px, 90vw);
		}

		.calendar-inputs {
			flex-direction: column;
			gap: 0.5rem;
		}

		.calendar-container {
			padding: 0.5rem;
		}

		.calendar-months,
		.calendar-years {
			grid-template-columns: repeat(4, 1fr);
		}
	}
</style>
