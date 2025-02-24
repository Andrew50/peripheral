<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { streamInfo } from '$lib/core/stores';
	import { writable } from 'svelte/store';
	import type { Instance } from '$lib/core/types';
	import {
		UTCSecondstoESTSeconds,
		ESTSecondstoUTCSeconds,
		ESTSecondstoUTCMillis,
		getReferenceStartTimeForDateMilliseconds,
		timeframeToSeconds
	} from '$lib/core/timestamp';

	export let instance: Instance;
	export let currentBarTimestamp: number;
	const countdown = writable('Bar Closed');

	let interval: ReturnType<typeof setInterval>;

	function formatTime(seconds: number): string {
		const years = Math.floor(seconds / (365 * 24 * 60 * 60));
		const months = Math.floor((seconds % (365 * 24 * 60 * 60)) / (30 * 24 * 60 * 60));
		const weeks = Math.floor((seconds % (30 * 24 * 60 * 60)) / (7 * 24 * 60 * 60));
		const days = Math.floor((seconds % (7 * 24 * 60 * 60)) / (24 * 60 * 60));
		const hours = Math.floor((seconds % (24 * 60 * 60)) / (60 * 60));
		const minutes = Math.floor((seconds % (60 * 60)) / 60);
		const secs = Math.floor(seconds % 60);

		if (years > 0) return `${years}y ${months}m`;
		if (months > 0) return `${months}m ${weeks}w`;
		if (weeks > 0) return `${weeks}w ${days}d`;
		if (days > 0) return `${days}d ${hours}h`;
		if (hours > 0) return `${hours}h ${minutes}m`;
		if (minutes > 0) return `${minutes}m ${secs < 10 ? '0' : ''}${secs}s`;
		return `${secs < 10 ? '0' : ''}${secs}s`;
	}

	function calculateCountdown() {
		if (!instance.timeframe) return;

		const currentTimeInSeconds = Math.floor($streamInfo.timestamp / 1000);
		console.log('Current time (seconds):', currentTimeInSeconds);
		console.log('Current bar timestamp:', currentBarTimestamp);

		// If currentBarTimestamp is undefined, use the current time rounded down to the nearest day
		if (typeof currentBarTimestamp === 'undefined') {
			const currentDate = new Date(currentTimeInSeconds * 1000);
			currentDate.setHours(0, 0, 0, 0);
			currentBarTimestamp = Math.floor(currentDate.getTime() / 1000);
		}

		const chartTimeframeInSeconds = timeframeToSeconds(instance.timeframe, currentTimeInSeconds);
		console.log('Timeframe in seconds:', chartTimeframeInSeconds);

		let nextBarClose = currentBarTimestamp + chartTimeframeInSeconds;

		// For daily timeframes, adjust to market close (4:00 PM EST)
		if (instance.timeframe.includes('d')) {
			// Convert nextBarClose to EST date
			const nextCloseDate = new Date(nextBarClose * 1000);
			const estOptions = { timeZone: 'America/New_York', hour12: false };
			const formatter = new Intl.DateTimeFormat('en-US', {
				...estOptions,
				year: 'numeric',
				month: 'numeric',
				day: 'numeric'
			});

			// Set to 4:00 PM EST of the same day
			const [month, day, year] = formatter.format(nextCloseDate).split('/');
			const marketCloseDate = new Date(
				`${year}-${month.padStart(2, '0')}-${day.padStart(2, '0')}T16:00:00`
			);
			marketCloseDate.setTime(
				marketCloseDate.getTime() + marketCloseDate.getTimezoneOffset() * 60 * 1000
			);
			nextBarClose = Math.floor(marketCloseDate.getTime() / 1000);
		}

		console.log('Next bar close:', nextBarClose);

		const currentTimeEST = UTCSecondstoESTSeconds(currentTimeInSeconds);
		console.log('Current time EST:', currentTimeEST);

		const remainingTime = nextBarClose - currentTimeEST;
		console.log('Remaining time:', remainingTime);

		if (remainingTime > 0) {
			const formattedTime = formatTime(remainingTime);
			console.log('Formatted time:', formattedTime);
			countdown.set(formattedTime);
		} else {
			countdown.set('Bar Closed');
		}
	}

	onMount(() => {
		interval = setInterval(() => {
			calculateCountdown();
		}, 1000);
	});

	onDestroy(() => {
		clearInterval(interval);
	});
</script>

<div class="countdown-overlay">
	{$countdown}
</div>

<style>
	.countdown-overlay {
		position: absolute; /* Position relative to the parent div */
		bottom: 20px; /* Position from the bottom of the parent div */
		right: 20px; /* Position from the right of the parent div */
		background: rgba(0, 0, 0, 0.7); /* Semi-transparent background */
		padding: 10px 20px; /* Padding for better spacing */
		border-radius: 10px; /* Rounded corners */
		color: white; /* Text color */
		font-size: 12px; /* Adjust font size */
		z-index: 990; /* Ensure it's on top of other elements */
		box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3); /* Add a subtle shadow */
	}
</style>
