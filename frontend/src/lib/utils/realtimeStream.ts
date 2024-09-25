import {writable } from 'svelte/store';
import type{Writable } from 'svelte/store';
import {privateRequest,base_url} from '$lib/core/backend'
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
    private streamNameToStore: Map<string,Writable<any>> = new Map()

    public start(){
      if(typeof window === 'undefined') return; 
        for (const [channelName] of activeChannels.keys()){
            this.subscribe(channelName)
        }
        this.socket = new WebSocket(this.url);
        this.socket.addEventListener('close', () => {
            this.connectionStatus.set('disconnected');
            if (this.shouldReconnect) {
                this.reconnect();
            }
        });
        this.socket.addEventListener('open', () => {
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
                const store = this.streamNameToStore.get(channelName)
                if (store) {
                    store.set(data);
                } 
            }
        });
        this.socket.addEventListener('error', (error) => {
            this.socket?.close();
        });
    }
    public stop() {
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
        const [securityId,streamType] = channelName.split("-")
        console.log(channelName)
        const channel = activeChannels.get(channelName)
        if (!channel){
            console.log("couldnt find active channel for", channel)
                return 
        }
        const store = channel.store

        privateRequest<string>("getCurrentTicker",{securityId:parseInt(securityId)})
        .then((ticker:string)=>{
            if (ticker == "delisted"){
                console.log("no realtime for ",securityId)
            }else{
                const streamName = `${ticker}-${streamType}`
                this.streamNameToStore.set(streamName,store)
                if(this.socket?.readyState === WebSocket.OPEN) {
                    const subscriptionRequest : SubscriptionRequest = {
                        action : 'subscribe',
                        channelName: streamName,
                    };
                    this.socket?.send(JSON.stringify(subscriptionRequest));
                }
            }
        })
    }
    public unsubscribe(channelName:string){
        const [securityId,streamType] = channelName.split("-")
        privateRequest<string>("getCurrentTicker",{securityId:parseInt(securityId)})
        .then((ticker:string)=>{
            if (ticker == "delisted"){
                console.log("no realtime for ",securityId)
            }else{
                const streamName = `${ticker}-${streamType}`
                if(this.socket?.readyState === WebSocket.OPEN) {
                    const unsubscriptionRequest : SubscriptionRequest = {
                        action: 'unsubscribe',
                        channelName: streamName,
                    };
                    this.streamNameToStore.delete(streamName)
                    this.socket.send(JSON.stringify(unsubscriptionRequest))
                }
            }
        })    
    }
    private reconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          const reconnectDelay = Math.min(
            this.reconnectInterval * this.reconnectAttempts,
            this.maxReconnectInterval
          );
          setTimeout(() => {
            this.start();
          }, reconnectDelay);
        } else {
        }
      }
}
