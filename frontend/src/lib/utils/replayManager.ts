import { privateRequest } from "$lib/core/backend";
import type { Instance } from "$lib/core/types";

import type { TradeData, ChartRequest } from "$lib/features/chart/interface";

let replayStatus : boolean = false; 
let globalCurrentTimestamp : number = 0; // 0 means realTime 

export function beginReplay(inst? : Instance) {
    if (!inst) {return}
    if (!inst.timestamp) {return}
    replayStatus = true; 
    console.log("TEST")
    requestReplayTrades(inst, inst.timestamp)
}



function requestReplayTrades (inst: Instance, timestamp : number) { 

    privateRequest<TradeData[]>("getTradeData", {securityId:inst.securityId, time:timestamp, lengthOfTime:60000, extendedHours:false})
        .then((tradeDataList: TradeData[]) => {
            console.log("Returned data")
            console.log(tradeDataList)
            if(!(Array.isArray(tradeDataList) && tradeDataList.length > 1)) { return}
            console.log(tradeDataList)
        })
}