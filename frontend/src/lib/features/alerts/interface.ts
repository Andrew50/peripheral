import type {Instance} from '$lib/core/types'
import {writable} from 'svelte/store'
export interface AlertLog extends Instance, Alert {}
export interface Alert {
    alertId?: number
    alertType: string
    setupId?: number
    securityId?: number
    ticker?: string
    price?: number
}


export let alerts = writable<Alert[]>(undefined);
export let alertLogs = writable<AlertLog[]>(undefined);
import { privateRequest } from '$lib/core/backend';

export function newPriceAlert(instance:Instance){
    if (!instance.price || !instance.securityId) return;
    newAlert({
        securityId: instance.securityId,
        price: instance.price,
        alertType: 'price'
    })
}

export function newAlert(alert:Alert){
    privateRequest<Alert>('newAlert',alert , true).then((createdAlert: Alert) => {
        alerts.update((currentAlerts:Alert[]) => {
            if (Array.isArray(currentAlerts) && currentAlerts.length > 0){
                return [...currentAlerts, createdAlert]
            }else{
                return [createdAlert]
            }
        })
    })

}
