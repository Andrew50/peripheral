// socket.ts
import { get, writable } from 'svelte/store';
import { handleTimestampUpdate } from '$lib/utils/stores/stores';
import type { TradeData, QuoteData, CloseData, Alert, Watchlist } from '$lib/utils/types/types';
import type { HorizontalLine } from '$lib/utils/stores/stores';
import { base_url } from '$lib/utils/helpers/backend';
import { browser } from '$app/environment';
import { handleAlert } from './alert';
import type { AlertData } from '$lib/utils/types/types';
import { enqueueTick } from './streamHub';

// Type definitions for dynamic updates - moved to top
export type WatchlistUpdate = {
	type: 'watchlist_update';
	action: 'add' | 'remove' | 'update' | 'create' | 'delete';
	watchlistId?: number;
	watchlistName?: string;
	item?: {
		watchlistItemId: number;
		securityId: number;
		ticker: string;
		[key: string]: unknown;
	};
	itemId?: number;
};

export type HorizontalLineUpdate = {
	type: 'horizontal_line_update';
	action: 'add' | 'remove' | 'update';
	securityId: number;
	line: {
		id: number;
		securityId: number;
		price: number;
		color: string;
		lineWidth: number;
	};
};

export type AlertUpdate = {
	type: 'alert_update';
	action: 'add' | 'remove' | 'update' | 'trigger';
	alert: {
		alertId: number;
		alertType: string;
		alertPrice?: number;
		securityId?: number;
		ticker?: string;
		active: boolean;
		direction?: boolean;
		triggeredTimestamp?: number;
	};
};

export type StrategyUpdate = {
	type: 'strategy_update';
	action: 'add' | 'remove' | 'update';
	strategy: {
		strategyId: number;
		name: string;
		activeScreen?: boolean;
		[key: string]: unknown;
	};
};

export type AgentStatusUpdate = {
	messageType: 'AgentStatusUpdate';
	headline: string;
	type: string; // e.g., 'FunctionUpdate', 'WebSearchQuery'
	data: unknown; // The actual data - string for FunctionUpdate, object for WebSearchQuery
};

export type TitleUpdate = {
	type: 'titleUpdate';
	conversation_id: string;
	title: string;
};

export type ChatResponse = {
	type: 'chat_response';
	request_id: string;
	success: boolean;
	data?: unknown;
	error?: string;
};

// NEW: Import stores for dynamic updates
interface WatchlistItem {
	watchlistItemId: number;
	securityId: number;
	ticker: string;
	[key: string]: unknown;
}

// Local interface for strategy objects used in socket updates
// This is different from the database Strategy interface in types.ts
interface SocketStrategy {
	strategyId: number;
	name: string;
	activeScreen?: boolean;
	[key: string]: unknown;
}

interface StoreModule {
	watchlists: any;
	currentWatchlistId: any;
	currentWatchlistItems: any;
	flagWatchlist: any;
	flagWatchlistId: number | undefined;
	horizontalLines: any;
	activeAlerts: any;
	inactiveAlerts: any;
	strategies: any;
}

let storesInitialized = false;
let storeModule: StoreModule | null = null;
let initializationPromise: Promise<void> | null = null;

// Initialize store references when needed - improved with retry logic
async function initializeStoreReferences(): Promise<void> {
	if (!browser) return;

	// Return existing promise if initialization is already in progress
	if (initializationPromise) {
		return initializationPromise;
	}

	// If already initialized, return immediately
	if (storesInitialized && storeModule) {
		return Promise.resolve();
	}

	initializationPromise = (async () => {
		try {
			storeModule = await import('$lib/utils/stores/stores');

			// Verify that essential stores are available
			if (!storeModule?.watchlists || !storeModule?.currentWatchlistId) {
				throw new Error('Essential watchlist stores not available');
			}

			storesInitialized = true;
			console.log('✅ Store module initialized successfully for socket updates');
		} catch (error) {
			console.warn('❌ Failed to initialize store references:', error);
			storesInitialized = false;
			storeModule = null;
			// Reset promise so it can be retried
			initializationPromise = null;
			throw error;
		}
	})();

	return initializationPromise;
}

// NEW: Frontend update handler functions

// Handle watchlist updates
async function handleWatchlistUpdate(update: WatchlistUpdate) {
	if (!browser) return;

	try {
		await initializeStoreReferences();
	} catch (error) {
		console.warn('❌ Store module not available for watchlist update, skipping:', error);
		return;
	}

	if (!storeModule) {
		console.warn('❌ Store module not available for watchlist update');
		return;
	}

	try {
		console.log('📋 Processing watchlist update:', update.action, update);
		switch (update.action) {
			case 'add':
				if (update.item) {
					// Add to current watchlist items if it's the active watchlist
					const currentWatchlistId = get(storeModule.currentWatchlistId);
					if (update.watchlistId === currentWatchlistId) {
						storeModule.currentWatchlistItems.update((items: WatchlistItem[]) => {
							const currentItems = Array.isArray(items) ? items : [];
							// Check if item already exists to avoid duplicates
							if (!currentItems.find((item: WatchlistItem) =>
								item.securityId === update.item!.securityId ||
								item.ticker === update.item!.ticker
							)) {
								return [...currentItems, update.item!];
							}
							return currentItems;
						});
					}

					// Add to flag watchlist if applicable
					const flagWatchlistId = storeModule.flagWatchlistId;
					if (update.watchlistId === flagWatchlistId) {
						storeModule.flagWatchlist.update((items: WatchlistItem[]) => {
							const currentItems = Array.isArray(items) ? items : [];
							if (!currentItems.find((item: WatchlistItem) =>
								item.securityId === update.item!.securityId ||
								item.ticker === update.item!.ticker
							)) {
								return [...currentItems, update.item!];
							}
							return currentItems;
						});
					}
				}
				break;

			case 'remove':
				if (update.itemId) {
					// Remove from current watchlist items
					storeModule.currentWatchlistItems.update((items: WatchlistItem[]) =>
						Array.isArray(items) ? items.filter((item: WatchlistItem) => item.watchlistItemId !== update.itemId) : []
					);

					// Remove from flag watchlist
					storeModule.flagWatchlist.update((items: WatchlistItem[]) =>
						Array.isArray(items) ? items.filter((item: WatchlistItem) => item.watchlistItemId !== update.itemId) : []
					);
				}
				break;

			case 'create':
				if (update.watchlistId && update.watchlistName) {
					// Check if watchlist already exists to prevent duplicates
					const currentWatchlists = get(storeModule.watchlists);
					const exists = Array.isArray(currentWatchlists) &&
						currentWatchlists.find((list: Watchlist) => list.watchlistId === update.watchlistId);

					if (!exists) {
						storeModule.watchlists.update((lists: Watchlist[]) => {
							const currentLists = Array.isArray(lists) ? lists : [];
							const newWatchlist: Watchlist = {
								watchlistId: update.watchlistId!,
								watchlistName: update.watchlistName!
							};
							return [...currentLists, newWatchlist];
						});

						// NEW: Also update visibleWatchlistIds to make the new watchlist visible
						try {
							const { addToVisibleTabs } = await import('$lib/features/watchlist/watchlistUtils');
							addToVisibleTabs(update.watchlistId);
							console.log('✅ Added new watchlist to visible tabs');
						} catch (error) {
							console.warn('❌ Failed to add watchlist to visible tabs:', error);
						}
					} else {
						console.log('📋 Watchlist already exists, skipping duplicate creation');
					}
				}
				break;

			case 'delete':
				if (update.watchlistId) {
					// Check if watchlist exists before trying to delete
					const currentWatchlists = get(storeModule.watchlists);
					const exists = Array.isArray(currentWatchlists) &&
						currentWatchlists.find((list: Watchlist) => list.watchlistId === update.watchlistId);

					if (exists) {
						storeModule.watchlists.update((lists: Watchlist[]) =>
							Array.isArray(lists) ? lists.filter((list: Watchlist) => list.watchlistId !== update.watchlistId) : []
						);

						// NEW: Also remove from visibleWatchlistIds
						try {
							const { visibleWatchlistIds } = await import('$lib/features/watchlist/watchlistUtils');
							visibleWatchlistIds.update((ids: number[]) =>
								Array.isArray(ids) ? ids.filter((id: number) => id !== update.watchlistId) : []
							);
							console.log('✅ Removed deleted watchlist from visible tabs');
						} catch (error) {
							console.warn('❌ Failed to remove watchlist from visible tabs:', error);
						}

						// If the deleted watchlist was the current one, try to select another
						const currentWatchlistId = get(storeModule.currentWatchlistId);
						if (currentWatchlistId === update.watchlistId) {
							try {
								const { selectWatchlist } = await import('$lib/features/watchlist/watchlistUtils');
								const remainingWatchlists = get(storeModule.watchlists);
								if (Array.isArray(remainingWatchlists) && remainingWatchlists.length > 0) {
									selectWatchlist(String(remainingWatchlists[0].watchlistId));
									console.log('✅ Selected new current watchlist after deletion');
								}
							} catch (error) {
								console.warn('❌ Failed to select new current watchlist:', error);
							}
						}
					} else {
						console.log('📋 Watchlist does not exist, skipping deletion');
					}
				}
				break;

			case 'update':
				// Handle watchlist name updates
				if (update.watchlistName && update.watchlistId) {
					storeModule.watchlists.update((lists: Watchlist[]) =>
						Array.isArray(lists) ? lists.map((list: Watchlist) =>
							list.watchlistId === update.watchlistId
								? { ...list, watchlistName: update.watchlistName! }
								: list
						) : []
					);
				}
				break;

			default:
				console.warn('❌ Unknown watchlist update action:', update.action);
		}

		console.log('✅ Successfully processed watchlist update:', update.action);
	} catch (error) {
		console.warn('❌ Error handling watchlist update:', error);
	}
}

// Handle horizontal line updates
function handleHorizontalLineUpdate(update: HorizontalLineUpdate) {
	if (!browser) return;

	try {
		console.log('📏 Processing horizontal line update:', update.action, update);

		// Import horizontalLines store dynamically to avoid circular dependencies
		import('$lib/utils/stores/stores').then(({ horizontalLines }) => {
			horizontalLines.update((lines: HorizontalLine[]) => {
				const currentLines = Array.isArray(lines) ? lines : [];

				switch (update.action) {
					case 'add':
						// Add new line if it doesn't already exist
						if (!currentLines.find((line: HorizontalLine) => line.id === update.line.id)) {
							return [...currentLines, {
								id: update.line.id,
								securityId: update.line.securityId,
								price: update.line.price,
								color: update.line.color,
								lineWidth: update.line.lineWidth
							}];
						}
						return currentLines;

					case 'remove':
						// Remove line by ID
						return currentLines.filter((line: HorizontalLine) => line.id !== update.line.id);

					case 'update':
						// Update existing line
						return currentLines.map((line: HorizontalLine) =>
							line.id === update.line.id
								? {
									...line,
									price: update.line.price,
									color: update.line.color,
									lineWidth: update.line.lineWidth
								}
								: line
						);

					default:
						console.warn('Unknown horizontal line action:', update.action);
						return currentLines;
				}
			});

			console.log('📏 Horizontal line store updated successfully');
		}).catch(error => {
			console.warn('❌ Error updating horizontal line store:', error);
		});

		// Keep the custom event for backwards compatibility
		if (typeof window !== 'undefined') {
			window.dispatchEvent(new CustomEvent('horizontalLineUpdate', {
				detail: update
			}));
		}
	} catch (error) {
		console.warn('❌ Error handling horizontal line update:', error);
	}
}

// Handle alert updates
async function handleAlertUpdate(update: AlertUpdate) {
	if (!browser) return;

	try {
		await initializeStoreReferences();
	} catch (error) {
		console.warn('❌ Store module not available for alert update, skipping:', error);
		return;
	}

	if (!storeModule) {
		console.warn('❌ Store module not available for alert update');
		return;
	}

	try {
		console.log('🔔 Processing alert update:', update.action, update);
		switch (update.action) {
			case 'add':
				storeModule.activeAlerts.update((alerts: Alert[]) => {
					const currentAlerts = Array.isArray(alerts) ? alerts : [];
					return [...currentAlerts, update.alert as Alert];
				});
				break;

			case 'remove':
				storeModule.activeAlerts.update((alerts: Alert[]) =>
					Array.isArray(alerts) ? alerts.filter((alert: Alert) => alert.alertId !== update.alert.alertId) : []
				);
				storeModule.inactiveAlerts.update((alerts: Alert[]) =>
					Array.isArray(alerts) ? alerts.filter((alert: Alert) => alert.alertId !== update.alert.alertId) : []
				);
				break;

			case 'update':
				storeModule.activeAlerts.update((alerts: Alert[]) =>
					Array.isArray(alerts) ? alerts.map((alert: Alert) =>
						alert.alertId === update.alert.alertId ? { ...alert, ...update.alert } : alert
					) : []
				);
				break;

			case 'trigger':
				// Move from active to inactive alerts
				storeModule.activeAlerts.update((alerts: Alert[]) =>
					Array.isArray(alerts) ? alerts.filter((alert: Alert) => alert.alertId !== update.alert.alertId) : []
				);
				storeModule.inactiveAlerts.update((alerts: Alert[]) => {
					const currentAlerts = Array.isArray(alerts) ? alerts : [];
					return [...currentAlerts, { ...update.alert, active: false } as Alert];
				});
				break;
		}
	} catch (error) {
		console.warn('Error handling alert update:', error);
	}
}

// Handle strategy updates
async function handleStrategyUpdate(update: StrategyUpdate) {
	if (!browser) return;

	try {
		await initializeStoreReferences();
	} catch (error) {
		console.warn('❌ Store module not available for strategy update, skipping:', error);
		return;
	}

	if (!storeModule) {
		console.warn('❌ Store module not available for strategy update');
		return;
	}

	try {
		console.log('📊 Processing strategy update:', update.action, update);
		switch (update.action) {
			case 'add':
				storeModule.strategies.update((strategies: SocketStrategy[]) => {
					const currentStrategies = Array.isArray(strategies) ? strategies : [];
					return [...currentStrategies, { ...update.strategy, activeScreen: true } as SocketStrategy];
				});
				break;

			case 'remove':
				storeModule.strategies.update((strategies: SocketStrategy[]) =>
					Array.isArray(strategies) ? strategies.filter((strat: SocketStrategy) => strat.strategyId !== update.strategy.strategyId) : []
				);
				break;

			case 'update':
				storeModule.strategies.update((strategies: SocketStrategy[]) =>
					Array.isArray(strategies) ? strategies.map((strat: SocketStrategy) =>
						strat.strategyId === update.strategy.strategyId ? { ...strat, ...update.strategy } as SocketStrategy : strat
					) : []
				);
				break;
		}
	} catch (error) {
		console.warn('Error handling strategy update:', error);
	}
}

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
		resolve: (value: unknown) => void;
		reject: (error: Error) => void;
	}
>();

// Store for single pending chat request while disconnected
let pendingChatRequest: {
	requestId: string;
	resolve: (value: unknown) => void;
	reject: (error: Error) => void;
	query: string;
	context: unknown[];
	activeChartContext: unknown;
	conversationId: string;
	timeoutId: NodeJS.Timeout;
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
		pendingChatRequests.forEach((request) => {
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
		if (data && data.messageType === 'AgentStatusUpdate') {
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

		// NEW: Handle dynamic updates
		if (data && data.type === 'watchlist_update') {
			console.log('📋 Watchlist Update Received:', data);
			handleWatchlistUpdate(data as WatchlistUpdate).catch(error => {
				console.warn('❌ Error in watchlist update handler:', error);
			});
			return;
		}

		if (data && data.type === 'horizontal_line_update') {
			console.log('📏 Horizontal Line Update Received:', data);
			handleHorizontalLineUpdate(data as HorizontalLineUpdate);
			return;
		}

		if (data && data.type === 'alert_update') {
			console.log('🔔 Alert Update Received:', data);
			handleAlertUpdate(data as AlertUpdate).catch(error => {
				console.warn('❌ Error in alert update handler:', error);
			});
			return;
		}

		if (data && data.type === 'strategy_update') {
			console.log('📊 Strategy Update Received:', data);
			handleStrategyUpdate(data as StrategyUpdate).catch(error => {
				console.warn('❌ Error in strategy update handler:', error);
			});
			return;
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
					(channelName.includes('-slow-regular') || channelName.includes('-slow-extended'))
				) {
					if (data.price === undefined) {
						return;
					}
					const securityId = parseInt(channelName.split('-')[0]);
					if (!isNaN(securityId)) {
						const tickData: {
							securityid: number;
							price: number;
							data: unknown;
							shouldUpdatePrice: boolean;
							isExtended?: boolean;
						} = {
							securityid: securityId,
							price: data.price,
							data: data,
							shouldUpdatePrice: data.shouldUpdatePrice
						};

						// If this is extended hours data, mark it for extended calculation
						if (channelName.includes('-slow-extended')) {
							tickData.isExtended = true;
						}
						// Skip price updates based on shouldUpdatePrice flag
						if (data.shouldUpdatePrice) {
							enqueueTick(tickData);
						} else {
							// Still enqueue for volume updates but without price
							const volumeOnlyData = {
								securityid: securityId,
								data: data,
								isExtended: channelName.includes('-slow-extended')
							};
							enqueueTick(volumeOnlyData);
						}
					}
				}

				// Handle close data for the hub (both regular and extended)
				if (
					(channelName.includes('-close-regular') || channelName.includes('-close-extended')) &&
					data.price !== undefined
				) {
					const securityId = parseInt(channelName.split('-')[0]);
					if (!isNaN(securityId)) {
						const tickData: {
							securityid: number;
							data: unknown;
							prevClose?: number;
							extendedClose?: number;
						} = {
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

export function disconnect() {
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
	context: unknown[] = [],
	activeChartContext: unknown = null,
	conversationId: string = ''
): { promise: Promise<unknown>; cancel: () => void } {
	// Generate unique request ID
	const requestId = `chat_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

	const promise = new Promise((resolve, reject) => {

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
		cancelChatQuery(requestId);
	};

	return { promise, cancel };
}

// Helper function to send chat query immediately
function sendChatQueryNow(
	requestId: string,
	query: string,
	context: unknown[],
	activeChartContext: unknown,
	conversationId: string,
	resolve: (value: unknown) => void,
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
