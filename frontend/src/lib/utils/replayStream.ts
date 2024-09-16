import { privateRequest } from '$lib/core/backend';
import { activeChannels } from '$lib/utils/stream';
import type { Stream } from '$lib/utils/stream';
import {get} from 'svelte/store'

export class ReplayStream implements Stream {
    private replayStatus: boolean = false;
    private playbackSpeed = 10;
    private buffer = 5000000;
    private loopCooldown = 10;
    private isPaused: boolean = false;
    private accumulatedPauseTime: number = 0;
    private pauseStartTime: number = 0;
    private startTime: number = 0;
    private initialTimestamp: number = 0;
    private tickMap: Map<string,Array<any>> = new Map()

    public subscribe(channelName: string) {
        this.tickMap.set(channelName,[])
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
            this.tickMap.set(channel,[])
        }
        this.loop()
    }

    private loop(){
        for (let [channel,ticks] of this.tickMap.entries()){
            const currentTime = Date.now();
            const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
            const simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;
            const latestTime = ticks[ticks.length-1]?.time
            console.log(latestTime,simulatedTime + this.buffer)
            if (ticks.length === 0 || latestTime < simulatedTime + this.buffer){
                const [securityId, type] = channel.split("-")
                let req;
                if (type === "quote"){
                     req = "getQuoteData"
                }else{
                     req = "getTradeData"
                }
                privateRequest<[]>(req, {
                    securityId: parseInt(securityId),
                    time: latestTime ?? this.initialTimestamp,
                    lengthOfTime: this.buffer,
                    extendedHours: false
                }).then((v:Array<any>)=>{
                    this.tickMap.set(channel,this.tickMap.get(channel).concat(v));
                })
            }
            if (ticks.length > 0){
                let i = 0
                const store = activeChannels.get(channel)?.store
                while (i < ticks.length && ticks[i].time <= simulatedTime) {
                    store?.set(ticks[i])
                    i ++ 
                }
                this.tickMap.set(channel,this.tickMap.get(channel).slice(i));
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

