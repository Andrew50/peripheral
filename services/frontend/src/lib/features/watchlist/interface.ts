import { writable, get } from 'svelte/store';
import { dispatchMenuChange, watchlists } from '$lib/utils/stores/stores';
import type { Watchlist } from '$lib/utils/types/types';
import { privateRequest } from '$lib/utils/helpers/backend';

export const openWatchlistId = writable<number | null>(null);

export async function openWatchlist(idOrName: number | string) {
  dispatchMenuChange.set('watchlist');
  if (typeof idOrName === 'number') {
    openWatchlistId.set(idOrName);
    return;
  }

  let list = get(watchlists);
  let match = Array.isArray(list)
    ? list.find((w) => w.watchlistName.toLowerCase() === idOrName.toLowerCase())
    : undefined;

  if (!match) {
    try {
      const refreshed = await privateRequest<Watchlist[]>('getWatchlists', {});
      watchlists.set(refreshed || []);
      match = refreshed?.find((w) =>
        w.watchlistName.toLowerCase() === idOrName.toLowerCase()
      );
    } catch {
      // ignore errors
    }
  }

  if (match) {
    openWatchlistId.set(match.watchlistId);
  }
}
