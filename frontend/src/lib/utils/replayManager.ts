import { privateRequest } from "$lib/core/backend";
import type { Instance } from "$lib/core/types";

import type { TradeData, ChartRequest } from "$lib/features/chart/interface";

let replayStatus : boolean = false; 
let globalCurrentTimestamp : number = 0; // 0 means realTime 
let playbackSpeed = 1;
let isPaused: boolean = false;
let accumulatedPauseTime: number = 0;
let pauseStartTime: number = 0;
let startTime: number = 0;
let initialTimestamp: number = 0;
let index: number = 0;
let trades: TradeData[] = [];
let pendingTrades: TradeData[] = [];
let lastUpdateTime: number = 0;
let updateInterval: number = 20; 
export function beginReplay(inst? : Instance) {
    if (!inst) {return}
    if (!inst.timestamp) {return}
    replayStatus = true; 
    requestReplayTrades(inst, inst.timestamp).then((tradeDataList) => {
        if(!(Array.isArray(tradeDataList) && tradeDataList.length > 1)) { 
            console.error("No trades to replay")
            return; 
        }
        trades = tradeDataList.sort((a,b) => a.time - b.time); 
        replayTrades();
    })
    .catch((error) => {
        console.error("Failed to retrieve trades:", error)
    })
}



function requestReplayTrades (inst: Instance, timestamp : number): Promise<TradeData[]> { 
    return privateRequest<TradeData[]>("getTradeData", {
        securityId:inst.securityId, 
        time:timestamp, 
        lengthOfTime:60000, 
        extendedHours:false}
    );
}
function replayTrades() {
    console.log("Starting replay.")
    if (trades.length === 0) {
        console.error("No trades available for replay.")
        return; 
    }
    index = 0;
    accumulatedPauseTime = 0;
    startTime = Date.now(); 
    initialTimestamp = trades[0].time || Date.now(); 
    lastUpdateTime = Date.now();

    processTrades(); 
}
function processTrades() {
    if (!replayStatus || isPaused) return; 
    const currentTime = Date.now();
    const elapsedTime = currentTime - startTime - accumulatedPauseTime;
    const simulatedTime = initialTimestamp + elapsedTime * playbackSpeed;
    
    while(index < trades.length && trades[index].time <= simulatedTime) {
        pendingTrades.push(trades[index])
        index++; 
    }
    if(currentTime - lastUpdateTime >= updateInterval) {
        if(pendingTrades.length > 0) {
            updateChartHandler(pendingTrades);
            pendingTrades = [];
        }
        lastUpdateTime = currentTime; 
    }
    if (index < trades.length) {
        requestAnimationFrame(processTrades);
    } else {
        replayStatus = false; 
        console.log("Replay completed")
    }
}
function pauseReplay() {
    if (!isPaused) {
        isPaused = true;
        pauseStartTime = Date.now() 
    }
}
function resumeReplay() {
    if(isPaused) {
        isPaused = false;
        accumulatedPauseTime += Date.now() - pauseStartTime;
        processTrades();
    }
}
function changePlaybackSpeed(newSpeed: number) {
    const currentTime = Date.now();
    const elapsedTime = currentTime - startTime - accumulatedPauseTime; 
    const simulatedTime = initialTimestamp + elapsedTime*playbackSpeed; 

    playbackSpeed = newSpeed;

    startTime = currentTime - (simulatedTime - initialTimestamp) / playbackSpeed; 
}
function updateChartHandler(pendingTradesToUpdate : TradeData[]) {
    for (const trade of pendingTradesToUpdate) {
        console.log(trade, new Date(trade.time).toString())
    }
}