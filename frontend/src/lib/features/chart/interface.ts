

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
export interface ChartQueryDispatch extends Instance {
    bars: number;
    direction: string;
    requestType: string;
    includeLastBar: boolean;
    chartId: ChartId
}
export interface ChartEventDispatch {
    event: ChartEvent,
    chartId: ChartId
    data?: any
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
import type { Instance } from '$lib/core/types'
import type { UTCTimestamp } from 'lightweight-charts'
import type { Writable } from 'svelte/store'
import { writable } from 'svelte/store'

export type ChartId = number | "all"
export type ChartEvent = "" | "replay" | "realtime" | "addHorizontalLine"

export let selectedChartId: ChartId = 0

export function setActiveChart(chartId: number) { selectedChartId = chartId }

export const chartQueryDispatcher: Writable<ChartQueryDispatch> = writable({ timestamp: 0, extendedHours: false, timeframe: "1d", ticker: "" })
export const chartEventDispatcher: Writable<ChartEventDispatch> = writable({ event: "", chartId: 0, data: null })
export function eventChart(event: ChartEvent, chartId: ChartId = "all", data: any = null) {
    chartEventDispatcher.set({ event: event, chartId: chartId, data: data })
}
export function addHorizontalLine(price: number) {
    chartEventDispatcher.set({ event: "addHorizontalLine", chartId: selectedChartId, data: price })
}
export function queryChart(newInstance: Instance, includeLast: boolean = true): void {
    const req: ChartQueryDispatch = {
        ...newInstance,
        bars: 600,
        direction: "backward",
        requestType: "loadNewTicker",
        includeLastBar: includeLast,
        chartId: selectedChartId,
    }
    chartQueryDispatcher.set(req)
}
