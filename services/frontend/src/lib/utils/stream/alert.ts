import type { Alert, AlertData, AlertLog } from '$lib/utils/types/types';
import { activeAlerts, inactiveAlerts, alertLogs, alertPopup } from '$lib/utils/stores/stores';
import { get } from 'svelte/store';
export function handleAlert(data: AlertData) {
	alertPopup.set(data);

	if (get(activeAlerts) !== undefined) {
		// Remove from active alerts

		// Add to inactive alerts
		inactiveAlerts.update((currentInactive: Alert[] | undefined) => {
			const alertToMove = get(activeAlerts)?.find((alert: Alert) => alert.alertId === data.alertId);
			if (currentInactive !== undefined && alertToMove) {
				return [...currentInactive, { ...alertToMove, active: false }];
			}
			return currentInactive;
		});
		activeAlerts.update((currentAlerts: Alert[] | undefined) => {
			if (currentAlerts !== undefined) {
				return currentAlerts.filter((alert: Alert) => alert.alertId !== data.alertId);
			}
		});
		// Update alert logs
		alertLogs.update((currentLogs: AlertLog[] | undefined) => {
			if (currentLogs !== undefined) {
				// Create an AlertLog object from the AlertData
				const alertLog: AlertLog = {
					...data,
					alertType: 'triggered' // Add required property
				};
				return [...currentLogs, alertLog];
			}
			return currentLogs; // Return unchanged if undefined
		});
	}
}
