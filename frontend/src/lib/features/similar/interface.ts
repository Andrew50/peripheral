import type { Instance } from '$lib/core/types';
import { writable } from 'svelte/store';
import type { Writable } from 'svelte/store';
import { activeMenu, changeMenu } from '$lib/core/stores';
import { get } from 'svelte/store';

export const instance: Writable<Instance> = writable({ ticker: '', timestamp: 0 });

export function querySimilarInstances(baseIns: Instance): void {
	instance.update((ins) => ({
		...ins,
		...baseIns
	}));
	if (get(activeMenu) !== 'similar') {
		changeMenu('similar');
	}
}
