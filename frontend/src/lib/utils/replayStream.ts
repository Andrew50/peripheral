//replayStream.ts
import { privateRequest } from '$lib/core/backend';
import { activeChannels } from '$lib/utils/stream';
import type { Stream } from '$lib/utils/stream';
import {currentTimestamp, replayInfo, timeEvent} from '$lib/core/stores';
import type{ReplayInfo, TimeEvent} from '$lib/core/stores';
import {DateTime} from 'luxon';

export class ReplayStream implements Stream {
    public replayStatus: boolean = false;
    public simulatedTime: number = 0; 
    private playbackSpeed = 1;
    private baseBuffer = 10000 // milliseconds
    private buffer = this.baseBuffer;
    private loopCooldown = 100;
    public isPaused: boolean = false;
    private accumulatedPauseTime: number = 0;
    private pauseStartTime: number = 0;
    private startTime: number = 0; // milliseconds
    private initialTimestamp: number = 0;
    private tickMap: Map<string,{reqInbound:boolean,lastUpdateTime:number,ticks:Array<any>}> = new Map()

    private timeoutID: number | null = null;
    public subscribe(channelName: string) {
        this.tickMap.set(channelName,{reqInbound:false,lastUpdateTime:0,ticks:[]})
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
        if(this.isPaused) {return;}
        replayInfo.update((r:ReplayInfo)=>{
            if(r.status !== "paused") {
                r.status = "active"
            }
            return r
        })
        const currentTime = Date.now();
        const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
        this.simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;
        for (let [channel,v] of this.tickMap.entries()){
            const latestTime = v.ticks[v.ticks.length-1]?.timestamp
            const [securityId, type] = channel.split("-")
            if (!v.reqInbound && (!latestTime || latestTime < this.simulatedTime + this.buffer)){
                v.reqInbound = true  
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
                while (i < v.ticks.length && v.ticks[i].timestamp <= this.simulatedTime) {
                    i ++ 
                }
                if ( i > 0){
                    if (type === "slow"){
                         if (v.lastUpdateTime < this.simulatedTime  ){
                            activeChannels.get(channel)?.store.set(v.ticks.splice(0,i))
                            v.lastUpdateTime = 1000 + this.simulatedTime
                         }
                     }else {
                        activeChannels.get(channel)?.store.set(v.ticks.splice(0,i))
                    }
                }
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
            this.isPaused = false;
            replayInfo.update((r:ReplayInfo) => {
                r.status = "active"
                return r 
            })
            console.log('Test')
            this.accumulatedPauseTime += Date.now() - this.pauseStartTime;
            this.loop()
        }
    }

    public changeSpeed(newSpeed: number) {
        const currentTime = Date.now();
        if (this.isPaused) {
            this.accumulatedPauseTime += currentTime - this.pauseStartTime;
            this.pauseStartTime = currentTime;
        }

        const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
        const simulatedTime = this.initialTimestamp + elapsedTime * this.playbackSpeed;
        this.buffer = Math.floor(this.baseBuffer * this.playbackSpeed)
        this.playbackSpeed = newSpeed;
        this.startTime = currentTime - (simulatedTime - this.initialTimestamp) / this.playbackSpeed - this.accumulatedPauseTime;
        replayInfo.update((r:ReplayInfo)=> {
            r.replaySpeed = newSpeed
            return r 
        })
    }
    public jumpToNextMarketOpen() {
        // Get the current simulated time
        this.pause()
        const currentSimulatedTime = this.simulatedTime;
    
        // Create a DateTime object in UTC
        let dateTime = DateTime.fromMillis(currentSimulatedTime, { zone: 'UTC' });
    
        // Convert to America/New_York time zone
        dateTime = dateTime.setZone('America/New_York');
    
        // Increment the date by one day
        dateTime = dateTime.plus({ days: 1 });
    
        // Find the next weekday (Monday to Friday)
        while (dateTime.weekday === 6 || dateTime.weekday === 7) {
            dateTime = dateTime.plus({ days: 1 });
        }
    
        // Set the time to 9 am
        dateTime = dateTime.set({ hour: 9, minute: 30, second: 0, millisecond: 0 });
    
        // Convert back to UTC
        const newSimulatedTime = dateTime.toUTC().toMillis();
    
        // Adjust initialTimestamp
        const currentTime = Date.now();
        const elapsedTime =
            currentTime - this.startTime - this.accumulatedPauseTime;
        this.initialTimestamp =
            newSimulatedTime - elapsedTime * this.playbackSpeed;
    
        // Clear tickMap and reset
        for (const v of this.tickMap.values()) {
            v.ticks = [];
            v.reqInbound = false;
        }
    
        // Update replayInfo
        replayInfo.update((r: ReplayInfo) => {
            r.startTimestamp = this.initialTimestamp;
            return r;
        });   
        currentTimestamp.set(newSimulatedTime)
        timeEvent.update((v:TimeEvent)=>{
            v.event = "replay"
            return {...v}
        })
        this.resume()
    }
    public jumpToNextDay() {
        // Pause the replay
        this.pause();
        const currentTime = Date.now();
        const currentSimulatedTime = this.simulatedTime;
    
        // Create a DateTime object in UTC
        let dateTime = DateTime.fromMillis(currentSimulatedTime, { zone: 'UTC' });
    
        // Convert to America/New_York time zone
        dateTime = dateTime.setZone('America/New_York');
    
        // Increment the date by one day
        dateTime = dateTime.plus({ days: 1 });
    
        // Find the next weekday (Monday to Friday)
        while (dateTime.weekday === 6 || dateTime.weekday === 7) {
            dateTime = dateTime.plus({ days: 1 });
        }
    
        // Set the time to 4 am
        dateTime = dateTime.set({ hour: 4, minute: 0, second: 0, millisecond: 0 });
    
        // Convert back to UTC
        const newSimulatedTime = dateTime.toUTC().toMillis();
    
        // Adjust initialTimestamp
        const elapsedTime = currentTime - this.startTime - this.accumulatedPauseTime;
        this.initialTimestamp = newSimulatedTime - elapsedTime * this.playbackSpeed;
    
        // Adjust startTime
        this.startTime = currentTime - this.accumulatedPauseTime;
    
        // Clear tickMap and reset
        for (const v of this.tickMap.values()) {
            v.ticks = [];
            v.reqInbound = false;
        }
    
        // Update replayInfo
        replayInfo.update((r: ReplayInfo) => {
            r.startTimestamp = this.initialTimestamp;
            return r;
        });
        currentTimestamp.set(newSimulatedTime)
        // Notify timeEvent
        timeEvent.update((v: TimeEvent) => {
            v.event = "replay";
            return { ...v };
        });
    
        // Resume the replay
        this.resume();
    }
}

