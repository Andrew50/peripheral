import {writable } from 'svelte/store';
import type {Writable} from 'svelte/store';

type SubscriptionRequest = {
    action: 'subscribe' | 'unsubscribe';
    channelName: string; 
};
type DataType = 'trades-slow' | 'trades-fast' | 'quotes';
export class WebSocketManager {
    private socket: WebSocket | null = null;
    private url: string;
    private reconnectInterval: number = 5000; // Reconnect interval in milliseconds 
    private maxReconnectInterval: number = 30000;
    private reconnectAttempts: number = 0;
    private maxReconnectAttempts: number = 5; 
    private shouldReconnect: boolean = true;
    private pendingSubscriptions: Set<string> = new Set();
    private messageHandlers: ((data: any) => void)[] = [];
    private tickerStores: Map<string, Writable<any>> = new Map(); 
    private tickerRefCounts: Map<string, number> = new Map();




    public connectionStatus = writable<'connected' | 'disconnected' | 'connecting'>('connecting');

    constructor(url: string) {
        this.url = url;
        this.connect();
      }
    private connect() {
    this.socket = new WebSocket(this.url);

    this.socket.addEventListener('open', () => {
        console.log('WebSocket connection established');
        this.connectionStatus.set('connected');
        this.reconnectAttempts = 0; // Reset reconnect attempts
        this.reconnectInterval = 5000; // Reset reconnect interval

        // Resubscribe to pending channels
        this.pendingSubscriptions.forEach((channelName) => {
            this.subscribe(channelName);
        });
    });

    this.socket.addEventListener('message', (event) => {
        const data = JSON.parse(event.data);
        const ticker = data.ticker;
        const channel = data.channel;
        if (channel) {
            let tickerStore = this.tickerStores.get(channel)
            if (tickerStore) {
                tickerStore.set(data);
            } else {
                tickerStore = writable<any>(data);
                this.tickerStores.set(channel, tickerStore);
            }
        } else {
            console.warn("received message w/o ticker:", data)
        }
    });

    this.socket.addEventListener('close', () => {
        console.log('WebSocket connection closed');
        this.connectionStatus.set('disconnected');
        if (this.shouldReconnect) {
            this.reconnect();
        }
    });

    this.socket.addEventListener('error', (error) => {
        console.error('WebSocket error:', error);
        // Close the socket to trigger reconnect
        this.socket?.close();
    });
    }
    private reconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          const reconnectDelay = Math.min(
            this.reconnectInterval * this.reconnectAttempts,
            this.maxReconnectInterval
          );
          console.log(`Reconnecting in ${reconnectDelay / 1000} seconds...`);
          setTimeout(() => {
            this.connect();
          }, reconnectDelay);
        } else {
          console.error('Max reconnect attempts reached.');
        }
      }
      public subscribe(channelName: string) {
        if(this.socket?.readyState === WebSocket.OPEN) {
            const subscriptionRequest : SubscriptionRequest = {
                action : 'subscribe',
                channelName: channelName,
            };
            this.socket?.send(JSON.stringify(subscriptionRequest));
        }
        this.pendingSubscriptions.add(channelName);
      }
      public unsubscribe(channelName : string) {
        if(this.socket?.readyState === WebSocket.OPEN) {
            const unsubscriptionRequest : SubscriptionRequest = {
                action: 'unsubscribe',
                channelName: channelName,
            };
            this.socket.send(JSON.stringify(unsubscriptionRequest))
        }
        this.pendingSubscriptions.delete(channelName);
      }
      private subscribeChannel(channel: string) {
        this.subscribe(channel)
      }
      private unsubscribeChannel(channel : string) {
        this.unsubscribe(channel)
      }
      public getTickerStore(ticker : string, type: DataType): Writable<any> {
        const channel = type + '-' + ticker;
        let tickerStore = this.tickerStores.get(channel);
        if (!tickerStore) {
            tickerStore = writable<any>(null);
            this.tickerStores.set(channel, tickerStore)
        }
        const currentCount = this.tickerRefCounts.get(channel) || 0;
        this.tickerRefCounts.set(channel, currentCount + 1)

        if (currentCount === 0) {
            this.subscribeChannel(channel);
        }
      }
      public unsubscribeTickerStore(ticker: string, type: DataType) {
        const channel = type + '-' + ticker;
        const currentCount = this.tickerRefCounts.get(channel) || 0;
        if (currentCount > 1) {
          this.tickerRefCounts.set(channel, currentCount - 1);
        } else {
          this.tickerRefCounts.delete(channel);
          this.tickerStores.delete(channel);
          this.unsubscribeChannel(channel);
        }
      }
    
      public close() {
        this.shouldReconnect = false;
        this.socket?.close();
      }

}