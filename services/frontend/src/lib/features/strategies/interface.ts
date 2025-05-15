import type { Instance } from '$lib/utils/types/types';
import { privateRequest } from '$lib/utils/helpers/backend';
import { writable } from 'svelte/store';
import { dispatchMenuChange } from '$lib/utils/stores/stores';
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
