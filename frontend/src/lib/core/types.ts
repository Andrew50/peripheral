export interface Instance {
    ticker?: string
    datetime?: string
    timestamp?: number
    securityId?: number
    timeframe?: string
    extendedHours?: boolean
}
export interface chartRequest extends Instance{
    bars: number;
    direction: string;
    requestType: string;
}
