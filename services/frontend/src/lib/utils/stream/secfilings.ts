import { writable } from 'svelte/store';

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

// Format timestamp to a consistent format
function formatTimestamp(timestamp: number | string): number {
	try {
		// Remove console.log for formatting timestamp

		// If timestamp is already a number, return it
		if (typeof timestamp === 'number') {
			return timestamp;
		}

		// If timestamp is a string that can be parsed as a number, convert it
		if (!isNaN(Number(timestamp))) {
			return Number(timestamp);
		}

		// Otherwise, try to parse it as a date string
		return new Date(timestamp).getTime();
	} catch (e) {
		console.error('Error formatting timestamp:', e, 'for value:', timestamp);
		return 0;
	}
}

// Handle incoming SEC filing messages from WebSocket
export function handleSECFilingMessage(message: Filing | { channel: string; data: Filing | Filing[] }): void {
	// Remove console.log for received SEC filing message

	// If the message is already an array, use it directly
	if (Array.isArray(message)) {
		// Remove console.log for processing SEC filings from array

		// Remove console.log for first filing's timestamp

		globalFilings.set(message);
		return;
	}

	// Check if the message has the expected structure with channel and data
	if (message && typeof message === 'object' && 'channel' in message && message.channel === 'sec-filings' && message.data) {
		// If data is an array, it's the initial load or a batch update
		if (Array.isArray(message.data)) {
			// Remove console.log for processing SEC filings from channel data

			// Remove console.log for first filing's timestamp

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
			// Remove console.log for processing single filing

			// Ensure the filing has a timestamp
			const filing = message.data;
			if (!filing.timestamp) {
				console.warn('Single filing missing timestamp:', filing);
				filing.timestamp = Date.now();
			}

			globalFilings.update((filings: Filing[]) => {
				// Add the new filing at the beginning
				return [filing, ...filings];
			});
		}
	} else {
		console.warn('Received unexpected SEC filing message format:', message);

		// Try to handle the message as a direct filing object
		if (message && typeof message === 'object' && 'type' in message && 'url' in message) {
			// Remove console.log for treating message as direct filing object

			// Ensure the filing has a timestamp
			if (!message.timestamp) {
				console.warn('Direct filing missing timestamp:', message);
				message.timestamp = Date.now();
			}

			globalFilings.update((filings: Filing[]) => [message as Filing, ...filings]);
		}
	}
}

// Process SEC filings data from the socket
export function processSECFilingsMessage(message: Filing | Filing[] | { channel: string; data: Filing | Filing[] }, callback: (filings: Filing[]) => void): void {
	try {
		// Remove console.log for received SEC filing message

		// Handle array of filings
		if (Array.isArray(message)) {
			// Remove console.log for processing SEC filings from array

			const formattedFilings = message.map((filing) => {
				// Remove console.log for first filing timestamp

				return {
					...filing,
					timestamp: formatTimestamp(filing.timestamp)
				};
			});

			callback(formattedFilings);
			return;
		}

		// Handle channel data format
		if (typeof message === 'object' && message && 'channel' in message && message.channel === 'secfilings' && Array.isArray(message.data)) {
			// Remove console.log for processing SEC filings from channel data

			const formattedFilings = message.data.map((filing: Filing) => {
				// Remove console.log for first filing timestamp

				if (!filing.timestamp) {
					console.warn('Filing missing timestamp:', filing);
					filing.timestamp = Date.now();
				}

				return {
					...filing,
					timestamp: formatTimestamp(filing.timestamp)
				};
			});

			callback(formattedFilings);
			return;
		}

		// Handle single filing in channel data
		if (
			typeof message === 'object' && message && 'channel' in message &&
			message.channel === 'secfilings' &&
			typeof message.data === 'object' &&
			message.data !== null
		) {
			// Remove console.log for processing single filing

			const filing = message.data;

			if (!filing.timestamp) {
				console.warn('Single filing missing timestamp:', filing);
				filing.timestamp = Date.now();
			}

			callback([
				{
					...filing,
					timestamp: formatTimestamp(filing.timestamp)
				}
			]);
			return;
		}

		// Handle unexpected format
		console.warn('Received unexpected SEC filing message format:', message);

		// Try to handle it as a direct filing object
		// Remove console.log for treating message as direct filing object

		if (typeof message === 'object' && message && !Array.isArray(message) && 'timestamp' in message) {
			const filing = message as Filing;
			if (!filing.timestamp) {
				console.warn('Direct filing missing timestamp:', filing);
				filing.timestamp = Date.now();
			}

			callback([
				{
					...filing,
					timestamp: formatTimestamp(filing.timestamp)
				}
			]);
		}
	} catch (error) {
		console.error('Error processing SEC filings message:', error);
	}
}
