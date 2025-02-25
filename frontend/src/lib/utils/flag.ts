import { flagWatchlist, flagWatchlistId } from '$lib/core/stores';
import type { Instance } from '$lib/core/types';
import { privateRequest } from '$lib/core/backend';
import { get } from 'svelte/store';

export function flagSecurity(instance: Instance) {
	const flagInstance = get(flagWatchlist)?.find((v: Instance) => v.ticker === instance.ticker);
	if (flagInstance) {
		//in the flag watchlist
		const flagInstanceId = flagInstance.watchlistItemId;
		privateRequest<void>('deleteWatchlistItem', { watchlistItemId: flagInstanceId }).then(() => {
			flagWatchlist.update((v: Instance[]) => {
				console.log(v);
				console.log(flagInstanceId);
				return v.filter((i: Instance) => i.watchlistItemId !== flagInstanceId);
			});
			console.log(get(flagWatchlist));
		});
	} else {
		privateRequest<number>('newWatchlistItem', {
			securityId: instance.securityId,
			watchlistId: flagWatchlistId
		}).then((watchlistItemId: number) => {
			instance = { watchlistItemId: watchlistItemId, ...instance };
			flagWatchlist.update((v: Instance[]) => {
				if (!Array.isArray(v)) {
					return [v];
				}
				return [instance, ...v];
			});
		});
	}
}
