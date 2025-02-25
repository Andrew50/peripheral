import { writable } from 'svelte/store';
import { privateRequest } from '$lib/core/backend';
import type { Setup } from '$lib/core/types';

export const setups = writable<Setup[]>([]);

privateRequest<Setup[]>('getSetups', {})
  .then((v: Setup[]) => {
    setups.set(v);
  })
  .catch((error) => {
    console.error('Error fetching setups:', error);
  });
