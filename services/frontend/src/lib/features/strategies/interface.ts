import type { Instance, Strategy } from '$lib/utils/types/types';
import { privateRequest } from '$lib/utils/helpers/backend';
import { writable, get } from 'svelte/store';
import { dispatchMenuChange, bottomWindowRequest, strategies } from '$lib/utils/stores/stores';
export const openStrategyId = writable<number | null>(null);
export type SetupEvent = 'new' | 'save' | 'cancel' | number;
export let eventDispatcher = writable<SetupEvent>();
export function setSample(setupId: number, instance: Instance): void {
	if (!setupId || !instance.securityId || !instance.timestamp) return;
	privateRequest<void>('setSample', { setupId: setupId, ...instance });
}
export async function newSetup(): Promise<number | null> {
	return new Promise((resolve, reject) => {
		dispatchMenuChange.set('setups');
		eventDispatcher.set('new');

		const unsub = eventDispatcher.subscribe((v: SetupEvent) => {
			if (v === 'new') {
				// No action required, waiting for a selection
			} else if (v === 'cancel' || v === 'save') {
				unsub();
				resolve(null); // Resolve with null on cancel
			} else {
				unsub();
				resolve(v); // Resolve with setupId
			}
                });
        });
}

export async function openStrategy(idOrName: number | string) {
        bottomWindowRequest.set('strategies');
        if (typeof idOrName === 'number') {
                openStrategyId.set(idOrName);
                return;
        }
        let list = get(strategies);
        let match = Array.isArray(list)
                ? list.find((s) => s.name.toLowerCase() === idOrName.toLowerCase())
                : undefined;
        if (!match) {
                try {
                        const refreshed = await privateRequest<Strategy[]>('getStrategies', {});
                        strategies.set(refreshed || []);
                        match = refreshed?.find((s) => s.name.toLowerCase() === idOrName.toLowerCase());
                } catch {
                        // ignore errors
                }
        }
        if (match) {
                openStrategyId.set(match.strategyId);
        }
}
