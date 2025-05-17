import { writable } from 'svelte/store';
import { dispatchMenuChange } from '$lib/utils/stores/stores';

export const openWatchlistId = writable<number | null>(null);

export function openWatchlist(id: number) {
  dispatchMenuChange.set('watchlist');
  openWatchlistId.set(id);
}
