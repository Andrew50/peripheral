import type {Instance} from '$lib/core/types'
export interface AlertLog extends Instance, Alert {}
export interface Alert {
    alertId: string
    alertType: string
    securityId?: number
    ticker?: string
    price?: number
}


