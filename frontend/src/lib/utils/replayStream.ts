import { privateRequest } from '$lib/core/backend';
import { activeChannels } from '$lib/utils/stream';
import type { Stream } from '$lib/utils/stream';
import {replayInfo} from '$lib/core/stores'
import type{ReplayInfo} from '$lib/core/stores'

export class ReplayStream implements Stream {
    public replayStatus: boolean = false;
    public simulatedTime: number = 0; 
    private playbackSpeed = 1;
    private baseBuffer = 10000 // milliseconds
    private buffer = this.baseBuffer;
    private loopCooldown = 20;
    public isPaused: boolean = false;
    private accumulatedPauseTime: number = 0;
    private pauseStartTime: number = 0;
    private startTime: number = 0; // milliseconds
    private initialTimestamp: number = 0;
    private tickMap: Map<string,{reqInbound:boolean,ticks:Array<any>}> = new Map()

    private timeoutID: number | null = null;
    public subscribe(channelName: string) {
        this.tickMap.set(channelName,{reqInbound:false,ticks:[]})
    }
    public unsubscribe(channelName: string) {
        this.tickMap.delete(channelName)
    }

    public start(timestamp: number) {
        //changeChart(instance, false)
        this.startTime = Date.now()
        this.initialTimestamp = timestamp
        this.replayStatus = true; 
        for (const channel of activeChannels.keys()){
            this.subscribe(channel)
        }
        replayInfo.update((r:ReplayInfo)=>{
            r.startTimestamp = timestamp
            return r
        })
        this.loop()
    }

    private loop(){
        console.log(this.isPaused)
        if(this.isPaused) {return;}
        replayInfo.update((r:ReplayInfo)=>{
            r.status = "active"
            return r
        })
        const currentTime = Date.now();
        for (let [channel,v] of this.tickMap.entries()){
            const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
            this.simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;
            const latestTime = v.ticks[v.ticks.length-1]?.timestamp
            if (!v.reqInbound && (!latestTime || latestTime < this.simulatedTime + this.buffer)){
                v.reqInbound = true
                const [securityId, type] = channel.split("-")
                //console.log(securityId)
                let req;
                if (type === "quote"){
                     req = "getQuoteData"
                }else{
                     req = "getTradeData"
                }
                privateRequest<[]>(req, {
                    securityId: parseInt(securityId),
                    time: latestTime ?? Math.floor(this.simulatedTime),
                    lengthOfTime: this.buffer,
                    extendedHours: false
                },false).then((n:Array<any>)=>{
                    if(Array.isArray(n)){
                        this.tickMap.get(channel).ticks.push(...n)
                        this.tickMap.get(channel).reqInbound = false
                    }
                })
            }
            if (v.ticks.length > 0){
                let i = 0
                const store = activeChannels.get(channel)?.store
                while (i < v.ticks.length && v.ticks[i].timestamp <= this.simulatedTime) {
                    store?.set(v.ticks[i])
                    i ++ 
                }
                v.ticks.splice(0,i)
                //this.tickMap.set(channel,this.tickMap.get(channel).slice(i));
            }
        }
        if (this.replayStatus && !this.isPaused){
            this.timeoutID = setTimeout(()=>this.loop(), this.loopCooldown) as unknown as number;
        }
    }

    public stop() {
        this.replayStatus = false;
        this.isPaused = false;
        replayInfo.update((r:ReplayInfo)=>{
            r.status = "inactive"
            r.replaySpeed = 1 
            return r
        })
    }

    public pause() {
        if (!this.isPaused) {
            this.isPaused = true;
            this.pauseStartTime = Date.now();
        if (this.timeoutID !== null) {
            clearTimeout(this.timeoutID);
            this.timeoutID = null;
        }
        replayInfo.update((r:ReplayInfo)=>{
            r.status = "paused"
            return r
        });
        }
    }

    public resume() {
        if (this.isPaused) {
            console.log("TEST WTF")
            this.isPaused = false;
            replayInfo.update((r:ReplayInfo) => {
                r.status = "active"
                return r 
            })
            this.accumulatedPauseTime += Date.now() - this.pauseStartTime;
            this.loop()
        }
    }

    public changeSpeed(newSpeed: number) {
        const currentTime = Date.now();
        const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
        const simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;
        this.buffer = Math.floor(this.baseBuffer * this.playbackSpeed)
        this.playbackSpeed = newSpeed;
        this.startTime = currentTime - (simulatedTime - this.initialTimestamp) / this.playbackSpeed;
        replayInfo.update((r:ReplayInfo)=> {
            r.replaySpeed = newSpeed
            return r 
        })
    }
}

