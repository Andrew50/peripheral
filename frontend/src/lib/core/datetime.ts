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
export function ESTStringToUTCTimestamp(easternString : string): number{
    const easternTime = DateTime.fromFormat(easternString, 'yyyy-MM-dd HH:mm:ss', {zone: 'America/New_York'})

    const utcTimestamp: number = easternTime.toUTC().toMillis();

    return utcTimestamp; 
}
export function UTCTimestampToESTString(utcTimestamp : number): string {
    const utcDatetime = DateTime.fromMillis(utcTimestamp, {zone: 'utc'})
    const easternTime = utcDatetime.setZone('America/New_York')
    return easternTime.toFormat('yyyy-MM-dd HH:mm:ss')
}