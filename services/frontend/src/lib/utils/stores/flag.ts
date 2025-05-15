import { flagWatchlist, flagWatchlistId } from '$lib/utils/stores/stores';
import type { Instance } from '$lib/utils/types/types';
import { privateRequest } from '$lib/utils/helpers/backend';
import { get } from 'svelte/store';

// Extended interface that includes watchlistItemId
export interface ExtendedInstance extends Instance {
	watchlistItemId?: number;
}

export function flagSecurity(instance: Instance) {
	const flagInstance = get(flagWatchlist)?.find((v: ExtendedInstance) => v.ticker === instance.ticker);
	if (flagInstance) {
		//in the flag watchlist
		const flagInstanceId = (flagInstance as ExtendedInstance).watchlistItemId;
		privateRequest<void>('deleteWatchlistItem', { watchlistItemId: flagInstanceId }).then(() => {
			flagWatchlist.update((v: ExtendedInstance[]) => {
				return v.filter((i: ExtendedInstance) => i.watchlistItemId !== flagInstanceId);
			});
		});
	} else {
		privateRequest<number>('newWatchlistItem', {
			securityId: instance.securityId,
			watchlistId: flagWatchlistId
		}).then((watchlistItemId: number) => {
			const extendedInstance: ExtendedInstance = { ...instance, watchlistItemId };
			flagWatchlist.update((v: ExtendedInstance[]) => {
				if (!Array.isArray(v)) {
					return [extendedInstance];
				}
				return [extendedInstance, ...v];
			});
		});
	}
}
