import { writable } from 'svelte/store';
import { DateTime } from 'luxon';

// Define the Filing type to match the backend structure
export interface Filing {
  company_name: string;
  type: string;
  date: string;
  url: string;
  accession_number: string;
  description?: string;
  ticker: string;
  timestamp: number;
}

// Create a store for the filings
export const globalFilings = writable<Filing[]>([]);

// Format timestamp to a readable date/time string
export function formatTimestamp(timestamp: number): string {
  if (!timestamp) return 'Unknown date';

  console.log('Formatting timestamp:', timestamp, typeof timestamp);

  try {
    const dt = DateTime.fromMillis(timestamp).setZone('America/New_York');
    return dt.toFormat('MMM dd, yyyy h:mm a ZZZZ');
  } catch (e) {
    console.error('Error formatting timestamp:', e, 'for value:', timestamp);
    return 'Invalid date';
  }
}

// Handle incoming SEC filing messages from WebSocket
export function handleSECFilingMessage(message: any): void {
  console.log('Received SEC filing message:', JSON.stringify(message, null, 2));

  // If the message is already an array, use it directly
  if (Array.isArray(message)) {
    console.log(`Processing ${message.length} SEC filings from array`);

    // Debug the first filing's timestamp
    if (message.length > 0) {
      console.log('First filing timestamp:', message[0].timestamp, typeof message[0].timestamp);
    }

    globalFilings.set(message);
    return;
  }

  // Check if the message has the expected structure with channel and data
  if (message && message.channel === 'sec-filings' && message.data) {
    // If data is an array, it's the initial load or a batch update
    if (Array.isArray(message.data)) {
      console.log(`Processing ${message.data.length} SEC filings from channel data`);

      // Debug the first filing's timestamp
      if (message.data.length > 0) {
        console.log('First filing timestamp:', message.data[0].timestamp, typeof message.data[0].timestamp);
      }

      // Ensure all filings have a timestamp (use current time as fallback)
      const processedFilings = message.data.map((filing: Filing) => {
        if (!filing.timestamp) {
          console.warn('Filing missing timestamp:', filing);
          return {
            ...filing,
            timestamp: Date.now()
          };
        }
        return filing;
      });

      globalFilings.set(processedFilings);
    }
    // If it's a single filing, add it to the beginning of the list
    else {
      console.log('Processing single filing:', message.data);

      // Ensure the filing has a timestamp
      const filing = message.data;
      if (!filing.timestamp) {
        console.warn('Single filing missing timestamp:', filing);
        filing.timestamp = Date.now();
      }

      globalFilings.update(filings => {
        // Add the new filing at the beginning
        return [filing, ...filings];
      });
    }
  } else {
    console.warn('Received unexpected SEC filing message format:', message);

    // Try to handle the message as a direct filing object
    if (message && message.type && message.url) {
      console.log('Treating message as direct filing object');

      // Ensure the filing has a timestamp
      if (!message.timestamp) {
        console.warn('Direct filing missing timestamp:', message);
        message.timestamp = Date.now();
      }

      globalFilings.update(filings => [message, ...filings]);
    }
  }
} 