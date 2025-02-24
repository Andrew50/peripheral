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
import { privateRequest } from '$lib/core/backend'
import { streamInfo } from '$lib/core/stores'
import type { Instance } from '$lib/core/types'
import type { UTCTimestamp } from 'lightweight-charts'
import type { Writable } from 'svelte/store'
import { writable, get } from 'svelte/store'

export type ChartId = number | "all"
export type ChartEvent = "" | "replay" | "realtime" | "addHorizontalLine"

export let selectedChartId: ChartId = 0

export const activeChartInstance: Writable<Instance | null> = writable(null)

export function setActiveChart(chartId: ChartId, currentChartInstance: Instance) {
    selectedChartId = chartId
    // Create a new instance object to ensure reactivity
    const updatedInstance = {
        ...currentChartInstance,
        ticker: currentChartInstance.ticker,
        securityId: currentChartInstance.securityId,
        timeframe: currentChartInstance.timeframe,
        extendedHours: currentChartInstance.extendedHours ?? false,
        timestamp: currentChartInstance.timestamp ?? 0,
        detailsFetched: currentChartInstance.detailsFetched ?? false
    }
    // Force a new object reference to trigger store updates
    activeChartInstance.set(updatedInstance)
}

export const chartQueryDispatcher: Writable<ChartQueryDispatch> = writable({ timestamp: 0, extendedHours: false, timeframe: "1d", ticker: "" })
export const chartEventDispatcher: Writable<ChartEventDispatch> = writable({ event: "", chartId: 0, data: null })
export function eventChart(event: ChartEvent, chartId: ChartId = "all", data: any = null) {
    chartEventDispatcher.set({ event: event, chartId: chartId, data: data })
}
export function addHorizontalLine(price: number) {
    chartEventDispatcher.set({ event: "addHorizontalLine", chartId: selectedChartId, data: price })
}
export function queryChart(newInstance: Instance, includeLast: boolean = true): void {
    newInstance.bars = 400
    newInstance.direction = "backward"
    newInstance.requestType = "loadNewTicker"
    newInstance.includeLastBar = includeLast
    newInstance.chartId = selectedChartId

    if (get(streamInfo).replayActive) {
        newInstance.timestamp = get(streamInfo).timestamp
    }

    // Ensure we have all necessary instance properties
    if (!newInstance.detailsFetched && newInstance.securityId) {
        privateRequest('getTickerMenuDetails', { securityId: newInstance.securityId }, true)
            .then((details) => {
                const updatedInstance = {
                    ...newInstance,
                    ...details,
                    detailsFetched: true
                }
                chartQueryDispatcher.set(updatedInstance)
                setActiveChart(selectedChartId, updatedInstance)
            })
    } else {
        chartQueryDispatcher.set(newInstance)
        setActiveChart(selectedChartId, newInstance)
    }
}
