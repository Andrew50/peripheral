export interface Instance {
    ticker?: string
    timestamp?: number
    securityId?: number
    timeframe?: string
    extendedHours?: boolean
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
    activeScreen: boolean
}
export interface Watchlist {
    watchlistName: string
    watchlistId: number
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
}
