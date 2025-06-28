import { writable } from 'svelte/store';
import { privateRequest } from '$lib/utils/helpers/backend';
import type { Setup } from '$lib/utils/types/types';

export const setups = writable<Setup[]>([]);

// Fetch setups from the backend
privateRequest<Setup[]>('getSetups', {})
	.then((v: Setup[]) => {
		setups.set(v);
	})
	.catch((error) => {
		console.error('Error fetching setups:', error);
	});
