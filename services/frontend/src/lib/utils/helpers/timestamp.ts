import { DateTime } from 'luxon';

import { systemClockOffset } from '../stores/stores';

function getEasternTimeOffset(date: Date) {
	//helper function
	const options: Intl.DateTimeFormatOptions = {
		timeZone: 'America/New_York',
		timeZoneName: 'short'
	};
	const formatter = new Intl.DateTimeFormat([], options);
	const parts: Intl.DateTimeFormatPart[] = formatter.formatToParts(date);
	// Extract the time zone name (EDT or EST) from the formatted parts
	const timeZoneAbbr = parts.find((part) => part.type === 'timeZoneName')?.value;

	if (!timeZoneAbbr) {
		return 0;
	}
	// Determine the offset: EDT is UTC-4, EST is UTC-5
	if (timeZoneAbbr === 'EDT') {
		return -4 * 60 * 60; // UTC-4 in seconds
	} else if (timeZoneAbbr === 'EST') {
		return -5 * 60 * 60; // UTC-5 in seconds
	}

	return 0; // Fallback (shouldn't happen)
}

export function UTCSecondstoESTSeconds(utcTimestamp: number): number {
	const dateUTC = new Date(utcTimestamp * 1000);
	const offset = getEasternTimeOffset(dateUTC);
	return utcTimestamp + offset;
}
export function ESTSecondstoUTCSeconds(easternTimestamp: number): number {
	const dateEST = new Date(easternTimestamp * 1000);
	const offset1 = getEasternTimeOffset(dateEST);
	const offset2 = getEasternTimeOffset(new Date((easternTimestamp - offset1) * 1000));
	if (offset1 == offset2 || offset1 < offset2) {
		return easternTimestamp - offset1;
	} else if (offset1 > offset2) {
		return easternTimestamp - offset2;
	}
	return -1;
}
export function ESTSecondstoUTCMillis(easternTimestamp: number): number {
	const dateEST = new Date(easternTimestamp * 1000);
	const offset1 = getEasternTimeOffset(dateEST);
	const offset2 = getEasternTimeOffset(new Date((easternTimestamp - offset1) * 1000));
	if (offset1 == offset2 || offset1 < offset2) {
		return (easternTimestamp - offset1) * 1000;
	} else if (offset1 > offset2) {
		return (easternTimestamp - offset2) * 1000;
	}
	return -1;
}
export function ESTStringToUTCTimestamp(easternString: string): number {
	const formats = ['yyyy-MM-dd H:m:ss', 'yyyy-MM-dd H:m', 'yyyy-MM-dd H', 'yyyy-MM-dd'];
	for (const format of formats) {
		try {
			const easternTime = DateTime.fromFormat(easternString, format, { zone: 'America/New_York' });
			if (easternTime.isValid) {
				const utcTimestamp: number = easternTime.toUTC().toMillis();
				return utcTimestamp;
			}
		} catch (error) {
			console.error(`Error parsing date with format ${format}: `, error);
		}
	}
	return 0;
}
/*const easternTime = DateTime.fromFormat(easternString, 'yyyy-MM-dd HH:mm:ss', {zone: 'America/New_York'})
const utcTimestamp: number = easternTime.toUTC().toMillis();
return utcTimestamp; */

export function UTCTimestampToESTString(utcTimestamp: number, dateOnly = false): string {
	if (utcTimestamp === 0 || utcTimestamp === undefined) {
		return 'Current';
	}
	const utcDatetime = DateTime.fromMillis(utcTimestamp, { zone: 'utc' });
	const easternTime = utcDatetime.setZone('America/New_York');
	if (dateOnly) {
		return easternTime.toFormat('yyyy-MM-dd'); // Date only
	}
	return easternTime.toFormat('yyyy-MM-dd HH:mm:ss');
}
export function ESTTimestampToESTString(utcTimestamp: number, dateOnly = false): string {
	if (utcTimestamp === 0 || utcTimestamp === undefined) {
		return 'Current';
	}
	const utcDatetime = DateTime.fromMillis(utcTimestamp, { zone: 'utc' });
	//const easternTime = utcDatetime.setZone('America/New_York')
	if (dateOnly) {
		return utcDatetime.toFormat('yyyy-MM-dd'); // Date only
	}
	return utcDatetime.toFormat('yyyy-MM-dd HH:mm:ss');
}
export function timeframeToSeconds(timeframe: string): number {
	if (timeframe.includes('s')) {
		return parseInt(timeframe);
	} else if (timeframe.includes('h')) {
		return 3600 * parseInt(timeframe);
	} else if (timeframe.includes('d')) {
		return 86400 * parseInt(timeframe);
	} else if (timeframe.includes('w')) {
		return 604800 * parseInt(timeframe);
	} else if (timeframe.includes('m')) {
		return 2592000 * parseInt(timeframe);
	} else if (!(timeframe.includes('m') || timeframe.includes('q'))) {
		return 60 * parseInt(timeframe);
	}
	return 0;
}
export function getStartOfMonthXMonthsOut(startTimestamp: number, monthsOut: number) {
	const timeZone = 'America/New_York';
	let dt = DateTime.fromMillis(startTimestamp, { zone: timeZone });
	dt = dt.plus({ months: monthsOut });
	dt = dt.startOf('month').startOf('day');
	return dt.toMillis();
}
export function getReferenceStartTimeForDateMilliseconds(
	timestamp: number,
	extendedHours?: boolean
): number {
	const date = new Date(timestamp);

	// Use Intl.DateTimeFormat to determine the offset for America/New_York (Eastern Time)
	const options: Intl.DateTimeFormatOptions = {
		timeZone: 'America/New_York',
		year: 'numeric',
		month: 'numeric',
		day: 'numeric',
		hour: 'numeric',
		minute: 'numeric',
		second: 'numeric',
		hour12: false
	};

	// Get the year, month, day in the correct time zone
	const formatter = new Intl.DateTimeFormat('en-US', options);
	const parts = formatter.formatToParts(date);

	const getPart = (type: string) => parseInt(parts.find((p) => p.type === type)?.value || '0', 10);

	const year = getPart('year');
	const month = getPart('month') - 1; // JavaScript months are 0-indexed
	const day = getPart('day');

	let hours = 9,
		minutes = 30;
	if (extendedHours) {
		hours = 4;
		minutes = 0;
	}

	// Now construct the Date in Eastern Time with the correct offset
	return new Date(Date.UTC(year, month, day, hours, minutes, 0)).getTime();
}
export function isOutsideMarketHours(timestamp: number): boolean {
	// Create a date object from the timestamp
	const date = new Date(timestamp);

	// Check for weekend first (0 = Sunday, 6 = Saturday)
	const day = date.getDay();
	if (day === 0 || day === 6) {
		return true;
	}

	// Determine if daylight saving is in effect for EST/EDT
	const isDST = isDaylightSavingTime(date);

	// Convert to EST or EDT depending on DST
	const timezoneOffset = isDST ? -4 : -5; // EDT is UTC-4, EST is UTC-5
	const estDate = new Date(date.getTime() + timezoneOffset * 60 * 60 * 1000);

	// Get the hours in EST/EDT
	const hours = estDate.getUTCHours(); // Hours in EST or EDT time

	// Define the start and end hours for market time in 24-hour format (EST)
	const startHour = 4; // 4:00 AM
	const endHour = 20; // 8:00 PM

	// Check if the timestamp is outside the market hours
	return hours < startHour || hours >= endHour;
}

function isDaylightSavingTime(date: Date): boolean {
	// Get the current year
	const year = date.getUTCFullYear();

	// Start and end dates of DST in the US for the given year
	const dstStart = new Date(Date.UTC(year, 2, 8, 7)); // 2nd Sunday of March at 2:00 AM (UTC-4)
	const dstEnd = new Date(Date.UTC(year, 10, 1, 6)); // 1st Sunday of November at 2:00 AM (UTC-5)
	// Return whether the date falls within DST
	return date >= dstStart && date < dstEnd;
}
export function getRealTimeTime(): number {
	return Date.now() + systemClockOffset;
}
