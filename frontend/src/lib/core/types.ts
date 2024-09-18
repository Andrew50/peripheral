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

export interface Watch extends Instance {
  flagged: boolean,
  itemId?: number
}
export interface Watchlist {
    watchlistName: string
    watchlistId: number
}
