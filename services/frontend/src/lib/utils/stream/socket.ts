// socket.ts
import { get, writable } from 'svelte/store';
import { streamInfo, handleTimestampUpdate } from '$lib/utils/stores/stores';
import type { StreamInfo, TradeData, QuoteData, CloseData } from '$lib/utils/types/types';
import { base_url } from '$lib/utils/helpers/backend';
import { browser } from '$app/environment';
import { handleAlert } from './alert';
import type { AlertData } from '$lib/utils/types/types';
import { enqueueTick } from './streamHub';

// Define the type for function status updates from backend (simplified)
export type AgentStatusUpdate = {
	type: 'AgentStatusUpdate';
	userMessage: string;
};

// Define the type for title updates from backend
export type TitleUpdate = {
	type: 'titleUpdate';
	conversation_id: string;
	title: string;
};

// Define the type for chat responses from backend
export type ChatResponse = {
	type: 'chat_response';
	request_id: string;
	success: boolean;
	data?: any;
	error?: string;
};

// Store to hold the current function status message
export const agentStatusStore = writable<AgentStatusUpdate | null>(null);

// Store to hold the latest title update
export const titleUpdateStore = writable<TitleUpdate | null>(null);

// Callback for handling message ID updates (set by chat component)
let messageIdUpdateCallback: ((messageId: string, conversationId: string) => void) | null = null;

export function setMessageIdUpdateCallback(callback: ((messageId: string, conversationId: string) => void) | null) {
	messageIdUpdateCallback = callback;
}

// Store to manage pending chat requests
const pendingChatRequests = new Map<
	string,
	{
		resolve: (value: any) => void;
		reject: (error: Error) => void;
	}
>();

// Store for single pending chat request while disconnected
let pendingChatRequest: {
	requestId: string;
	resolve: (value: any) => void;
	reject: (error: Error) => void;
	query: string;
	context: any[];
	activeChartContext: any;
	conversationId: string;
	timeoutId: any;
} | null = null;

// Chat request timeout duration (30 seconds)
const CHAT_REQUEST_TIMEOUT = 30000;

// Track if we're currently attempting to connect (to prevent multiple simultaneous attempts)
let isConnecting = false;

export type TimeType = 'regular' | 'extended';
export type ChannelType = //"fast" | "slow" | "quote" | "close" | "all"

		| 'fast-regular'
		| 'fast-extended'
		| 'slow-regular'
		| 'slow-extended'
		| 'close-regular'
		| 'close-extended'
		| 'quote'
		| 'all'; //all trades

export type StreamData = TradeData | QuoteData | CloseData | number;
export type StreamCallback = (v: TradeData | QuoteData | CloseData | number) => void;

export const activeChannels: Map<string, StreamCallback[]> = new Map();
export const connectionStatus = writable<'connected' | 'disconnected' | 'connecting'>('connecting');
export const pendingSubscriptions = new Set<string>();

export type SubscriptionRequest = {
	action: 'subscribe' | 'unsubscribe' | 'replay' | 'pause' | 'play' | 'realtime' | 'speed';
	channelName?: string;
	timestamp?: number;
	speed?: number;
	extendedHours?: boolean;
};

export let socket: WebSocket | null = null;
let reconnectInterval: number = 5000; //ms
const maxReconnectInterval: number = 30000;
let reconnectAttempts: number = 0;
const maxReconnectAttempts: number = 5;
let shouldReconnect: boolean = true;

export const latestValue = new Map<string, StreamData>();
import { isPublicViewing } from '$lib/utils/stores/stores';

export function connect() {
	if (!browser) return;
	if (get(isPublicViewing)) return;

	isConnecting = true;
	connectionStatus.set('connecting');

	try {
		const token = sessionStorage.getItem('authToken');
		const socketUrl = base_url + '/ws' + '?token=' + token;
		socket = new WebSocket(socketUrl);
	} catch (e) {
		console.error(e);
		isConnecting = false;
		setTimeout(connect, 1000);
		return;
	}
	socket.addEventListener('close', () => {
		connectionStatus.set('disconnected');
		isConnecting = false;

		// Reject all pending chat requests
		pendingChatRequests.forEach((request, requestId) => {
			request.reject(new Error('WebSocket connection closed'));
		});
		pendingChatRequests.clear();

		// Reject pending chat request
		if (pendingChatRequest) {
			clearTimeout(pendingChatRequest.timeoutId);
			pendingChatRequest.reject(new Error('WebSocket connection closed'));
			pendingChatRequest = null;
		}

		if (shouldReconnect) {
			reconnect();
		}
	});
	socket.addEventListener('open', () => {
		connectionStatus.set('connected');
		isConnecting = false;
		reconnectAttempts = 0;
		reconnectInterval = 5000;

		// Resubscribe to all active channels and pending subscriptions
		const allChannels = new Set([...activeChannels.keys(), ...pendingSubscriptions]);
		for (const channelName of allChannels) {
			subscribe(channelName);
		}
		pendingSubscriptions.clear();

		// Process pending chat request
		processPendingChatRequest();
	});
	socket.addEventListener('message', (event) => {
		let data;
		try {
			data = JSON.parse(event.data);
		} catch {
			console.warn('Failed to parse WebSocket message:', event.data);
			return;
		}

		// Check message type first
		if (data && data.type === 'AgentStatusUpdate') {
			const statusUpdate = data as AgentStatusUpdate;
			agentStatusStore.set(statusUpdate);
			return; // Handled agent status update
		}

		// Handle title updates
		if (data && data.type === 'titleUpdate') {
			const titleUpdate = data as TitleUpdate;
			titleUpdateStore.set(titleUpdate);
			return; // Handled title update
		}

		// Handle chat initialization updates
		if (data && data.type === 'ChatInitializationUpdate') {
			if (messageIdUpdateCallback && data.message_id && data.conversation_id) {
				messageIdUpdateCallback(data.message_id, data.conversation_id);
			}
			return; // Handled chat initialization update
		}

		// Handle chat responses
		if (data && data.type === 'chat_response') {
			const chatResponse = data as ChatResponse;
			const pendingRequest = pendingChatRequests.get(chatResponse.request_id);

			if (pendingRequest) {
				pendingChatRequests.delete(chatResponse.request_id);

				if (chatResponse.success) {
					pendingRequest.resolve(chatResponse.data);
				} else {
					// Create error with response data so frontend can extract messageID and conversationID
					const error = new Error(chatResponse.error || 'Chat request failed') as any;
					error.response = chatResponse.data; // Attach response data to error
					pendingRequest.reject(error);
				}
			}
			return; // Handled chat response
		}

		// Handle other message types (based on channel)
		const channelName = data.channel;
		if (channelName) {
			if (channelName === 'alert') {
				handleAlert(data as AlertData);
			} else if (channelName === 'timestamp') {
				handleTimestampUpdate(data.timestamp);
			} else {
				// Also feed data to the new streamHub system
				if (
					(channelName.includes('-slow-regular') || channelName.includes('-slow-extended')) &&
					data.price !== undefined
				) {
					const securityId = parseInt(channelName.split('-')[0]);
					if (!isNaN(securityId)) {
						const tickData: any = {
							securityid: securityId,
							price: data.price,
							data: data
						};

						// If this is extended hours data, mark it for extended calculation
						if (channelName.includes('-slow-extended')) {
							tickData.isExtended = true;
						}

						enqueueTick(tickData);
					}
				}

				// Handle close data for the hub (both regular and extended)
				if (
					(channelName.includes('-close-regular') || channelName.includes('-close-extended')) &&
					data.price !== undefined
				) {
					const securityId = parseInt(channelName.split('-')[0]);
					if (!isNaN(securityId)) {
						const tickData: any = {
							securityid: securityId,
							data: data
						};

						// Set appropriate reference price field based on channel type
						if (channelName.includes('-close-regular')) {
							tickData.prevClose = data.price;
						} else if (channelName.includes('-close-extended')) {
							tickData.extendedClose = data.price;
						}

						enqueueTick(tickData);
					}
				}
				latestValue.set(channelName, data);
				const callbacks = activeChannels.get(channelName);
				if (callbacks) {
					callbacks.forEach((callback) => callback(data));
				}
			}
		}
	});
	socket.addEventListener('error', () => {
		socket?.close();
	});
}

function disconnect() {
	shouldReconnect = false;
	connectionStatus.set('disconnected');

	if (socket) {
		activeChannels.forEach((_, channelName) => {
			unsubscribe(channelName);
		});
		socket.close();
		socket = null;
	}
}

function reconnect() {
	if (reconnectAttempts < maxReconnectAttempts) {
		reconnectAttempts++;
		const reconnectDelay = Math.min(reconnectInterval * reconnectAttempts, maxReconnectInterval);
		setTimeout(connect, reconnectDelay);
	}
}

export function subscribe(channelName: string) {
	if (socket?.readyState === WebSocket.OPEN) {
		const subscriptionRequest: SubscriptionRequest = {
			action: 'subscribe',
			channelName: channelName
		};
		socket.send(JSON.stringify(subscriptionRequest));
	} else {
		// Store the subscription request to be sent when connection is established
		pendingSubscriptions.add(channelName);
	}
}

export function unsubscribe(channelName: string) {
	if (socket?.readyState === WebSocket.OPEN) {
		const unsubscriptionRequest: SubscriptionRequest = {
			action: 'unsubscribe',
			channelName: channelName
		};
		socket.send(JSON.stringify(unsubscriptionRequest));
	}
}

export function subscribeSECFilings() {
	if (socket?.readyState === WebSocket.OPEN) {
		socket.send(
			JSON.stringify({
				action: 'subscribe-sec-filings'
			})
		);
	} else {
		// Store the subscription request to be sent when connection is established
		pendingSubscriptions.add('sec-filings');
	}
}

export function unsubscribeSECFilings() {
	if (socket?.readyState === WebSocket.OPEN) {
		socket.send(
			JSON.stringify({
				action: 'unsubscribe-sec-filings'
			})
		);
	}

	// Remove from pending subscriptions if present
	pendingSubscriptions.delete('sec-filings');
}

// Send chat query via WebSocket
export function sendChatQuery(
	query: string,
	context: any[] = [],
	activeChartContext: any = null,
	conversationId: string = ''
): { promise: Promise<any>; cancel: () => void } {
	let requestId: string;

	const promise = new Promise((resolve, reject) => {
		// Generate unique request ID
		requestId = `chat_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

		if (socket?.readyState === WebSocket.OPEN) {
			// Connection is open, send immediately
			sendChatQueryNow(
				requestId,
				query,
				context,
				activeChartContext,
				conversationId,
				resolve,
				reject
			);
		} else {
			// Cancel any existing pending chat request
			if (pendingChatRequest) {
				clearTimeout(pendingChatRequest.timeoutId);
				pendingChatRequest.reject(new Error('Chat request cancelled - new request initiated'));
			}

			// Connection is not open, store the request and attempt immediate reconnection
			const timeoutId = setTimeout(() => {
				if (pendingChatRequest?.requestId === requestId) {
					pendingChatRequest = null;
					reject(new Error('Chat request timeout - could not establish connection'));
				}
			}, CHAT_REQUEST_TIMEOUT);

			pendingChatRequest = {
				requestId,
				resolve,
				reject,
				query,
				context,
				activeChartContext,
				conversationId,
				timeoutId
			};

			// Immediately attempt to reconnect if not already connecting
			if (!isConnecting && shouldReconnect) {
				// Reset reconnection attempts for user-initiated requests
				reconnectAttempts = 0;
				connect();
			}
		}
	});

	const cancel = () => {
		if (requestId) {
			cancelChatQuery(requestId);
		}
	};

	return { promise, cancel };
}

// Helper function to send chat query immediately
function sendChatQueryNow(
	requestId: string,
	query: string,
	context: any[],
	activeChartContext: any,
	conversationId: string,
	resolve: (value: any) => void,
	reject: (error: Error) => void
) {
	// Store the promise resolvers
	pendingChatRequests.set(requestId, { resolve, reject });

	// Create the chat query message
	const chatQuery = {
		action: 'chat_query',
		request_id: requestId,
		query: query,
		context: context,
		activeChartContext: activeChartContext,
		conversation_id: conversationId
	};

	try {
		socket?.send(JSON.stringify(chatQuery));
	} catch (error) {
		// Clean up on send failure
		pendingChatRequests.delete(requestId);
		reject(error instanceof Error ? error : new Error(String(error)));
	}
}

// Process pending chat request when connection is restored
function processPendingChatRequest() {
	if (!pendingChatRequest) return;

	// Clear the timeout since we're processing now
	clearTimeout(pendingChatRequest.timeoutId);

	// Send the pending request
	sendChatQueryNow(
		pendingChatRequest.requestId,
		pendingChatRequest.query,
		pendingChatRequest.context,
		pendingChatRequest.activeChartContext,
		pendingChatRequest.conversationId,
		pendingChatRequest.resolve,
		pendingChatRequest.reject
	);

	// Clear the pending request
	pendingChatRequest = null;
}

// Cancel a chat query by request ID
export function cancelChatQuery(requestId: string) {
	// Check active requests
	const activePendingRequest = pendingChatRequests.get(requestId);
	if (activePendingRequest) {
		pendingChatRequests.delete(requestId);
		activePendingRequest.reject(new Error('Chat request cancelled'));
		return;
	}

	// Check pending chat request
	if (pendingChatRequest?.requestId === requestId) {
		clearTimeout(pendingChatRequest.timeoutId);
		pendingChatRequest.reject(new Error('Chat request cancelled'));
		pendingChatRequest = null;
	}
}

// Add browser window close handler
if (browser) {
	window.addEventListener('beforeunload', () => {
		// Unsubscribe from all channels
		activeChannels.forEach((_, channelName) => {
			if (channelName === 'sec-filings') {
				unsubscribeSECFilings();
			} else {
				unsubscribe(channelName);
			}
		});

		// Close the socket
		if (socket && socket.readyState === WebSocket.OPEN) {
			// Set flag to prevent automatic reconnection
			shouldReconnect = false;
			socket.close();
		}
	});
}
