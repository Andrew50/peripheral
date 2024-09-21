import {writable } from 'svelte/store';
import {base_url} from '$lib/core/backend'
import {activeChannels} from '$lib/utils/stream'
type SubscriptionRequest = {
    action: 'subscribe' | 'unsubscribe';
    channelName: string; 
};
import type {Stream} from '$lib/utils/stream'
export class RealtimeStream implements Stream{
    private socket: WebSocket | null = null;
    private url: string = base_url + "/ws"
    private reconnectInterval: number = 5000; //ms
    private maxReconnectInterval: number = 30000;
    private reconnectAttempts: number = 0;
    private maxReconnectAttempts: number = 5; 
    private shouldReconnect: boolean = true;
    private connectionStatus = writable<'connected' | 'disconnected' | 'connecting'>('connecting');

    public start(){
      if(typeof window === 'undefined') return; 
        for (const [channelName] of activeChannels.keys()){
            this.subscribe(channelName)
        }
        this.socket = new WebSocket(this.url);
        this.socket.addEventListener('close', () => {
            console.log('WebSocket connection closed');
            this.connectionStatus.set('disconnected');
            if (this.shouldReconnect) {
                this.reconnect();
            }
        });
        this.socket.addEventListener('open', () => {
            console.log('WebSocket connection established');
            this.connectionStatus.set('connected');
            this.reconnectAttempts = 0; 
            this.reconnectInterval = 5000; 
            for (const [channelName] of activeChannels.keys()){
                this.subscribe(channelName)
            }
        });
        this.socket.addEventListener('message', (event) => {
            const data = JSON.parse(event.data);
            const channelName = data.channel;
            if (channelName) {
                let activeChannel = activeChannels.get(channelName)
                if (activeChannel) {
                    activeChannel.store.set(data);
                } 
            }
        });
        this.socket.addEventListener('error', (error) => {
            console.error('WebSocket error:', error);
            this.socket?.close();
        });
    }
    public stop() {
        console.log('Stopping WebSocket connection and clearing subscriptions...');
        this.shouldReconnect = false; 
        this.connectionStatus.set('disconnected');

        if (this.socket) {
            for (const channelName of activeChannels.keys()) {
                this.unsubscribe(channelName); 
            }
            this.socket.close();
            this.socket = null;
        }
    }
    public subscribe(channelName:string){
        if(this.socket?.readyState === WebSocket.OPEN) {
            const subscriptionRequest : SubscriptionRequest = {
                action : 'subscribe',
                channelName: channelName,
            };
            this.socket?.send(JSON.stringify(subscriptionRequest));
        }
    }
    public unsubscribe(channelName:string){
        if(this.socket?.readyState === WebSocket.OPEN) {
            const unsubscriptionRequest : SubscriptionRequest = {
                action: 'unsubscribe',
                channelName: channelName,
            };
            this.socket.send(JSON.stringify(unsubscriptionRequest))
        }
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
            this.start();
          }, reconnectDelay);
        } else {
          console.error('Max reconnect attempts reached.');
        }
      }
}
