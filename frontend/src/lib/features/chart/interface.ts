

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
    includeLastBar: boolean;
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
import type { UTCTimestamp } from 'lightweight-charts'
import type {Writable} from 'svelte/store'
import {writable} from 'svelte/store'

export let selectedChartId: number = 0

export function setActiveChart(chartId:number){selectedChartId = chartId}

export let chartQuery: Writable<Instance> = writable({timestamp:0, extendedHours:false, timeframe:"1d",ticker:""})
export function changeChart(newInstance : Instance, includeLast : boolean = true):void{
        const req: ChartRequest = {
///            ...oldInstance,
            ...newInstance,
            bars: 600,
            direction: "backward",
            requestType: "loadNewTicker",
            includeLastBar: includeLast,
        }
        ///return req
        chartQuery.set(req)
    //})
}
