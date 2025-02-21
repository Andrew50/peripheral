import type { Writable } from 'svelte/store';
export interface Instance {
    ticker?: string
    timestamp?: number
    securityId?: number
    timeframe?: string
    extendedHours?: boolean
    price?: number;
    name?: string;
    market?: string;
    locale?: string;
    primary_exchange?: string;
    active?: boolean;
    market_cap?: number;
    description?: string;
    logo?: string;
    icon?: string;
    share_class_shares_outstanding?: number;
    industry?: string;
    sector?: string;
    totalShares?: number;
}
export interface Setup {
    setupId: number;
    name: string;
    timeframe: string;
    bars: number;
    threshold: number;
    dolvol: number;
    adr: number;
    mcap: number;
    score: number;
    activeScreen: boolean
}
export interface Watchlist {
    watchlistName: string
    watchlistId: number
}
export interface CloseData {
    price: number;
    channel: string;
}
export interface TradeData {
    timestamp: number;
    price: number;
    size: number;
    exchange: number;
    prevClose: number;
    conditions: Array<number>;
}
export interface QuoteData {
    timestamp: number;
    bidPrice: number;
    askPrice: number;
    bidSize: number;
    askSize: number;
}
export interface Settings {
    chartColumns: number
    chartRows: number
    dolvol: boolean
    adrPeriod: number
    divideTaS: boolean
    filterTaS: boolean
    showFilings: boolean
}
export interface StreamInfo {
    status: "replay" | "realtime" | "paused",
    startTimestamp: number | null,
    replaySpeed: number,
}
export interface AlertData {
    message: string;
    alertId: number;
    timestamp: number;
    securityId: number;
}
export interface AlertLog extends Instance, Alert { }
export interface Alert {
    active?: boolean;
    alertId?: number
    alertType: string
    setupId?: number
    algoId?: number
    securityId?: number
    ticker?: string
    price?: number
}