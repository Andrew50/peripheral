import { UTCSecondstoESTSeconds } from '$lib/utils/helpers/timestamp';
import html2canvas from 'html2canvas';
import type { UTCTimestamp } from 'lightweight-charts';

export async function handleScreenshot(chartId: string) {
	try {
		// Get the entire chart container including legend
		const chartContainer = document.getElementById(`chart_container-${chartId}`);
		if (!chartContainer) return;

		// Use html2canvas to capture the entire container
		const canvas = await html2canvas(chartContainer, {
			backgroundColor: 'black', // Match your chart background
			scale: 2, // Higher quality
			logging: false,
			useCORS: true
		});

		// Convert to blob
		const blob = await new Promise<Blob>((resolve) => {
			canvas.toBlob((blob) => {
				if (blob) resolve(blob);
			}, 'image/png');
		});

		// Copy to clipboard
		await navigator.clipboard.write([
			new ClipboardItem({
				[blob.type]: blob
			})
		]);

		console.log('Chart copied to clipboard!');
	} catch (error) {
		console.error('Failed to copy chart:', error);
	}
}

// Improved function to adjust events to trading days and handle collisions
export function adjustEventsToTradingDays(events: any[], candleData: any[]) {
	// Exit early if we don't have both event data and candle data
	if (!events.length || !candleData.length) return events;

	// Create a map of all valid trading timestamps
	const validTradingTimes = new Set<number>();
	candleData.forEach((candle) => {
		validTradingTimes.add(typeof candle.time === 'number' ? candle.time : Number(candle.time));
	});

	// Sort trading times for binary search
	const sortedTradingTimes = Array.from(validTradingTimes).sort((a, b) => a - b);

	// First pass: adjust all events to valid trading days
	const adjustedEvents = events.map((event) => {
		const eventTime = event.time;

		// If this timestamp already exists in our candle data, keep it as is
		if (validTradingTimes.has(eventTime)) {
			return event;
		}

		// Find the closest trading day (preferring the next trading day)
		let closest = sortedTradingTimes[0]; // Default to first trading day
		let minDiff = Math.abs(eventTime - closest);

		for (const tradingTime of sortedTradingTimes) {
			const diff = tradingTime - eventTime;

			// Prefer the next trading day (positive diff) when possible
			if (diff > 0 && diff < minDiff) {
				closest = tradingTime;
				minDiff = diff;
			}
			// If we can't find a next trading day, use the previous closest
			else if (diff < 0 && Math.abs(diff) < minDiff) {
				closest = tradingTime;
				minDiff = Math.abs(diff);
			}
		}

		// Return modified event with adjusted time
		return {
			...event,
			time: closest
		};
	});

	// Second pass: merge events that now share the same timestamp
	const mergedEventsMap = new Map<number, any>();

	adjustedEvents.forEach((event) => {
		const time = event.time;

		if (!mergedEventsMap.has(time)) {
			// First event at this timestamp - just add it
			mergedEventsMap.set(time, {
				time,
				events: [...event.events]
			});
		} else {
			// We already have an event at this timestamp - merge the events arrays
			const existingEvent = mergedEventsMap.get(time);

			// Combine and deduplicate events
			const combinedEvents = [...existingEvent.events, ...event.events];

			// Simple deduplication (assuming events with the same title are duplicates)
			const uniqueEvents = [];
			const seenTitles = new Set();

			for (const eventItem of combinedEvents) {
				// Create a unique key for this event (type + title)
				const eventKey = `${eventItem.type}:${eventItem.title}`;

				if (!seenTitles.has(eventKey)) {
					seenTitles.add(eventKey);
					uniqueEvents.push(eventItem);
				}
			}

			// Update the event with the merged, deduplicated array
			mergedEventsMap.set(time, {
				time,
				events: uniqueEvents
			});
		}
	});

	// Convert the map back to an array
	return Array.from(mergedEventsMap.values());
}

export function extendedHours(timestamp: number): boolean {
	// Convert timestamp to Eastern Time
	const estTimestampSeconds = UTCSecondstoESTSeconds((timestamp / 1000) as UTCTimestamp);
	const date = new Date(estTimestampSeconds * 1000);
	const minutes = date.getUTCHours() * 60 + date.getUTCMinutes();
	return minutes < 570 || minutes >= 960; // 9:30 AM - 4:00 PM EST
}
