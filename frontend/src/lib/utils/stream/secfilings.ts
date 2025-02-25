import { writable } from 'svelte/store';
import type { StreamData } from './socket';
import { UTCTimestampToESTString } from '$lib/core/timestamp';

// Store for SEC filings
export const globalFilings = writable<Filing[]>([]);

// Interface for SEC filings
export interface Filing {
    type: string;
    date: string;
    url: string;
    timestamp: number;
    ticker: string;
    channel?: string;
}

// Function to handle incoming global SEC filing messages
export function handleGlobalSECFilingMessage(data: StreamData) {
    console.log("SEC Filing message received:", data);

    // If data is an array, it's the initial load
    if (Array.isArray(data)) {
        console.log("Initial SEC filings data:", data);
        globalFilings.set(data as Filing[]);
    } else {
        console.log("New SEC filing:", data);
        globalFilings.update(currentFilings => {
            // Add the new filing at the beginning of the array
            const updatedFilings = [data as unknown as Filing, ...currentFilings];
            // Keep only the most recent 100 filings
            if (updatedFilings.length > 100) {
                return updatedFilings.slice(0, 100);
            }
            return updatedFilings;
        });
    }
}

// Format timestamp to readable date/time
export function formatTimestamp(timestamp: number): string {
    return UTCTimestampToESTString(timestamp);
} 