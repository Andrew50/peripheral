import {RealtimeStream} from "$lib/utils/realtimeStream"
import {ReplayStream} from "$lib/utils/replayStream"
import type {Writable} from 'svelte/store'

export type ChannelType = "fast" | "slow" | "quote"
export const activeChannels: Map<string,{count:number,store:Writable<any>}> = new Map()

const realtimeStream = new RealtimeStream;
const replayStream = new ReplayStream;
let currentStream = realtimeStream;
export interface Stream {
    start(timestamp?:number): void;
    stop(): void;
    subscribe(securityId:number,channelType:ChannelType): Writable<any>;
    unsubscribe(securityId:number,channelType:ChannelType): void;
}


export interface SlowTrade {
    time: UTCTimestamp; 
    price: number; 
}

export interface FastTrade extends SlowTrade{
    volume: number; 
    exchange: number; 
}

export interface Quote {
    bid: number;
    ask: number;
    bidSize: number;
    askSize: number;
}

function getStream(securityId:number,channelType:ChannelType) {
        const channelName = `${securityId}-${channelType}`
        const activeChannel = activeChannels.get(channelName)
        if (activeChannel){
            activeChannel.count += 1
        }
  }
function releaseStream(securityId:number,channelType:ChannelType) {
        const channelName = `${securityId}-${channelType}`
        const activeChannel = activeChannels.get(channelName)
        if (activeChannel){
            activeChannel.count -= 1
            if (activeChannel.count < 1){
                activeChannels.delete(channelName)
            }else{
                activeChannels.set(channelName,activeChannel)
            }
        }
    }
  

export function startReplay(timestamp:number){
    currentStream.stop()
    currentStream = replayStream
    currentStream.start(timestamp)
}
export function stopReplay(){
    currentStream.stop()
    currentStream = realtimeStream
    currentStream.start()
}
