import {socket,subscribe,unsubscribe,activeChannels,getInitialValue} from './socket'
import type {SubscriptionRequest,StreamCallback} from './socket'
import {DateTime} from 'luxon';
import {eventChart} from "$lib/features/chart/interface"
import {streamInfo} from '$lib/core/stores'
import {chartEventDispatcher} from '$lib/features/chart/interface'
import { getReferenceStartTimeForDateMilliseconds, isOutsideMarketHours, ESTSecondstoUTCMillis, getRealTimeTime } from '$lib/core/timestamp';
import type { ReplayInfo } from '$lib/core/stores';
import { type Instance} from '$lib/core/types'
import {get} from 'svelte/store'
export function releaseStream(channelName: string, callback: StreamCallback) {
    let callbacks = activeChannels.get(channelName);
    if (callbacks) {
        callbacks = callbacks.filter(v => v !== callback);
        if (callbacks.length === 0) {
            activeChannels.delete(channelName);
            unsubscribe(channelName);
        } else {
            activeChannels.set(channelName, callbacks);
        }
    }
}

export function addStream<T extends StreamData>(instance: Instance, channelType: ChannelType, callback: StreamCallback): Function {
    if (!instance.securityId) return () => {};
    const channelName = `${instance.securityId}-${channelType}`;
    let callbacks = activeChannels.get(channelName);
    if (callbacks) {
        if(!callbacks.includes(callback)) { 
            callbacks.push(callback);
        }
    } else {
        activeChannels.set(channelName, [callback]);
        subscribe(channelName);
    }
    getInitialValue(channelName, callback);
        
    return () => releaseStream(channelName, callback);
}
export function startReplay(instance: Instance) {
    if (!instance.timestamp) return
    if(get(streamInfo).replayActive) {
        stopReplay()
    } 
    if (socket?.readyState === WebSocket.OPEN) {
        const timestampToUse = isOutsideMarketHours(instance.timestamp) 
            ? ESTSecondstoUTCMillis(getReferenceStartTimeForDateMilliseconds(instance.timestamp, ) / 1000)
            : instance.timestamp;
        setExtended(instance.extendedHours ?? false)
        const replayRequest: SubscriptionRequest = {
            action: 'replay',
            timestamp: timestampToUse,
        };
        socket.send(JSON.stringify(replayRequest));
        console.log("replay request sent")
        streamInfo.update((v) => {return {...v,replayActive:true,replayPaused:false,startTimestamp:timestampToUse,timestamp:timestampToUse}})
        chartEventDispatcher.set({event:"replay",chartId:"all"})
        //timeEvent.update((v: TimeEvent) => ({ ...v, event: 'replay' }));
    }
}

export function pauseReplay() {
    if (socket?.readyState === WebSocket.OPEN) {
        const pauseRequest: SubscriptionRequest = {
            action: 'pause',
        };
        socket.send(JSON.stringify(pauseRequest));
    }
    streamInfo.update((v) => ({...v, replayPaused: true, pauseTime: Date.now()}));
}

export function resumeReplay() {
    if (socket?.readyState === WebSocket.OPEN) {
        const playRequest: SubscriptionRequest = {
            action: 'play',
        };
        socket.send(JSON.stringify(playRequest));
    }
    streamInfo.update((v) => {
        const pauseDuration = Date.now() - (v.pauseTime || Date.now());
        return {
            ...v,
            replayPaused: false,
            timestamp: v.timestamp + pauseDuration * v.replaySpeed,
            lastUpdateTime: Date.now()
        };
    });
}

export function stopReplay() {
    if (socket?.readyState === WebSocket.OPEN) {
        const stopRequest: SubscriptionRequest = {
            action: 'realtime',
        };
        socket.send(JSON.stringify(stopRequest));
    }
    streamInfo.update((r: ReplayInfo) => ({ ...r, replayActive:false, timestamp:getRealTimeTime()}));
}
export function changeSpeed(speed:number) {
    if (socket?.readyState === WebSocket.OPEN) {
        const stopRequest: SubscriptionRequest = {
            action: 'speed',
            speed: speed
        };
        socket.send(JSON.stringify(stopRequest));
    }
    streamInfo.update((r: ReplayInfo) => ({ ...r,replaySpeed:speed }));
}

export function nextDay(){
    if (socket?.readyState === WebSocket.OPEN) {
        const stopRequest: SubscriptionRequest = {
            action: 'nextOpen',
        };
        socket.send(JSON.stringify(stopRequest));
    }
}
export function setExtended(extendedHours: boolean){
    if (socket?.readyState === WebSocket.OPEN) {
        const stopRequest: SubscriptionRequest = {
            action: 'setExtended',
            extendedHours: extendedHours,
        };
        socket.send(JSON.stringify(stopRequest));
    }
    streamInfo.update((r: ReplayInfo) => ({ ...r,extendedHours:extendedHours}));
}
