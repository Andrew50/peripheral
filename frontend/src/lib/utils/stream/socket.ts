import { writable} from 'svelte/store';
import { streamInfo } from '$lib/core/stores';
import type {StreamInfo,TradeData, QuoteData } from "$lib/core/types";
import { base_url } from '$lib/core/backend';
import {browser} from '$app/environment'

export type ChannelType = "fast" | "slow" | "quote" | "close" | "all";
export type StreamData = TradeData | QuoteData | number;
export type StreamCallback = (v: TradeData | QuoteData | number) => void;

export let activeChannels: Map<string, StreamCallback[]> = new Map();

type SubscriptionRequest = {
    action: 'subscribe' | 'unsubscribe' | 'replay' | 'pause' | 'play' | 'realtime' | 'speed';
    channelName?: string;
    timestamp?: number;
};

const socketUrl: string = base_url + "/ws";
export let socket: WebSocket | null = null;
let reconnectInterval: number = 5000; //ms
const maxReconnectInterval: number = 30000;
let reconnectAttempts: number = 0;
const maxReconnectAttempts: number = 5; 
let shouldReconnect: boolean = true;
const connectionStatus = writable<'connected' | 'disconnected' | 'connecting'>('connecting');
connect()

function connect() {
    if (!browser)return
    socket = new WebSocket(socketUrl);
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
        try{
            data = JSON.parse(event.data);
        }catch{
            return
        }
        const channelName = data.channel;
        if (channelName) {
            if (channelName === "timestamp"){
                streamInfo.update((v:StreamInfo)=>{return {...v,timestamp:data.timestamp}})
            }else{
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
        const subscriptionRequest: SubscriptionRequest = {
            action: 'subscribe',
            channelName: channelName,
        };
        socket.send(JSON.stringify(subscriptionRequest));
    }
}

export function unsubscribe(channelName: string) {
    if (socket?.readyState === WebSocket.OPEN) {
        const unsubscriptionRequest: SubscriptionRequest = {
            action: 'unsubscribe',
            channelName: channelName,
        };
        socket.send(JSON.stringify(unsubscriptionRequest));
    }
}


export function getInitialValue(channelName: string, callback: StreamCallback) {
    const [securityId,streamType] = channelName.split("-")
    let func
    switch(streamType){
        case "close": func = "getClose"; break;
        case "quote": func = "getQuote";break;
        case "all": func = "getTrade"; break;
        case "fast": func = "getTrade"; break;
        case "slow": func = "getTrade"; break;
        default: throw new Error("frontend: 19f-0")
    }
    //privateRequest(func,{securityId:securityId,timestamp:get(streamInfo).timestamp}).then((data: StreamData) => callback(data)); // might need to fixed
}

