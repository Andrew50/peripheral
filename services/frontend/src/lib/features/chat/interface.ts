import { type Instance } from '$lib/utils/types/types';
import { writable, get, type Writable } from 'svelte/store';

// Define a type for SEC Filing context items
export interface FilingContext {
	ticker?: string;
	securityId?: string | number;
	filingType: string; // e.g., "10-K", "8-K"
	link: string; // URL to the filing
	timestamp: number; // Keep timestamp for unique key in #each loops
}

export const inputValue: Writable<string> = writable('');
// Store for chat context items - can hold Instances or FilingContexts
export const contextItems: Writable<(Instance | FilingContext)[]> = writable<
	(Instance | FilingContext)[]
>([]);

export function addInstanceToChat(instance: Instance) {
	// Add instance to chat context (avoid duplicates based on securityId and timestamp)
	contextItems.update((items) => {
		const exists = items.some(
			(i) => i.securityId === instance.securityId && i.timestamp === instance.timestamp
		);
		return exists ? items : [...items, instance];
	});
}

// Remove an instance from chat context
export function removeInstanceFromChat(instance: Instance) {
	contextItems.update((items) =>
		items.filter(
			(i) => !(i.securityId === instance.securityId && i.timestamp === instance.timestamp)
		)
	);
}

// Add a filing to chat context
export function addFilingToChatContext(filing: FilingContext) {
	contextItems.update((items) => {
		// Avoid duplicates based on securityId and link
		const exists = items.some(
			(item) =>
				'link' in item && // Check if it's a FilingContext
				item.securityId === filing.securityId &&
				item.link === filing.link
		);
		return exists ? items : [...items, filing];
	});
}

// Remove a filing from chat context
export function removeFilingFromChat(filing: FilingContext) {
	contextItems.update((items) =>
		items.filter(
			(item) =>
				!(
					'link' in item && // Check if it's a FilingContext
					item.securityId === filing.securityId &&
					item.link === filing.link
				)
		)
	);
}

/** Signals +page.svelte to open the chat pane */
export const requestChatOpen = writable(false);

/** Holds the context and query to be processed by the chat component upon opening */
export const pendingChatQuery = writable<{
	context: (Instance | FilingContext)[];
	query: string;
} | null>(null);

export function openChatAndQuery(context: FilingContext | Instance, query: string) {
	pendingChatQuery.set({ context: [context], query }); // Context is always an array
	requestChatOpen.set(true); // Signal the page to open the chat
}

// Define the ContentChunk and TableData types to match the backend
export type TableData = {
	caption?: string;
	headers: string[];
	rows: any[][];
	// Pagination state
	currentPage?: number;
	rowsPerPage?: number;
	totalRows?: number;
	totalPages?: number;
};

export type PlotData = {
	chart_type: 'line' | 'bar' | 'scatter' | 'histogram' | 'heatmap';
	data: PlotTrace[];
	title?: string;
	layout?: PlotLayout;
	titleIcon?: string;
};

export type PlotTrace = {
	x?: any[];
	y?: any[];
	z?: any[];
	name?: string;
	type?: string;
	mode?: string;
	marker?: any;
	line?: any;
	[key: string]: any; // Allow additional Plotly trace properties
};

export type PlotLayout = {
	title?: string;
	xaxis?: PlotAxis;
	yaxis?: PlotAxis;
	width?: number;
	height?: number;
	[key: string]: any; // Allow additional Plotly layout properties
};

export type PlotAxis = {
	title?: string;
	[key: string]: any; // Allow additional Plotly axis properties
};

export type ContentChunk = {
	type: 'text' | 'table' | 'plot';
	content: string | TableData | PlotData;
};

export type QueryResponse = {
	text?: string;
	content_chunks?: ContentChunk[];
	suggestions?: string[];
	conversation_id?: string;
	message_id?: string;
	timestamp?: string | Date;
	completed_at?: string | Date;
};

// Conversation history type
export type ConversationData = {
	conversation_id?: string; // Active conversation ID
	title?: string; // Conversation title
	messages: Array<{
		message_id?: string; // Backend message ID (UUID)
		query: string;
		content_chunks?: ContentChunk[];
		response_text: string;
		timestamp: string | Date;
		context_items?: (Instance | FilingContext)[];
		suggested_queries?: string[];
		completed_at?: string | Date;
		status?: string;
	}>;
	timestamp: string | Date;
};

// Timeline event that can handle different types of events
export type TimelineEvent = {
	headline: string;
	timestamp: Date;
	type?: 'FunctionUpdate' | 'webSearchQuery' | 'webSearchCitations' | 'getWatchlistItems'; // Type of event
	data?: any; // For web search queries or other structured data
};

// Message type for chat history
export type Message = {
	message_id: string; // Use backend message_id directly
	content: string;
	sender: 'user' | 'assistant' | 'system';
	timestamp: Date;
	contentChunks?: ContentChunk[];
	isLoading?: boolean;
	suggestedQueries?: string[];
	contextItems?: (Instance | FilingContext)[];
	status?: string; // "pending", "completed", "error"
	completedAt?: Date; // When the response was completed
	isNewResponse?: boolean; // Flag to indicate this is a new response since last seen
};

// Type for suggested queries response
export type SuggestedQueriesResponse = {
	suggestions: string[];
};
// Conversation management types
export type ConversationInfo = {
	conversation_id: string;
	title: string;
	created_at: string;
	updated_at: string;
	last_message_query?: string;
	is_public: boolean;
};
// State for table sorting
export type SortState = {
	columnIndex: number | null;
	direction: 'asc' | 'desc' | null;
};
