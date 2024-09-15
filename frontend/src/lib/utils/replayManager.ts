import { privateRequest } from "$lib/core/backend";

import type { TradeData } from "$lib/features/chart/interface";

let replayStatus : boolean = false; 
let globalCurrentTimestamp : number = 0; // 0 means realTime 

function beginReplay(timestamp : number) {
    replayStatus = true; 

}
function 



function requestReplayTrades (timestamp : number) { 

    privateRequest<TradeData[]>("")
}