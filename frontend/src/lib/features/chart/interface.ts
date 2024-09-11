

export interface ShiftOverlay {
    startX: number
    startY: number
    x: number
    y: number
    width: number
    height: number
    isActive: boolean
    startPrice: number
    currentPrice: number
}
export interface ChartRequest extends Instance{
    bars: number;
    direction: string;
    requestType: string;
}
export interface BarData {
    time: UTCTimestamp;
    open: number; 
    high: number;
    low: number;
    close: number;
    volume: number;
}
export interface SecurityDateBounds {
    minDate: number;
    maxDate: number;
}
import type {Instance} from '$lib/core/types'
import type {Writable} from 'svelte/store'
import {writable} from 'svelte/store'


export let chartQuery: Writable<Instance> = writable({timestamp:0, extendedHours:false, timeframe:"1d",ticker:""})
export function changeChart(newInstance : Instance):void{
    chartQuery.update((oldInstance:Instance)=>{
        const req: ChartRequest = {
            ...oldInstance,
            ...newInstance,
            bars: 150,
            direction: "backward",
            requestType: "loadNewTicker"
        }
        return req
    })
}
