import type { Alert, AlertLog, Instance } from '$lib/utils/types/types';
import { writable } from 'svelte/store';
import { privateRequest } from '$lib/utils/helpers/backend';
import { activeAlerts, inactiveAlerts } from '$lib/utils/stores/stores';

export type { Alert, AlertLog };

export function newPriceAlert(instance: Instance) {
	if (!instance.price || !instance.securityId) return;
	newAlert({
		securityId: instance.securityId,
		price: instance.price,
		alertType: 'price',
		ticker: instance.ticker
	});
}

export function newAlert(alert: Alert) {
	if (alert.price) {
		alert.price = parseFloat(alert.price.toFixed(2));
	}
	// Convert Alert to Record<string, unknown> to satisfy the type requirement
	const alertRecord: Record<string, unknown> = { ...alert };

	privateRequest<Alert>('newAlert', alertRecord).then((createdAlert: Alert) => {
		createdAlert.ticker = alert.ticker;
		createdAlert.alertType = alert.alertType;
		if (activeAlerts !== undefined) {
			activeAlerts.update((currentAlerts: Alert[] | undefined) => {
				if (Array.isArray(currentAlerts) && currentAlerts.length > 0) {
					return [...currentAlerts, createdAlert];
				} else {
					return [createdAlert];
				}
			});
		}
	});
}
