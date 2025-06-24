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
export type FunctionStatusUpdate = {
	type: 'function_status';
	userMessage: string;
};

// Define the type for title updates from backend
export type TitleUpdate = {
	type: 'title_update';
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
export const functionStatusStore = writable<FunctionStatusUpdate | null>(null);

// Store to hold the latest title update
export const titleUpdateStore = writable<TitleUpdate | null>(null);



// Store to manage pending chat requests
const pendingChatRequests = new Map<string, {
	resolve: (value: any) => void;
	reject: (error: Error) => void;
}>();

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
	if( get(isPublicViewing))return;
	
	try {
		const token = sessionStorage.getItem('authToken');
		const socketUrl = base_url + '/ws' + '?token=' + token;
		socket = new WebSocket(socketUrl);
	} catch (e) {
		console.error(e);
		setTimeout(connect, 1000);
		return;
	}
	socket.addEventListener('close', () => {
		connectionStatus.set('disconnected');
		
		// Reject all pending chat requests
		pendingChatRequests.forEach((request, requestId) => {
			request.reject(new Error('WebSocket connection closed'));
		});
		pendingChatRequests.clear();
		
		if (shouldReconnect) {
			reconnect();
		}
	});
	socket.addEventListener('open', () => {
		connectionStatus.set('connected');
		reconnectAttempts = 0;
		reconnectInterval = 5000;


		// Resubscribe to all active channels and pending subscriptions
		const allChannels = new Set([...activeChannels.keys(), ...pendingSubscriptions]);
		for (const channelName of allChannels) {
			subscribe(channelName);
		}
		pendingSubscriptions.clear();
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
		if (data && data.type === 'function_status') {
			const statusUpdate = data as FunctionStatusUpdate;
			functionStatusStore.set(statusUpdate);
			return; // Handled function status update
		}

		// Handle title updates
		if (data && data.type === 'title_update') {
			const titleUpdate = data as TitleUpdate;
			titleUpdateStore.set(titleUpdate);
			return; // Handled title update
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
					pendingRequest.reject(new Error(chatResponse.error || 'Chat request failed'));
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
				if (channelName.includes('-slow-regular') && data.price !== undefined) {
					const securityId = parseInt(channelName.split('-')[0]);
					if (!isNaN(securityId)) {
						enqueueTick({
							securityid: securityId,
							price: data.price,
							data: data
						});
					}
				}
				
				// Handle close data for the hub
				if (channelName.includes('-close-regular') && data.price !== undefined) {
					const securityId = parseInt(channelName.split('-')[0]);
					if (!isNaN(securityId)) {
						enqueueTick({
							securityid: securityId,
							prevClose: data.price,
							data: data
						});
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
		socket.send(JSON.stringify({
			action: 'subscribe-sec-filings'
		}));
	} else {
		// Store the subscription request to be sent when connection is established
		pendingSubscriptions.add('sec-filings');
	}
}

export function unsubscribeSECFilings() {
	if (socket?.readyState === WebSocket.OPEN) {
		socket.send(JSON.stringify({
			action: 'unsubscribe-sec-filings'
		}));
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
		if (socket?.readyState !== WebSocket.OPEN) {
			reject(new Error('WebSocket is not connected'));
			return;
		}

		// Generate unique request ID
		requestId = `chat_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

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
			socket.send(JSON.stringify(chatQuery));
		} catch (error) {
			// Clean up on send failure
			pendingChatRequests.delete(requestId);
			reject(error);
		}
	});

	const cancel = () => {
		if (requestId) {
			cancelChatQuery(requestId);
		}
	};

	return { promise, cancel };
}

// Cancel a chat query by request ID
export function cancelChatQuery(requestId: string) {
	const pendingRequest = pendingChatRequests.get(requestId);
	if (pendingRequest) {
		pendingChatRequests.delete(requestId);
		pendingRequest.reject(new Error('Chat request cancelled'));
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
