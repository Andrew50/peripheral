import { writable } from 'svelte/store';
import { streamInfo } from '$lib/core/stores';
import type { StreamInfo, TradeData, QuoteData } from "$lib/core/types";
import { base_url } from '$lib/core/backend';
import { browser } from '$app/environment'
import { handleAlert } from './alert';
import type { AlertData } from './alert';
export type TimeType = "regular" | "extended"
export type ChannelType = //"fast" | "slow" | "quote" | "close" | "all"
    "fast-regular" |
    "fast-extended" |
    "slow-regular" |
    "slow-extended" |
    "close-regular" |
    "close-extended" |
    "quote" |
    "all" //all trades

export type StreamData = TradeData | QuoteData | number;
export type StreamCallback = (v: TradeData | QuoteData | number) => void;

export const activeChannels: Map<string, StreamCallback[]> = new Map();

type SubscriptionRequest = {
    action: 'subscribe' | 'unsubscribe' | 'replay' | 'pause' | 'play' | 'realtime' | 'speed';
    channelName?: string;
    timestamp?: number;
};

export let socket: WebSocket | null = null;
let reconnectInterval: number = 5000; //ms
const maxReconnectInterval: number = 30000;
let reconnectAttempts: number = 0;
const maxReconnectAttempts: number = 5;
let shouldReconnect: boolean = true;
const connectionStatus = writable<'connected' | 'disconnected' | 'connecting'>('connecting');
connect()

function connect() {
    if (!browser) return
    try {
        const token = sessionStorage.getItem("authToken")
        const socketUrl = base_url + "/ws" + "?token=" + token;
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
        for (const [channelName] of activeChannels.keys()) {
            subscribe(channelName);
        }
    });
    socket.addEventListener('message', (event) => {
        let data
        try {
            data = JSON.parse(event.data);
        } catch {
            return
        }
        const channelName = data.channel;
        if (channelName) {
            if (channelName === "alert") {
                handleAlert(data as AlertData);
            }
            else if (channelName === "timestamp") {
                streamInfo.update((v: StreamInfo) => { return { ...v, timestamp: data.timestamp } })
            } else {
                const callbacks = activeChannels.get(channelName);
                if (callbacks) {
                    callbacks.forEach(callback => callback(data));
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
        console.log("subscribing to", channelName)
        const subscriptionRequest: SubscriptionRequest = {
            action: 'subscribe',
            channelName: channelName,
        };
        socket.send(JSON.stringify(subscriptionRequest));
    }
}

export function unsubscribe(channelName: string) {
    if (socket?.readyState === WebSocket.OPEN) {
        console.log("unsubscribing from", channelName)
        const unsubscriptionRequest: SubscriptionRequest = {
            action: 'unsubscribe',
            channelName: channelName,
        };
        socket.send(JSON.stringify(unsubscriptionRequest));
    }
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

