import type { Instance } from "$lib/core/types"
import {RealtimeStream} from "$lib/utils/realtimeStream"
import {ReplayStream} from "$lib/utils/replayStream"
import type {Writable} from 'svelte/store'
import {writable} from 'svelte/store'

export type ChannelType = "fast" | "slow" | "quote"
export const activeChannels: Map<string,{count:number,store:Writable<any>}> = new Map()

const realtimeStream = new RealtimeStream;
export const replayStream = new ReplayStream;
let currentStream = realtimeStream;
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

export function getStream(ticker:string,channelType:ChannelType): [Writable<any>,Function]{
    const channelName = `${ticker}-${channelType}`
    console.log(channelName)
    let channel = activeChannels.get(channelName)
    if (channel){
        channel.count += 1
    }else{
        currentStream.subscribe(channelName)
        channel = {count:1,store:writable({})}
    }
    activeChannels.set(channelName,channel)
    return [channel.store, () => releaseStream(channelName)]
}
  

export function startReplay(instance : Instance){
    currentStream.stop()
    currentStream = replayStream
    currentStream.start(instance)
}
export function pauseReplay() {
    if(currentStream != replayStream) {return;} 
    currentStream.pause();

}
export function resumeReplay() {
    if(currentStream != replayStream) {return;} 
    currentStream.resume();
}
export function stopReplay(){
    currentStream.stop()
    currentStream = realtimeStream
    currentStream.start()
}
