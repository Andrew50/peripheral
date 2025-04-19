import { type Instance } from '$lib/core/types';
import { privateRequest } from '$lib/core/backend';
import {writable, get} from "svelte/store"

// Define a type for SEC Filing context items
export interface FilingContext {
	ticker?: string;
	securityId?: string | number;
	filingType: string; // e.g., "10-K", "8-K"
	link: string; // URL to the filing
	timestamp: number; // Keep timestamp for unique key in #each loops
}

export const inputValue = writable("");
// Store for chat context items - can hold Instances or FilingContexts
export const contextItems = writable<(Instance | FilingContext)[]>([]);

export async function addInstanceToChat(instance: Instance) {
	// Add instance to chat context (avoid duplicates based on securityId and timestamp)
	contextItems.update(items => {
		const exists = items.some(i =>
			i.securityId === instance.securityId &&
			i.timestamp === instance.timestamp
		);
		return exists ? items : [...items, instance];
	});
}

// Remove an instance from chat context
export function removeInstanceFromChat(instance: Instance) {
	contextItems.update(items =>
		items.filter(i =>
			!(i.securityId === instance.securityId && i.timestamp === instance.timestamp)
		)
	);
}

// Add a filing to chat context
export function addFilingToChatContext(filing: FilingContext) {
	contextItems.update(items => {
		// Avoid duplicates based on securityId and link
		const exists = items.some(item =>
			'link' in item && // Check if it's a FilingContext
			item.securityId === filing.securityId &&
			item.link === filing.link
		);
		return exists ? items : [...items, filing];
	});
}

// Remove a filing from chat context
export function removeFilingFromChat(filing: FilingContext) {
	contextItems.update(items =>
		items.filter(item =>
			!('link' in item && // Check if it's a FilingContext
				item.securityId === filing.securityId &&
				item.link === filing.link)
		)
	);
}

