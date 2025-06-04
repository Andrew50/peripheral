// socket.ts
import { writable } from 'svelte/store';
import { streamInfo, handleTimestampUpdate } from '$lib/utils/stores/stores';
import type { StreamInfo, TradeData, QuoteData, CloseData } from '$lib/utils/types/types';
import { base_url } from '$lib/utils/helpers/backend';
import { browser } from '$app/environment';
import { handleAlert } from './alert';
import type { AlertData } from '$lib/utils/types/types';

// Define the type for function status updates from backend (simplified)
export type FunctionStatusUpdate = {
        type: 'function_status';
        userMessage: string;
};

export type BacktestRowMessage = {
        type: 'backtest_row';
        strategyId: number;
        data: any;
};

export type BacktestRowsMessage = {
        type: 'backtest_rows';
        strategyId: number;
        rows: any[];
};

export type BacktestProgressMessage = {
        type: 'backtest_progress';
        strategyId: number;
        percent: number;
};

export type BacktestSummaryMessage = {
        type: 'backtest_summary';
        strategyId: number;
        summary: any;
};

// Store to hold the current function status message
export const functionStatusStore = writable<FunctionStatusUpdate | null>(null);

export type StoreRefreshMessage = {
       type: 'store_refresh';
       store: string;
       params?: Record<string, any>;
};

export const storeRefresh = writable<StoreRefreshMessage | null>(null);

export const backtestProgress = writable<BacktestProgressMessage | null>(null);

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

const backtestRowCallbacks: Map<number, ((row: any) => void)[]> = new Map();
const backtestSummaryCallbacks: Map<number, ((summary: any) => void)[]> = new Map();
const backtestProgressCallbacks: Map<number, ((p: BacktestProgressMessage) => void)[]> = new Map();

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
connect();

function connect() {
	if (!browser) return;
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
                        return;
                }
               if (data && data.type === 'store_refresh') {
                       storeRefresh.set(data as StoreRefreshMessage);
                       return;
               }
                if (data && data.type === 'backtest_row') {
                        const msg = data as BacktestRowMessage;
                        const cbs = backtestRowCallbacks.get(msg.strategyId);
                        cbs?.forEach((cb) => cb(msg.data));
                        return;
                }
                if (data && data.type === 'backtest_rows') {
                        const msg = data as BacktestRowsMessage;
                        const cbs = backtestRowCallbacks.get(msg.strategyId);
                        cbs?.forEach((cb) => msg.rows.forEach((r) => cb(r)));
                        return;
                }
                if (data && data.type === 'backtest_progress') {
                        const msg = data as BacktestProgressMessage;
                        backtestProgress.set(msg);
                        return;
                }
                if (data && data.type === 'backtest_summary') {
                        const msg = data as BacktestSummaryMessage;
                        const cbs = backtestSummaryCallbacks.get(msg.strategyId);
                        cbs?.forEach((cb) => cb(msg.summary));
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

export function addBacktestRowListener(strategyId: number, cb: (row: any) => void): () => void {
        let arr = backtestRowCallbacks.get(strategyId);
        if (!arr) { arr = []; backtestRowCallbacks.set(strategyId, arr); }
        arr.push(cb);
        return () => {
                const a = backtestRowCallbacks.get(strategyId);
                if (!a) return;
                const i = a.indexOf(cb);
                if (i !== -1) a.splice(i, 1);
                if (a.length === 0) backtestRowCallbacks.delete(strategyId);
        };
}

export function addBacktestSummaryListener(strategyId: number, cb: (sum: any) => void): () => void {
        let arr = backtestSummaryCallbacks.get(strategyId);
        if (!arr) { arr = []; backtestSummaryCallbacks.set(strategyId, arr); }
        arr.push(cb);
        return () => {
                const a = backtestSummaryCallbacks.get(strategyId);
                if (!a) return;
                const i = a.indexOf(cb);
                if (i !== -1) a.splice(i, 1);
                if (a.length === 0) backtestSummaryCallbacks.delete(strategyId);
        };
}

export function addBacktestProgressListener(strategyId: number, cb: (p: BacktestProgressMessage) => void): () => void {
        let arr = backtestProgressCallbacks.get(strategyId);
        if (!arr) { arr = []; backtestProgressCallbacks.set(strategyId, arr); }
        arr.push(cb);
        return () => {
                const a = backtestProgressCallbacks.get(strategyId);
                if (!a) return;
                const i = a.indexOf(cb);
                if (i !== -1) a.splice(i, 1);
                if (a.length === 0) backtestProgressCallbacks.delete(strategyId);
        };
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



/*export function getInitialValue(channelName: string, callback: StreamCallback) {
	console.warn("getInitialValue", channelName)
	return
	const [securityId, streamType] = channelName.split("-")
	let func
	switch (streamType) {
		case "close": func = "getClose"; break;
		case "quote": func = "getQuote"; break;
		case "all": func = "getTrade"; break;
		case "fast_trades": func = "getTrade"; break;//these might have to change to not get
		case "slow_trades": func = "getTrade"; break;
		case "fast_quotes": func = "getQuote"; break;
		case "slow_quotes": func = "getQuote"; break;
		default: throw new Error("frontend: 19f-0")
	}
	//privateRequest(func,{securityId:securityId,timestamp:get(streamInfo).timestamp}).then((data: StreamData) => callback(data)); // might need to fixed
}
*/

// /socket.ts

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
