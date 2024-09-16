import { privateRequest } from '$lib/core/backend';
import { activeChannels } from '$lib/utils/stream';
import type { Stream } from '$lib/utils/stream';
import {get} from 'svelte/store'

export class ReplayStream implements Stream {
    private replayStatus: boolean = false;
    private playbackSpeed = 1;
    private buffer = 50000;
    private loopCooldown = 20;
    private isPaused: boolean = false;
    private accumulatedPauseTime: number = 0;
    private pauseStartTime: number = 0;
    private startTime: number = 0;
    private initialTimestamp: number = 0;
    private tickMap: Map<string,{reqInbound:boolean,ticks:Array<any>}> = new Map()

    public subscribe(channelName: string) {
        this.tickMap.set(channelName,{reqInbound:false,ticks:[]})
    }
    public unsubscribe(channelName: string) {
        this.tickMap.delete(channelName)
    }

    public start(timestamp: number) {
        if (!timestamp) return;
        this.startTime = Date.now()
        this.initialTimestamp = timestamp
        this.replayStatus = true; 
        for (const channel of activeChannels.keys()){
            this.tickMap.set(channel,{reqInbound:false,ticks:[]})
        }
        this.loop()
    }

    private loop(){
        const currentTime = Date.now();
        for (let [channel,v] of this.tickMap.entries()){
            //console.log(ticks.length)
            const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
            const simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;
            const latestTime = v.ticks[v.ticks.length-1]?.time
            if (!v.reqInbound && (!latestTime || latestTime < simulatedTime + this.buffer)){
                v.reqInbound = true
                const [securityId, type] = channel.split("-")
                let req;
                if (type === "quote"){
                     req = "getQuoteData"
                }else{
                     req = "getTradeData"
                }
                console.log("reg")
                privateRequest<[]>(req, {
                    securityId: parseInt(securityId),
                    time: latestTime ?? this.initialTimestamp,
                    lengthOfTime: this.buffer,
                    extendedHours: false
                },true).then((n:Array<any>)=>{
                    this.tickMap.get(channel).ticks.push(...n)
                    //this.tickMap.set(channel,this.tickMap.get(channel).concat(v));
                })
            }
            if (v.ticks.length > 0){
                let i = 0
                const store = activeChannels.get(channel)?.store
                while (i < v.ticks.length && v.ticks[i].time <= simulatedTime) {
                    store?.set(v.ticks[i])
                    i ++ 
                }
                v.ticks.splice(0,i)
                //this.tickMap.set(channel,this.tickMap.get(channel).slice(i));
            }
        }
        if (this.replayStatus && !this.isPaused){
            setTimeout(()=>this.loop(), this.loopCooldown);
        }
    }

    public stop() {
        this.replayStatus = false;
        this.isPaused = false;
        console.log('Replay stopped.');
    }

    public pause() {
        if (!this.isPaused) {
            this.isPaused = true;
            this.pauseStartTime = Date.now();
            console.log('Replay paused.');
        }
    }

    public resume() {
        if (this.isPaused) {
            this.isPaused = false;
            this.accumulatedPauseTime += Date.now() - this.pauseStartTime;
            console.log('Replay resumed.');
            this.loop()
        }
    }

    public changeSpeed(newSpeed: number) {
        const currentTime = Date.now();
        const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
        const simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;

        this.playbackSpeed = newSpeed;
        this.startTime = currentTime - (simulatedTime - this.initialTimestamp) / this.playbackSpeed;

        console.log(`Playback speed changed to ${newSpeed}`);
    }
}

