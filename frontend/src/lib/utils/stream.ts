//stream.ts
import type { Instance,TradeData,QuoteData } from "$lib/core/types"
import {RealtimeStream} from "$lib/utils/realtimeStream"
import {ReplayStream} from "$lib/utils/replayStream"
import type {Writable} from 'svelte/store'
import {writable, get} from 'svelte/store'
import {ESTSecondstoUTCSeconds} from '$lib/core/timestamp'
import {privateRequest} from '$lib/core/backend'
export type ChannelType = "fast" | "slow" | "quote" | "close"
export type Channels = Map<string,{count:number,store:Writable<any>}>
export let activeChannels: Channels = new Map()
import {timeEvent,currentTimestamp, replayInfo} from '$lib/core/stores'
import type {ReplayInfo} from '$lib/core/stores'
import type {TimeEvent} from '$lib/core/stores'

import { getReferenceStartTimeForDateMilliseconds, isOutsideMarketHours } from '$lib/core/timestamp';
import { chartQuery } from '$lib/features/chart/interface';
import { ESTSecondstoUTCMillis } from '$lib/core/timestamp';

const realtimeStream = new RealtimeStream;
export const replayStream = new ReplayStream;
let currentStream: RealtimeStream | ReplayStream = realtimeStream;
currentStream.start();
export interface Stream {
    start(timestamp?:number): void;
    stop(): void;
    subscribe(channelName:string): void;
    unsubscribe(channelName:string): void;
}

export function releaseStream(channelName:string) {
    const activeChannel = activeChannels.get(channelName)
    if (activeChannel){
        activeChannel.count -= 1
        if (activeChannel.count < 1){
            activeChannels.delete(channelName)
            currentStream.unsubscribe(channelName)
        }else{
            activeChannels.set(channelName,activeChannel)
        }
    }
}

export function getStream<T extends TradeData[]|QuoteData[]|number>(instance:Instance,channelType:ChannelType): [Writable<T>,Function]{
    if (!instance.securityId) return [writable(),()=>{}];
    if (channelType == "close"){
        const s = writable(0) as Writable<T>
        const unsubscribe = timeEvent.subscribe((v:TimeEvent)=>{
            if (v.event === "newDay" || v.event === "replay"){
                 privateRequest<number>("getPrevClose",{timestamp:v.UTCtimestamp,securityId:instance.securityId})
                 .then((price:number)=>{
                     s.set(price as T)
                 }).catch((error)=>{})
            }
        })
        return [s, unsubscribe]
    }
    if (["fast","slow","quote"].includes(channelType)){
        const channelName = `${instance.securityId}-${channelType}`
        let channel = activeChannels.get(channelName)
        if (channel){
            channel.count += 1
            activeChannels.set(channelName,channel)
        }else{
            channel = {count:1,store:writable({})}
            activeChannels.set(channelName,channel)
            currentStream.subscribe(channelName)
        }
        const store = channel.store as Writable<T>
        return [store, (()=>releaseStream(channelName))]
    }
}
  

export function startReplay(timestamp : number){
    console.log(timestamp)
    currentStream.stop()
    currentStream = replayStream
    var timestampToUse = timestamp
    if(isOutsideMarketHours(timestampToUse)) {
        timestampToUse = ESTSecondstoUTCMillis(getReferenceStartTimeForDateMilliseconds(timestampToUse, get(chartQuery).extendedHours)/1000)
        console.log("-----",timestamp,timestampToUse)
    }
    currentStream.start(timestampToUse)
    currentTimestamp.set(timestampToUse)
    timeEvent.update((v:TimeEvent)=>{
        v.event = "replay"
        return {...v}
    })
}
export function pauseReplay() {
    if(currentStream !== replayStream) {return;} 
    currentStream.pause();

}
export function resumeReplay() {
    if(currentStream !== replayStream) {return;} 
    currentStream.resume();
}
export function stopReplay(){
    currentStream.stop()
    replayInfo.update((r:ReplayInfo) => {
        r.status = "inactive";
        return r
    })
    currentStream = realtimeStream
    currentStream.start()
}
export function replayJumpToNextDay() {
    if(currentStream !== replayStream) {return;}
    currentStream.jumpToNextDay()
}
export function replayJumpToNextMarketOpen() {
    if(currentStream !== replayStream) {return;}
    currentStream.jumpToNextMarketOpen()
}
