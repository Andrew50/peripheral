export interface Instance {
    ticker?: string;
    timestamp?: number;
    securityId?: string | number;
    timeframe?: string;
    extendedHours?: boolean;
    price?: number;
    name?: string;
    market?: string;
    locale?: string;
    primary_exchange?: string;
    active?: boolean;
    market_cap?: number;
    dollar_volume?: number;
    description?: string;
    logo?: string;
    icon?: string;
    share_class_shares_outstanding?: number;
    industry?: string;
    sector?: string;
    totalShares?: number;
    sortOrder?: number;
    flagged?: boolean;
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
    activeScreen: boolean;
}
export interface Watchlist {
    watchlistName: string;
    watchlistId: number;
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
    chartColumns: number;
    chartRows: number;
    dolvol: boolean;
    adrPeriod: number;
    divideTaS: boolean;
    filterTaS: boolean;
    showFilings: boolean;
    enableScreensaver: boolean;
}
export interface StreamInfo {
    status: 'replay' | 'realtime' | 'paused';
    startTimestamp: number | null;
    replaySpeed: number;
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
    alertId?: number;
    alertType: string;
    setupId?: number;
    algoId?: number;
    securityId?: string | number;
    ticker?: string;
    price?: number;
    alertPrice?: number;
}

export interface Trade {
    time: number;
    type: string;
    price: number;
    shares: number;
    tradeId?: number;
    trade_direction?: string;
    status?: string;
    openQuantity?: number;
    closedPnL?: number | null;
}
