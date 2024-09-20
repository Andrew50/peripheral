import {DateTime} from 'luxon';

function getEasternTimeOffset(date : Date)  {
    const options: Intl.DateTimeFormatOptions= { timeZone: 'America/New_York', timeZoneName: 'short' };
    const formatter = new Intl.DateTimeFormat([], options);
    const parts: Intl.DateTimeFormatPart[] = formatter.formatToParts(date);
    // Extract the time zone name (EDT or EST) from the formatted parts
    const timeZoneAbbr = parts.find(part => part.type === 'timeZoneName')?.value;

    if(!timeZoneAbbr) {
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

export function UTCtoEST(utcTimestamp : number): number{
    const dateUTC = new Date(utcTimestamp * 1000);
    const offset = getEasternTimeOffset(dateUTC)
    return utcTimestamp + offset;
}
export function ESTtoUTC(easternTimestamp : number): number {
    const dateEST = new Date(easternTimestamp * 1000) 
    const offset1 = getEasternTimeOffset(dateEST)
    const offset2 = getEasternTimeOffset(new Date((easternTimestamp - offset1)*1000))
    if((offset1 == offset2) || (offset1 < offset2)) {
        return easternTimestamp - offset1
    } else if (offset1 > offset2) {
        return easternTimestamp - offset2 
    } 
    return -1
}
export function ESTSecondstoUTC(easternTimestamp : number): number {
    const dateEST = new Date(easternTimestamp * 1000) 
    const offset1 = getEasternTimeOffset(dateEST)
    const offset2 = getEasternTimeOffset(new Date((easternTimestamp - offset1)*1000))
    if((offset1 == offset2) || (offset1 < offset2)) {
        return (easternTimestamp - offset1)*1000
    } else if (offset1 > offset2) {
        return (easternTimestamp - offset2)*1000
    } 
    return -1
}
export function ESTStringToUTCTimestamp(easternString: string): number {
    const formats = ["yyyy-MM-dd H:m:ss", "yyyy-MM-dd H:m", "yyyy-MM-dd H", "yyyy-MM-dd"];
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

export function UTCTimestampToESTString(utcTimestamp : number): string {
    if (utcTimestamp === 0 || utcTimestamp === undefined){
        return "Current"
    }
    const utcDatetime = DateTime.fromMillis(utcTimestamp, {zone: 'utc'})
    const easternTime = utcDatetime.setZone('America/New_York')
    return easternTime.toFormat('yyyy-MM-dd HH:mm:ss')
}
export function timeframeToSeconds(timeframe : string): number {
    if (timeframe.includes('s')) {
        return parseInt(timeframe)
    }
    else if (!(timeframe.includes('m') || timeframe.includes('w') || 
    timeframe.includes('q') || timeframe.includes('d') || timeframe.includes('h'))) {
        return 60 * parseInt(timeframe)
    } 
    return 0 
}
export function getReferenceStartTimeForDateMilliseconds(timestamp: number, extendedHours?: boolean): number {
    const date = new Date(timestamp);

    // Use Intl.DateTimeFormat to determine the offset for America/New_York (Eastern Time)
    const options = {
        timeZone: 'America/New_York',
        year: 'numeric',
        month: 'numeric',
        day: 'numeric',
        hour: 'numeric',
        minute: 'numeric',
        second: 'numeric',
        hour12: false,
    };

    // Get the year, month, day in the correct time zone
    const formatter = new Intl.DateTimeFormat('en-US', options);
    const parts = formatter.formatToParts(date);

    const getPart = (type: string) => parseInt(parts.find(p => p.type === type)?.value || '0', 10);

    const year = getPart('year');
    const month = getPart('month') - 1; // JavaScript months are 0-indexed
    const day = getPart('day');

    let hours = 9, minutes = 30;
    if (extendedHours) {
        hours = 4;
        minutes = 0;
    }

    // Now construct the Date in Eastern Time with the correct offset
    return new Date(Date.UTC(year, month, day, hours, minutes, 0)).getTime();
}
